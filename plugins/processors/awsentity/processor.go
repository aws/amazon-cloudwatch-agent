// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsentity

import (
	"context"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	semconv "go.opentelemetry.io/collector/semconv/v1.22.0"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/extension/entitystore"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsentity/internal/entityattributes"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsentity/internal/k8sattributescraper"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
)

const (
	attributeAwsLogGroupNames      = "aws.log.group.names"
	attributeDeploymentEnvironment = "deployment.environment"
	attributeServiceName           = "service.name"
	attributeService               = "Service"
	EMPTY                          = ""
)

type scraper interface {
	Scrape(rm pcommon.Resource)
	Reset()
}

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
	var entityServiceNameSource string
	rm := md.ResourceMetrics()
	for i := 0; i < rm.Len(); i++ {
		if p.config.KubernetesMode != "" {
			p.k8sscraper.Scrape(rm.At(i).Resource())
		}
		resourceAttrs := rm.At(i).Resource().Attributes()
		logGroupNames, _ := resourceAttrs.Get(attributeAwsLogGroupNames)
		serviceName, _ := resourceAttrs.Get(attributeServiceName)
		environmentName, _ := resourceAttrs.Get(attributeDeploymentEnvironment)
		if serviceNameSource, sourceExists := resourceAttrs.Get(entityattributes.AttributeEntityServiceNameSource); sourceExists {
			entityServiceNameSource = serviceNameSource.Str()
		}

		entityServiceName := getServiceAttributes(resourceAttrs)
		entityEnvironmentName := environmentName.Str()
		if (entityServiceName == EMPTY || entityEnvironmentName == EMPTY) && p.config.ScrapeDatapointAttribute {
			entityServiceName, entityEnvironmentName = p.scrapeServiceAttribute(rm.At(i).ScopeMetrics())
		}
		if entityServiceName != EMPTY {
			resourceAttrs.PutStr(entityattributes.AttributeEntityServiceName, entityServiceName)
		}
		if entityEnvironmentName != EMPTY {
			resourceAttrs.PutStr(entityattributes.AttributeEntityDeploymentEnvironment, entityEnvironmentName)
		}
		if p.config.KubernetesMode != "" {
			fallbackEnvironment := entityEnvironmentName
			podInfo, ok := p.k8sscraper.(*k8sattributescraper.K8sAttributeScraper)
			if fallbackEnvironment == EMPTY && p.config.KubernetesMode == config.ModeEKS && ok && podInfo.Cluster != EMPTY && podInfo.Namespace != EMPTY {
				fallbackEnvironment = "eks:" + p.config.ClusterName + "/" + podInfo.Namespace
			} else if fallbackEnvironment == EMPTY && (p.config.KubernetesMode == config.ModeK8sEC2 || p.config.KubernetesMode == config.ModeK8sOnPrem) && ok && podInfo.Cluster != EMPTY && podInfo.Namespace != EMPTY {
				fallbackEnvironment = "k8s:" + p.config.ClusterName + "/" + podInfo.Namespace
			}
			fullPodName := scrapeK8sPodName(resourceAttrs)
			if fullPodName != EMPTY && entityServiceName != EMPTY && entityServiceNameSource != EMPTY {
				addPodToServiceEnvironmentMap(fullPodName, entityServiceName, fallbackEnvironment, entityServiceNameSource)
			} else if fullPodName != EMPTY && entityServiceName != EMPTY && entityServiceNameSource == EMPTY {
				addPodToServiceEnvironmentMap(fullPodName, entityServiceName, fallbackEnvironment, entitystore.ServiceNameSourceUnknown)
			}
		}
		p.k8sscraper.Reset()
		if logGroupNames.Str() == EMPTY || (serviceName.Str() == EMPTY && environmentName.Str() == EMPTY) {
			continue
		}

		logGroupNamesSlice := strings.Split(logGroupNames.Str(), "&")
		for _, logGroupName := range logGroupNamesSlice {
			if logGroupName == EMPTY {
				continue
			}
			addToEntityStore(entitystore.LogGroupName(logGroupName), serviceName.Str(), environmentName.Str())
		}
	}
	return md, nil
}

// scrapeServiceAttribute expands the datapoint attributes and search for
// service name and environment attributes. This is only used for components
// that only emit attributes on datapoint level.
func (p *awsEntityProcessor) scrapeServiceAttribute(scopeMetric pmetric.ScopeMetricsSlice) (string, string) {
	entityServiceName := EMPTY
	entityEnvironmentName := EMPTY
	for j := 0; j < scopeMetric.Len(); j++ {
		metric := scopeMetric.At(j).Metrics()
		for k := 0; k < metric.Len(); k++ {
			if entityServiceName != EMPTY && entityEnvironmentName != EMPTY {
				return entityServiceName, entityEnvironmentName
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
					if dpEnvironment, ok := dps.At(l).Attributes().Get(semconv.AttributeDeploymentEnvironment); ok {
						entityEnvironmentName = dpEnvironment.Str()
					}
				}
			case pmetric.MetricTypeSum:
				dps := m.Sum().DataPoints()
				for l := 0; l < dps.Len(); l++ {
					dpService := getServiceAttributes(dps.At(l).Attributes())
					if dpService != EMPTY {
						entityServiceName = dpService
					}
					if dpEnvironment, ok := dps.At(l).Attributes().Get(semconv.AttributeDeploymentEnvironment); ok {
						entityEnvironmentName = dpEnvironment.Str()
					}
				}
			case pmetric.MetricTypeHistogram:
				dps := m.Histogram().DataPoints()
				for l := 0; l < dps.Len(); l++ {
					dpService := getServiceAttributes(dps.At(l).Attributes())
					if dpService != EMPTY {
						entityServiceName = dpService
					}
					if dpEnvironment, ok := dps.At(l).Attributes().Get(semconv.AttributeDeploymentEnvironment); ok {
						entityEnvironmentName = dpEnvironment.Str()
					}
				}
			case pmetric.MetricTypeExponentialHistogram:
				dps := m.ExponentialHistogram().DataPoints()
				for l := 0; l < dps.Len(); l++ {
					dpService := getServiceAttributes(dps.At(l).Attributes())
					if dpService != EMPTY {
						entityServiceName = dpService
					}
					if dpEnvironment, ok := dps.At(l).Attributes().Get(semconv.AttributeDeploymentEnvironment); ok {
						entityEnvironmentName = dpEnvironment.Str()
					}
				}
			case pmetric.MetricTypeSummary:
				dps := m.Sum().DataPoints()
				for l := 0; l < dps.Len(); l++ {
					dpService := getServiceAttributes(dps.At(l).Attributes())
					if dpService != EMPTY {
						entityServiceName = dpService
					}
					if dpEnvironment, ok := dps.At(l).Attributes().Get(semconv.AttributeDeploymentEnvironment); ok {
						entityEnvironmentName = dpEnvironment.Str()
					}
				}
			default:
				p.logger.Debug("Ignore unknown metric type", zap.String("type", m.Type().String()))
			}

		}
	}
	return entityServiceName, entityEnvironmentName
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
