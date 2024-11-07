// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package containerinsights

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/awsemf"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/batchprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/gpu"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/kueue"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/metricstransformprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/awscontainerinsight"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/awscontainerinsightskueue"
)

const (
	ciPipelineName    = "containerinsights"
	kueuePipelineName = "kueueContainerInsights"
)

var (
	baseKey = common.ConfigKey(common.LogsKey, common.MetricsCollectedKey)
	eksKey  = common.ConfigKey(baseKey, common.KubernetesKey)
	ecsKey  = common.ConfigKey(baseKey, common.ECSKey)
)

type translator struct {
	pipelineName string
}

var _ common.Translator[*common.ComponentTranslators] = (*translator)(nil)

func NewTranslator() common.Translator[*common.ComponentTranslators] {
	return NewTranslatorWithName(ciPipelineName)
}

func NewTranslatorWithName(pipelineName string) common.Translator[*common.ComponentTranslators] {
	return &translator{pipelineName: pipelineName}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(component.DataTypeMetrics, t.pipelineName)
}

// Translate creates a pipeline for container insights if the logs.metrics_collected.ecs or logs.metrics_collected.kubernetes
// section is present.
func (t *translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if conf == nil || (!conf.IsSet(ecsKey) && !conf.IsSet(eksKey)) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: fmt.Sprint(ecsKey, " or ", eksKey)}
	}

	// create processor map with default batch processor based on pipeline name
	processors := common.NewTranslatorMap(batchprocessor.NewTranslatorWithNameAndSection(t.pipelineName, common.LogsKey))
	// create exporter map with default emf exporter based on pipeline name
	exporters := common.NewTranslatorMap(awsemf.NewTranslatorWithName(t.pipelineName))
	// create extensions map based on pipeline name
	extensions := common.NewTranslatorMap(agenthealth.NewTranslator(component.DataTypeLogs, []string{agenthealth.OperationPutLogEvents}))
	// create variable for receivers, use switch block below to assign
	var receivers common.TranslatorMap[component.Config]

	switch t.pipelineName {
	case ciPipelineName:
		// add aws container insights receiver
		receivers = common.NewTranslatorMap(awscontainerinsight.NewTranslator())
		// Append the metricstransformprocessor only if enhanced container insights is enabled
		enhancedContainerInsightsEnabled := awscontainerinsight.EnhancedContainerInsightsEnabled(conf)
		if enhancedContainerInsightsEnabled {
			// add metricstransformprocessor to processors for enhanced container insights
			processors.Set(metricstransformprocessor.NewTranslatorWithName(t.pipelineName))
			acceleratedComputeMetricsEnabled := awscontainerinsight.AcceleratedComputeMetricsEnabled(conf)
			if acceleratedComputeMetricsEnabled {
				processors.Set(gpu.NewTranslatorWithName(t.pipelineName))
			}
		}
	case kueuePipelineName:
		// add prometheus receiver for kueue
		receivers = common.NewTranslatorMap((awscontainerinsightskueue.NewTranslator()))
		KueueContainerInsightsEnabled := KueueContainerInsightsEnabled(conf)
		if KueueContainerInsightsEnabled {
			processors.Set(kueue.NewTranslatorWithName(t.pipelineName))
		}
	default:
		return nil, fmt.Errorf("unknown container insights pipeline name: %s", t.pipelineName)
	}

	return &common.ComponentTranslators{
		Receivers:  receivers,
		Processors: processors, // EKS & ECS CI sit under metrics_collected in "logs"
		Exporters:  exporters,
		Extensions: extensions,
	}, nil
}

func KueueContainerInsightsEnabled(conf *confmap.Conf) bool {
	return common.GetOrDefaultBool(conf, common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.KubernetesKey, common.EnableKueueContainerInsights), false)
}
