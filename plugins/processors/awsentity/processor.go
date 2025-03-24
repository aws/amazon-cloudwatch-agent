// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsentity

import (
	"context"
	"strings"

	"github.com/go-playground/validator/v10"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	semconv "go.opentelemetry.io/collector/semconv/v1.22.0"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/extension/entitystore"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsentity/entityattributes"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsentity/internal/k8sattributescraper"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/ec2tagger"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
)

const (
	attributeAwsLogGroupNames              = "aws.log.group.names"
	attributeDeploymentEnvironment         = "deployment.environment"
	attributeServiceName                   = "service.name"
	attributeService                       = "Service"
	attributeEC2TagAwsAutoscalingGroupName = "ec2.tag.aws:autoscaling:groupName"
	EMPTY                                  = ""
)

type scraper interface {
	Scrape(rm pcommon.Resource)
	Reset()
}

type EC2ServiceAttributes struct {
	InstanceId        string `validate:"required"`
	AutoScalingGroup  string `validate:"omitempty"`
	ServiceNameSource string `validate:"omitempty"`
}

type K8sServiceAttributes struct {
	Cluster           string `validate:"required"`
	Namespace         string `validate:"required"`
	Workload          string `validate:"required"`
	Node              string `validate:"required"`
	InstanceId        string `validate:"omitempty"`
	ServiceNameSource string `validate:"omitempty"`
}

// use a single instance of Validate, it caches struct info
var validate = validator.New(validator.WithRequiredStructEnabled())

// exposed as a variable for unit testing
var addToEntityStore = func(logGroupName entitystore.LogGroupName, serviceName string, environmentName string) {
	es := entitystore.GetEntityStore()
	if es == nil {
		return
	}
	es.AddServiceAttrEntryForLogGroup(logGroupName, serviceName, environmentName)
}

var addPodToServiceEnvironmentMap = func(podName string, serviceName string, environmentName string, serviceNameSource string) {
	es := entitystore.GetEntityStore()
	if es == nil {
		return
	}
	es.AddPodServiceEnvironmentMapping(podName, serviceName, environmentName, serviceNameSource)
}

var setAutoScalingGroup = func(asg string) {
	es := entitystore.GetEntityStore()
	if es == nil {
		return
	}
	es.SetAutoScalingGroup(asg)
}

var getEC2InfoFromEntityStore = func() entitystore.EC2Info {
	es := entitystore.GetEntityStore()
	if es == nil {
		return entitystore.EC2Info{}
	}

	return es.EC2Info()
}

var getAutoScalingGroupFromEntityStore = func() string {
	// Get the following metric attributes from the EntityStore: EC2.AutoScalingGroup
	es := entitystore.GetEntityStore()
	if es == nil {
		return ""
	}
	return es.GetAutoScalingGroup()
}

var getServiceNameSource = func() (string, string) {
	es := entitystore.GetEntityStore()
	if es == nil {
		return EMPTY, EMPTY
	}
	return es.GetMetricServiceNameAndSource()
}

// awsEntityProcessor looks for metrics that have the aws.log.group.names and either the service.name or
// deployment.environment resource attributes set, then adds the association between the log group(s) and the
// service/environment names to the entitystore extension.
type awsEntityProcessor struct {
	config     *Config
	k8sscraper scraper
	logger     *zap.Logger
}

func newAwsEntityProcessor(config *Config, logger *zap.Logger) *awsEntityProcessor {
	return &awsEntityProcessor{
		config:     config,
		k8sscraper: k8sattributescraper.NewK8sAttributeScraper(config.ClusterName),
		logger:     logger,
	}
}

