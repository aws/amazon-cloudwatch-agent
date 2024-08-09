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
)

const (
	attributeAwsLogGroupNames            = "aws.log.group.names"
	attributeDeploymentEnvironment       = "deployment.environment"
	attributeServiceName                 = "service.name"
	attributeService                     = "Service"
	attributeEntityServiceName           = "aws.entity.service.name"
	attributeEntityDeploymentEnvironment = "aws.entity.deployment.environment"
	EMPTY                                = ""
)

// exposed as a variable for unit testing
var addToEntityStore = func(logGroupName entitystore.LogGroupName, serviceName string, environmentName string) {
	es := entitystore.GetEntityStore()
	if es == nil {
		return
	}
	es.AddServiceAttrEntryForLogGroup(logGroupName, serviceName, environmentName)
}

// awsEntityProcessor looks for metrics that have the aws.log.group.names and either the service.name or
// deployment.environment resource attributes set, then adds the association between the log group(s) and the
// service/environment names to the entitystore extension.
type awsEntityProcessor struct {
	config *Config
	logger *zap.Logger
}

func newAwsEntityProcessor(config *Config, logger *zap.Logger) *awsEntityProcessor {
	return &awsEntityProcessor{
		config: config,
		logger: logger,
	}
}

func (p *awsEntityProcessor) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	rm := md.ResourceMetrics()
	for i := 0; i < rm.Len(); i++ {
		resourceAttrs := rm.At(i).Resource().Attributes()
		logGroupNames, _ := resourceAttrs.Get(attributeAwsLogGroupNames)
		serviceName, _ := resourceAttrs.Get(attributeServiceName)
		environmentName, _ := resourceAttrs.Get(attributeDeploymentEnvironment)

		entityServiceName := getServiceAttributes(resourceAttrs)
		entityEnvironmentName := environmentName.Str()
		if (entityServiceName == EMPTY || entityEnvironmentName == EMPTY) && p.config.ScrapeDatapointAttribute {
			entityServiceName, entityEnvironmentName = p.scrapeServiceAttribute(rm.At(i).ScopeMetrics())
		}
		if entityServiceName != EMPTY {
			resourceAttrs.PutStr(attributeEntityServiceName, entityServiceName)
		}
		if entityEnvironmentName != EMPTY {
			resourceAttrs.PutStr(attributeEntityDeploymentEnvironment, entityEnvironmentName)
		}

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
