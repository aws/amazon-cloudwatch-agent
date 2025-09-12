// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package containerinsightsjmx

import (
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/awsemf"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/cumulativetodeltaprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/filterprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/metricstransformprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/resourceprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/transformprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/otlp"
)

var (
	baseKey = common.ConfigKey(common.LogsKey, common.MetricsCollectedKey)
	eksKey  = common.ConfigKey(baseKey, common.KubernetesKey)
	jmxKey  = common.ConfigKey(eksKey, "jmx_container_insights")
)

type translator struct {
}

var _ common.PipelineTranslator = (*translator)(nil)

func NewTranslator() common.PipelineTranslator {
	return &translator{}
}

func (t *translator) ID() pipeline.ID {
	return pipeline.NewIDWithName(pipeline.SignalMetrics, common.PipelineNameContainerInsightsJmx)
}

// Translate creates a pipeline for container insights jmx if the logs.metrics_collected.kubernetes
// section is present.
func (t *translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if conf == nil || !conf.IsSet(jmxKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: jmxKey}
	}
	if !context.CurrentContext().RunInContainer() {
		return nil, nil
	}

	if val, _ := common.GetBool(conf, jmxKey); !val {
		return nil, nil
	}
	translators := common.ComponentTranslators{
		Receivers: otlp.NewTranslators(conf, common.PipelineNameJmx, pipeline.SignalLogs.String()),
		Processors: common.NewTranslatorMap(
			filterprocessor.NewTranslator(common.WithName(common.PipelineNameContainerInsightsJmx)),   // Filter metrics
			resourceprocessor.NewTranslator(common.WithName(common.PipelineNameContainerInsightsJmx)), // Change resource attribute names
			transformprocessor.NewTranslatorWithName(common.PipelineNameContainerInsightsJmx),         // Removes attributes that are not of [ClusterName, Namespace]
			metricstransformprocessor.NewTranslatorWithName(common.PipelineNameContainerInsightsJmx),  // Renames metrics and adds pool and area dimensions
			cumulativetodeltaprocessor.NewTranslator(
				common.WithName(common.PipelineNameContainerInsightsJmx),
				cumulativetodeltaprocessor.WithConfigKeys(jmxKey),
			),
		),
		Exporters: common.NewTranslatorMap(
			awsemf.NewTranslatorWithName(common.PipelineNameContainerInsightsJmx),
		),
		Extensions: common.NewTranslatorMap(
			agenthealth.NewTranslator(agenthealth.LogsName, []string{agenthealth.OperationPutLogEvents}),
			agenthealth.NewTranslatorWithStatusCode(agenthealth.StatusCodeName, nil, true),
		),
	}
	return &translators, nil
}