func (p *awsEntityProcessor) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	// Get the following metric attributes from the EntityStore: PlatformType, EC2.InstanceId, EC2.AutoScalingGroup

	rm := md.ResourceMetrics()
	for i := 0; i < rm.Len(); i++ {
		var logGroupNames, serviceName, environmentName string
		var entityServiceNameSource, entityPlatformType string
		var ec2Info entitystore.EC2Info
		resourceAttrs := rm.At(i).Resource().Attributes()
		switch p.config.EntityType {
		case entityattributes.Resource:
			if p.config.Platform == config.ModeEC2 {
				// ec2tagger processor may have picked up the ASG name from an ec2:DescribeTags call
				if getAutoScalingGroupFromEntityStore() == EMPTY && p.config.ScrapeDatapointAttribute {
					if autoScalingGroup := p.scrapeResourceEntityAttribute(rm.At(i).ScopeMetrics()); autoScalingGroup != EMPTY {
						setAutoScalingGroup(autoScalingGroup)
					}
				}
				ec2Info = getEC2InfoFromEntityStore()
				if ec2Info.GetInstanceID() != EMPTY {
					resourceAttrs.PutStr(entityattributes.AttributeEntityType, entityattributes.AttributeEntityAWSResource)
					resourceAttrs.PutStr(entityattributes.AttributeEntityResourceType, entityattributes.AttributeEntityEC2InstanceResource)
					resourceAttrs.PutStr(entityattributes.AttributeEntityIdentifier, ec2Info.GetInstanceID())
				}
				AddAttributeIfNonEmpty(resourceAttrs, entityattributes.AttributeEntityAwsAccountId, ec2Info.GetAccountID())
			}
		case entityattributes.Service:
			if logGroupNamesAttr, ok := resourceAttrs.Get(attributeAwsLogGroupNames); ok {
				logGroupNames = logGroupNamesAttr.Str()
			}
			if serviceNameAttr, ok := resourceAttrs.Get(attributeServiceName); ok {
				serviceName = serviceNameAttr.Str()
			}
			if environmentNameAttr, ok := resourceAttrs.Get(attributeDeploymentEnvironment); ok {
				environmentName = environmentNameAttr.Str()
			}
			if serviceNameSource, sourceExists := resourceAttrs.Get(entityattributes.AttributeEntityServiceNameSource); sourceExists {
				entityServiceNameSource = serviceNameSource.Str()
			}
			// resourcedetection processor may have picked up the ASG name from an ec2:DescribeTags call
			if autoScalingGroupNameAttr, ok := resourceAttrs.Get(attributeEC2TagAwsAutoscalingGroupName); ok {
				setAutoScalingGroup(autoScalingGroupNameAttr.Str())
			}

			entityServiceName := getServiceAttributes(resourceAttrs)
			entityEnvironmentName := environmentName
			if (entityServiceName == EMPTY || entityEnvironmentName == EMPTY) && p.config.ScrapeDatapointAttribute {
				entityServiceName, entityEnvironmentName, entityServiceNameSource = p.scrapeServiceAttribute(rm.At(i).ScopeMetrics())
				// If the entityServiceNameSource is empty here, that means it was not configured via instrumentation
				// If entityServiceName is a datapoint attribute, that means the service name is coming from the UserConfiguration source
				if entityServiceNameSource == entityattributes.AttributeServiceNameSourceUserConfig && entityServiceName != EMPTY {
					entityServiceNameSource = entityattributes.AttributeServiceNameSourceUserConfig
				}
			}
			if p.config.KubernetesMode != "" {
				p.k8sscraper.Scrape(rm.At(i).Resource())
				if p.config.Platform == config.ModeEC2 {
					ec2Info = getEC2InfoFromEntityStore()
				}

				if p.config.KubernetesMode == config.ModeEKS {
					entityPlatformType = entityattributes.AttributeEntityEKSPlatform
				} else {
					entityPlatformType = entityattributes.AttributeEntityK8sPlatform
				}

				podInfo, ok := p.k8sscraper.(*k8sattributescraper.K8sAttributeScraper)
				// Perform fallback mechanism for service and environment name if they
				// are empty
				if entityServiceName == EMPTY && ok && podInfo != nil && podInfo.Workload != EMPTY {
					entityServiceName = podInfo.Workload
					entityServiceNameSource = entitystore.ServiceNameSourceK8sWorkload
				}

				if entityEnvironmentName == EMPTY && ok && podInfo.Cluster != EMPTY && podInfo.Namespace != EMPTY {
					if p.config.KubernetesMode == config.ModeEKS {
						entityEnvironmentName = "eks:" + p.config.ClusterName + "/" + podInfo.Namespace
					} else if p.config.KubernetesMode == config.ModeK8sEC2 || p.config.KubernetesMode == config.ModeK8sOnPrem {
						entityEnvironmentName = "k8s:" + p.config.ClusterName + "/" + podInfo.Namespace
					}
				}

				// Add service information for a pod to the pod association map
				// so that agent can host this information in a server
				fullPodName := scrapeK8sPodName(resourceAttrs)
				if fullPodName != EMPTY && entityServiceName != EMPTY && entityServiceNameSource != EMPTY {
					addPodToServiceEnvironmentMap(fullPodName, entityServiceName, entityEnvironmentName, entityServiceNameSource)
				} else if fullPodName != EMPTY && entityServiceName != EMPTY && entityServiceNameSource == EMPTY {
					addPodToServiceEnvironmentMap(fullPodName, entityServiceName, entityEnvironmentName, entitystore.ServiceNameSourceUnknown)
				}
				eksAttributes := K8sServiceAttributes{
					Cluster:           podInfo.Cluster,
					Namespace:         podInfo.Namespace,
					Workload:          podInfo.Workload,
					Node:              podInfo.Node,
					InstanceId:        ec2Info.GetInstanceID(),
					ServiceNameSource: entityServiceNameSource,
				}
				AddAttributeIfNonEmpty(resourceAttrs, entityattributes.AttributeEntityType, entityattributes.Service)
				AddAttributeIfNonEmpty(resourceAttrs, entityattributes.AttributeEntityServiceName, entityServiceName)
				AddAttributeIfNonEmpty(resourceAttrs, entityattributes.AttributeEntityDeploymentEnvironment, entityEnvironmentName)

				if err := validate.Struct(eksAttributes); err == nil {
					resourceAttrs.PutStr(entityattributes.AttributeEntityPlatformType, entityPlatformType)
					resourceAttrs.PutStr(entityattributes.AttributeEntityCluster, eksAttributes.Cluster)
					resourceAttrs.PutStr(entityattributes.AttributeEntityNamespace, eksAttributes.Namespace)
					resourceAttrs.PutStr(entityattributes.AttributeEntityWorkload, eksAttributes.Workload)
					resourceAttrs.PutStr(entityattributes.AttributeEntityNode, eksAttributes.Node)
					AddAttributeIfNonEmpty(resourceAttrs, entityattributes.AttributeEntityInstanceID, ec2Info.GetInstanceID())
					AddAttributeIfNonEmpty(resourceAttrs, entityattributes.AttributeEntityAwsAccountId, ec2Info.GetAccountID())
					AddAttributeIfNonEmpty(resourceAttrs, entityattributes.AttributeEntityServiceNameSource, entityServiceNameSource)
				}
				p.k8sscraper.Reset()
			} else if p.config.Platform == config.ModeEC2 {
				//If entityServiceNameSource is empty, it was not configured via the config. Get the source in descending priority
				//  1. Incoming telemetry attributes
				//  2. CWA config
				//  3. instance tags - The tags attached to the EC2 instance. Only scrape for tag with the following key: service, application, app
				//  4. IAM Role - The IAM role name retrieved through IMDS(Instance Metadata Service)
				if entityServiceName == EMPTY && entityServiceNameSource == EMPTY {
					entityServiceName, entityServiceNameSource = getServiceNameSource()
				} else if entityServiceName != EMPTY && entityServiceNameSource == EMPTY {
					entityServiceNameSource = entitystore.ServiceNameSourceUnknown
				}

				entityPlatformType = entityattributes.AttributeEntityEC2Platform
				ec2Info = getEC2InfoFromEntityStore()

				if entityEnvironmentName == EMPTY {
					if getAutoScalingGroupFromEntityStore() != EMPTY {
						entityEnvironmentName = entityattributes.DeploymentEnvironmentFallbackPrefix + getAutoScalingGroupFromEntityStore()
					} else {
						entityEnvironmentName = entityattributes.DeploymentEnvironmentDefault
					}
				}

				AddAttributeIfNonEmpty(resourceAttrs, entityattributes.AttributeEntityType, entityattributes.Service)
				AddAttributeIfNonEmpty(resourceAttrs, entityattributes.AttributeEntityServiceName, entityServiceName)
				AddAttributeIfNonEmpty(resourceAttrs, entityattributes.AttributeEntityDeploymentEnvironment, entityEnvironmentName)
				AddAttributeIfNonEmpty(resourceAttrs, entityattributes.AttributeEntityAwsAccountId, ec2Info.GetAccountID())

				ec2Attributes := EC2ServiceAttributes{
					InstanceId:        ec2Info.GetInstanceID(),
					AutoScalingGroup:  getAutoScalingGroupFromEntityStore(),
					ServiceNameSource: entityServiceNameSource,
				}
				if err := validate.Struct(ec2Attributes); err == nil {
					resourceAttrs.PutStr(entityattributes.AttributeEntityPlatformType, entityPlatformType)
					AddAttributeIfNonEmpty(resourceAttrs, entityattributes.AttributeEntityInstanceID, ec2Attributes.InstanceId)
					AddAttributeIfNonEmpty(resourceAttrs, entityattributes.AttributeEntityAutoScalingGroup, ec2Attributes.AutoScalingGroup)
					AddAttributeIfNonEmpty(resourceAttrs, entityattributes.AttributeEntityServiceNameSource, ec2Attributes.ServiceNameSource)
				}
			}
			if logGroupNames == EMPTY || (serviceName == EMPTY && environmentName == EMPTY) {
				continue
			}

			logGroupNamesSlice := strings.Split(logGroupNames, "&")
			for _, logGroupName := range logGroupNamesSlice {
				if logGroupName == EMPTY {
					continue
				}
				addToEntityStore(entitystore.LogGroupName(logGroupName), serviceName, environmentName)
			}
		}

	}
	return md, nil
}

