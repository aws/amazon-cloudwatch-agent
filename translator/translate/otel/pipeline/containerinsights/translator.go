// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package containerinsights

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/awsemf"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/awsentity"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/batchprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/filterprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/gpu"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/groupbyattrsprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/kueue"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/metricstransformprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/awscontainerinsight"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/awscontainerinsightskueue"
)

const (
	ciPipelineName = common.PipelineNameContainerInsights
)

var (
	baseKey = common.ConfigKey(common.LogsKey, common.MetricsCollectedKey)
	eksKey  = common.ConfigKey(baseKey, common.KubernetesKey)
	ecsKey  = common.ConfigKey(baseKey, common.ECSKey)
)

type translator struct {
	pipelineName string
}

var _ common.PipelineTranslator = (*translator)(nil)

func NewTranslator() common.PipelineTranslator {
	return NewTranslatorWithName(ciPipelineName)
}

func NewTranslatorWithName(pipelineName string) common.PipelineTranslator {
	return &translator{pipelineName: pipelineName}
}

func (t *translator) ID() pipeline.ID {
	return pipeline.NewIDWithName(pipeline.SignalMetrics, t.pipelineName)
}

// Translate creates a pipeline for container insights if the logs.metrics_collected.ecs or logs.metrics_collected.kubernetes
// section is present.
func (t *translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if conf == nil || (!conf.IsSet(ecsKey) && !conf.IsSet(eksKey)) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: fmt.Sprint(ecsKey, " or ", eksKey)}
	}

	highFrequencyGPUMetricsEnabled := t.pipelineName == ciPipelineName && awscontainerinsight.IsHighFrequencyGPUMetricsEnabled(conf)
	batchprocessorTelemetryKey := common.LogsKey
	// Use 60s batch period for batch processor if high-frequency GPU metrics are enabled, otherwise use 5s
	if highFrequencyGPUMetricsEnabled {
		batchprocessorTelemetryKey = common.MetricsKey
	}
	// create processor map with
	// - default batch processor
	// - filter processor to drop prometheus metadata
	processors := common.NewTranslatorMap(
		batchprocessor.NewTranslatorWithNameAndSection(t.pipelineName, batchprocessorTelemetryKey),
		filterprocessor.NewTranslator(common.WithName(t.pipelineName)),
	)

	if highFrequencyGPUMetricsEnabled {
		processors.Set(groupbyattrsprocessor.NewTranslatorWithName(t.pipelineName))
	}

	// create exporter map with default emf exporter based on pipeline name
	exporters := common.NewTranslatorMap(awsemf.NewTranslatorWithName(t.pipelineName))
	// create extensions map based on pipeline name
	extensions := common.NewTranslatorMap(agenthealth.NewTranslator(agenthealth.LogsName, []string{agenthealth.OperationPutLogEvents}),
		agenthealth.NewTranslatorWithStatusCode(agenthealth.StatusCodeName, nil, true),
	)

	// create variable for receivers, use switch block below to assign
	var receivers common.TranslatorMap[component.Config, component.ID]

	switch t.pipelineName {
	case ciPipelineName:
		if conf.IsSet(eksKey) {
			processors.Set(awsentity.NewTranslatorWithEntityType(awsentity.Resource, common.PipelineNameContainerInsights, false))
		}
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
	case common.PipelineNameKueue:
		// add prometheus receiver for kueue
		receivers = common.NewTranslatorMap((awscontainerinsightskueue.NewTranslator()))
		processors.Set(kueue.NewTranslatorWithName(t.pipelineName))

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
