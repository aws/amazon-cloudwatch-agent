// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otlp

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/connector/forward"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/transformprocessor"
	otlpreceiver "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/otlp"
)

const pipelineName = "otlp"

var otlpKey = common.ConfigKey(common.OpenTelemetryKey, common.CollectKey, common.OtlpKey)

// NewTranslators returns OTLP pipeline translators for metrics, logs, and traces.
// Each pipeline creates OTLP receivers (grpc + http) and forwards to the shared base pipeline.
func NewTranslators(conf *confmap.Conf) common.PipelineTranslatorMap {
	translators := common.NewTranslatorMap[*common.ComponentTranslators, pipeline.ID]()
	if conf == nil || !conf.IsSet(otlpKey) {
		return translators
	}

	translators.Set(&otlpPipelineTranslator{signal: pipeline.SignalMetrics})
	translators.Set(&otlpPipelineTranslator{signal: pipeline.SignalLogs})
	translators.Set(&otlpPipelineTranslator{signal: pipeline.SignalTraces})

	return translators
}

type otlpPipelineTranslator struct {
	signal pipeline.Signal
}

var _ common.PipelineTranslator = (*otlpPipelineTranslator)(nil)

func (t *otlpPipelineTranslator) ID() pipeline.ID {
	return pipeline.NewIDWithName(t.signal, pipelineName)
}

func (t *otlpPipelineTranslator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if conf == nil || !conf.IsSet(otlpKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: otlpKey}
	}

	// Use the existing OTLP receiver translator which handles grpc_endpoint/http_endpoint parsing
	receivers := otlpreceiver.NewTranslators(conf, pipelineName, otlpKey)

	fwdConnector := forward.NewTranslator(common.OpenTelemetryKey)

	processors := common.NewTranslatorMap[component.Config, component.ID]()
	processors.Set(transformprocessor.NewTranslatorWithName("otlp_scope",
		transformprocessor.WithErrorMode("ignore"),
		transformprocessor.WithScopeStatements([]string{
			`set(attributes["cloudwatch.source"], "cloudwatch-agent")`,
			`set(attributes["cloudwatch.solution"], "otel-otlp")`,
		}),
	))
	if t.signal == pipeline.SignalLogs {
		processors.Set(transformprocessor.NewTranslatorWithName("otlp_log_source",
			transformprocessor.WithLogStatements([]string{
				`set(resource.attributes["aws.log.source"], "otlp") where resource.attributes["aws.log.source"] == nil`,
			}),
		))
	}

	return &common.ComponentTranslators{
		Receivers:  receivers,
		Processors: processors,
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](fwdConnector),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](),
		Connectors: common.NewTranslatorMap[component.Config, component.ID](fwdConnector),
	}, nil
}