// scrapeServiceAttribute expands the datapoint attributes and search for
// service name and environment attributes. This is only used for components
// that only emit attributes on datapoint level. This code block contains a lot
// of repeated code because OTEL metrics type do not have a common interface.
func (p *awsEntityProcessor) scrapeServiceAttribute(scopeMetric pmetric.ScopeMetricsSlice) (string, string, string) {
	entityServiceName := EMPTY
	entityServiceNameSource := EMPTY
	entityEnvironmentName := EMPTY
	for j := 0; j < scopeMetric.Len(); j++ {
		metric := scopeMetric.At(j).Metrics()
		for k := 0; k < metric.Len(); k++ {
			if entityServiceName != EMPTY && entityEnvironmentName != EMPTY && entityServiceNameSource != EMPTY {
				return entityServiceName, entityEnvironmentName, entityServiceNameSource
			}
			m := metric.At(k)
			switch m.Type() {
			case pmetric.MetricTypeGauge:
				dps := m.Gauge().DataPoints()
				for l := 0; l < dps.Len(); l++ {
					dpService := getServiceAttributes(dps.At(l).Attributes())
					if dpService != EMPTY {
						entityServiceName = dpService
					}
					if dpServiceNameSource, ok := dps.At(l).Attributes().Get(entityattributes.AttributeServiceNameSource); ok {
						entityServiceNameSource = dpServiceNameSource.Str()
						dps.At(l).Attributes().Remove(semconv.AttributeServiceName)
						dps.At(l).Attributes().Remove(entityattributes.AttributeServiceNameSource)
					}
					if dpEnvironment, ok := dps.At(l).Attributes().Get(semconv.AttributeDeploymentEnvironment); ok {
						entityEnvironmentName = dpEnvironment.Str()
					}
					if _, ok := dps.At(l).Attributes().Get(entityattributes.AttributeDeploymentEnvironmentSource); ok {
						dps.At(l).Attributes().Remove(semconv.AttributeDeploymentEnvironment)
						dps.At(l).Attributes().Remove(entityattributes.AttributeDeploymentEnvironmentSource)
					}

				}
			case pmetric.MetricTypeSum:
				dps := m.Sum().DataPoints()
				for l := 0; l < dps.Len(); l++ {
					dpService := getServiceAttributes(dps.At(l).Attributes())
					if dpService != EMPTY {
						entityServiceName = dpService
					}
					if dpServiceNameSource, ok := dps.At(l).Attributes().Get(entityattributes.AttributeServiceNameSource); ok {
						entityServiceNameSource = dpServiceNameSource.Str()
						dps.At(l).Attributes().Remove(semconv.AttributeServiceName)
						dps.At(l).Attributes().Remove(entityattributes.AttributeServiceNameSource)
					}
					if dpEnvironment, ok := dps.At(l).Attributes().Get(semconv.AttributeDeploymentEnvironment); ok {
						entityEnvironmentName = dpEnvironment.Str()
					}
					if _, ok := dps.At(l).Attributes().Get(entityattributes.AttributeDeploymentEnvironmentSource); ok {
						dps.At(l).Attributes().Remove(semconv.AttributeDeploymentEnvironment)
						dps.At(l).Attributes().Remove(entityattributes.AttributeDeploymentEnvironmentSource)
					}
				}
			case pmetric.MetricTypeHistogram:
				dps := m.Histogram().DataPoints()
				for l := 0; l < dps.Len(); l++ {
					dpService := getServiceAttributes(dps.At(l).Attributes())
					if dpService != EMPTY {
						entityServiceName = dpService
					}
					if dpServiceNameSource, ok := dps.At(l).Attributes().Get(entityattributes.AttributeServiceNameSource); ok {
						entityServiceNameSource = dpServiceNameSource.Str()
						dps.At(l).Attributes().Remove(semconv.AttributeServiceName)
						dps.At(l).Attributes().Remove(entityattributes.AttributeServiceNameSource)
					}
					if dpEnvironment, ok := dps.At(l).Attributes().Get(semconv.AttributeDeploymentEnvironment); ok {
						entityEnvironmentName = dpEnvironment.Str()
					}
					if _, ok := dps.At(l).Attributes().Get(entityattributes.AttributeDeploymentEnvironmentSource); ok {
						dps.At(l).Attributes().Remove(semconv.AttributeDeploymentEnvironment)
						dps.At(l).Attributes().Remove(entityattributes.AttributeDeploymentEnvironmentSource)
					}
				}
			case pmetric.MetricTypeExponentialHistogram:
				dps := m.ExponentialHistogram().DataPoints()
				for l := 0; l < dps.Len(); l++ {
					dpService := getServiceAttributes(dps.At(l).Attributes())
					if dpService != EMPTY {
						entityServiceName = dpService
					}
					if dpServiceNameSource, ok := dps.At(l).Attributes().Get(entityattributes.AttributeServiceNameSource); ok {
						entityServiceNameSource = dpServiceNameSource.Str()
						dps.At(l).Attributes().Remove(semconv.AttributeServiceName)
						dps.At(l).Attributes().Remove(entityattributes.AttributeServiceNameSource)
					}
					if dpEnvironment, ok := dps.At(l).Attributes().Get(semconv.AttributeDeploymentEnvironment); ok {
						entityEnvironmentName = dpEnvironment.Str()
					}
					if _, ok := dps.At(l).Attributes().Get(entityattributes.AttributeDeploymentEnvironmentSource); ok {
						dps.At(l).Attributes().Remove(semconv.AttributeDeploymentEnvironment)
						dps.At(l).Attributes().Remove(entityattributes.AttributeDeploymentEnvironmentSource)
					}
				}
			case pmetric.MetricTypeSummary:
				dps := m.Sum().DataPoints()
				for l := 0; l < dps.Len(); l++ {
					dpService := getServiceAttributes(dps.At(l).Attributes())
					if dpService != EMPTY {
						entityServiceName = dpService
					}
					if dpServiceNameSource, ok := dps.At(l).Attributes().Get(entityattributes.AttributeServiceNameSource); ok {
						entityServiceNameSource = dpServiceNameSource.Str()
						dps.At(l).Attributes().Remove(semconv.AttributeServiceName)
						dps.At(l).Attributes().Remove(entityattributes.AttributeServiceNameSource)
					}
					if dpEnvironment, ok := dps.At(l).Attributes().Get(semconv.AttributeDeploymentEnvironment); ok {
						entityEnvironmentName = dpEnvironment.Str()
					}
					if _, ok := dps.At(l).Attributes().Get(entityattributes.AttributeDeploymentEnvironmentSource); ok {
						dps.At(l).Attributes().Remove(semconv.AttributeDeploymentEnvironment)
						dps.At(l).Attributes().Remove(entityattributes.AttributeDeploymentEnvironmentSource)
					}
				}
			default:
				p.logger.Debug("Ignore unknown metric type", zap.String("type", m.Type().String()))
			}

		}
	}
	return entityServiceName, entityEnvironmentName, entityServiceNameSource
}

