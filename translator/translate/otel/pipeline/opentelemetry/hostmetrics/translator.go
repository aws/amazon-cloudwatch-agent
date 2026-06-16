// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package hostmetrics

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/connector/forward"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/hostmetrics"
)

const pipelineNameHostMetrics = "host_metrics"

var hostMetricsKey = common.ConfigKey(common.OpenTelemetryKey, common.CollectKey, common.HostMetricsKey)

type hostMetricsTranslator struct{}

var _ common.PipelineTranslator = (*hostMetricsTranslator)(nil)

func NewTranslator() common.PipelineTranslator {
	return &hostMetricsTranslator{}
}

func (t *hostMetricsTranslator) ID() pipeline.ID {
	return pipeline.NewIDWithName(pipeline.SignalMetrics, pipelineNameHostMetrics)
}

func (t *hostMetricsTranslator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if conf == nil || (!conf.IsSet(hostMetricsKey) && !conf.IsSet(common.DatabaseInsightsConfigKey)) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: hostMetricsKey + " or " + common.DatabaseInsightsConfigKey}
	}

	var opts []hostmetrics.Option
	opts = append(opts, hostmetrics.WithName(common.OpenTelemetryKey))
	if conf.IsSet(common.DatabaseInsightsConfigKey) {
		opts = append(opts, hostmetrics.WithProcessScraper(map[string]any{
			"include": map[string]any{
				"match_type": "regexp",
				"names":      []string{"postgres.*"},
			},
			"mute_process_all_errors": true,
			"metrics": map[string]any{
				"process.cpu.utilization": map[string]any{
					"enabled": true,
				},
				"process.memory.utilization": map[string]any{
					"enabled": true,
				},
			},
		}))
	}

	fwdConnector := forward.NewTranslator(common.OpenTelemetryKey)

	return &common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap[component.Config, component.ID](hostmetrics.NewTranslator(opts...)),
		Processors: common.NewTranslatorMap[component.Config, component.ID](),
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](fwdConnector),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](),
		Connectors: common.NewTranslatorMap[component.Config, component.ID](fwdConnector),
	}, nil
}