// scrapeResourceEntityAttribute expands the datapoint attributes and search for
// resource entity related attributes. This is only used for components
// that only emit attributes on datapoint level. This code block contains a lot
// of repeated code because OTEL metrics type do not have a common interface.
func (p *awsEntityProcessor) scrapeResourceEntityAttribute(scopeMetric pmetric.ScopeMetricsSlice) string {
	autoScalingGroup := EMPTY
	for j := 0; j < scopeMetric.Len(); j++ {
		metric := scopeMetric.At(j).Metrics()
		for k := 0; k < metric.Len(); k++ {
			if autoScalingGroup != EMPTY {
				return autoScalingGroup
			}
			m := metric.At(k)
			switch m.Type() {
			case pmetric.MetricTypeGauge:
				dps := m.Gauge().DataPoints()
				for l := 0; l < dps.Len(); l++ {
					if dpAutoScalingGroup, ok := dps.At(l).Attributes().Get(ec2tagger.CWDimensionASG); ok {
						autoScalingGroup = dpAutoScalingGroup.Str()
					}
				}
			case pmetric.MetricTypeSum:
				dps := m.Sum().DataPoints()
				for l := 0; l < dps.Len(); l++ {
					if dpAutoScalingGroup, ok := dps.At(l).Attributes().Get(ec2tagger.CWDimensionASG); ok {
						autoScalingGroup = dpAutoScalingGroup.Str()
					}
				}
			case pmetric.MetricTypeHistogram:
				dps := m.Histogram().DataPoints()
				for l := 0; l < dps.Len(); l++ {
					if dpAutoScalingGroup, ok := dps.At(l).Attributes().Get(ec2tagger.CWDimensionASG); ok {
						autoScalingGroup = dpAutoScalingGroup.Str()
					}
				}
			case pmetric.MetricTypeExponentialHistogram:
				dps := m.ExponentialHistogram().DataPoints()
				for l := 0; l < dps.Len(); l++ {
					if dpAutoScalingGroup, ok := dps.At(l).Attributes().Get(ec2tagger.CWDimensionASG); ok {
						autoScalingGroup = dpAutoScalingGroup.Str()
					}
				}
			case pmetric.MetricTypeSummary:
				dps := m.Sum().DataPoints()
				for l := 0; l < dps.Len(); l++ {
					if dpAutoScalingGroup, ok := dps.At(l).Attributes().Get(ec2tagger.CWDimensionASG); ok {
						autoScalingGroup = dpAutoScalingGroup.Str()
					}
				}
			default:
				p.logger.Debug("Ignore unknown metric type", zap.String("type", m.Type().String()))
			}

		}
	}
	return autoScalingGroup
}

// getServiceAttributes prioritize service name retrieval based on
// following attribute priority
// 1. service.name
// 2. Service
// Service is needed because Container Insights mainly uses Service as
// attribute for customer workflows
// https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Container-Insights-metrics-EKS.html
func getServiceAttributes(p pcommon.Map) string {
	if serviceName, ok := p.Get(semconv.AttributeServiceName); ok {
		return serviceName.Str()
	}
	if serviceName, ok := p.Get(attributeService); ok {
		return serviceName.Str()
	}
	return EMPTY
}

// scrapeK8sPodName gets the k8s pod name which is full pod name from the resource attributes
// This is needed to map the pod to the service/environment
func scrapeK8sPodName(p pcommon.Map) string {
	if podAttr, ok := p.Get(semconv.AttributeK8SPodName); ok {
		return podAttr.Str()
	}
	return EMPTY
}
