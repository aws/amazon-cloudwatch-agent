// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected/prometheus"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/awsemf"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/prometheusremotewrite"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/sigv4auth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/batchprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/rollupprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/adapter"
	otelprom "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/prometheus"
)

var (
	MetricsKey = common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.PrometheusKey)
	LogsKey    = common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.PrometheusKey)
)

type translator struct {
	name string
	common.DestinationProvider
}

var _ common.Translator[*common.ComponentTranslators] = (*translator)(nil)

func NewTranslator(opts ...common.TranslatorOption) common.Translator[*common.ComponentTranslators] {
	t := &translator{name: common.PipelineNamePrometheus}
	for _, opt := range opts {
		opt(t)
	}
	if t.Destination() != "" {
		t.name += "/" + t.Destination()
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(component.DataTypeMetrics, t.name)
}

// Translate creates a pipeline for prometheus if the logs.metrics_collected.prometheus
// section is present.
func (t *translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if conf == nil || !(conf.IsSet(MetricsKey) || conf.IsSet(LogsKey)) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: fmt.Sprint(MetricsKey + " or " + LogsKey)}
	}

	// return pipeline based on destination to keep source/destination combinations clearly separated
	// telegraf_prometheus - cloudwatch
	// otel_prometheus - AMP
	// this could change in future releases to support different source/destination combinations
	switch t.Destination() {
	case common.CloudWatchLogsKey:
		if !conf.IsSet(LogsKey) {
			return nil, fmt.Errorf("pipeline (%s) is missing prometheus configuration under logs section with destination (%s)", t.name, t.Destination())
		}
		return &common.ComponentTranslators{
			Receivers: common.NewTranslatorMap(adapter.NewTranslator(prometheus.SectionKey, LogsKey, time.Minute)),
			Processors: common.NewTranslatorMap(
				batchprocessor.NewTranslatorWithNameAndSection(t.name, common.LogsKey), // prometheus sits under metrics_collected in "logs"
			),
			Exporters: common.NewTranslatorMap(awsemf.NewTranslatorWithName(common.PipelineNamePrometheus)),
			Extensions: common.NewTranslatorMap(agenthealth.NewTranslator(component.DataTypeLogs, []string{agenthealth.OperationPutLogEvents}),
				agenthealth.NewTranslatorWithStatusCode(component.MustNewType("statuscode"), nil, true)),
		}, nil
	case common.AMPKey:
		if !conf.IsSet(MetricsKey) {
			return nil, fmt.Errorf("pipeline (%s) is missing prometheus configuration under metrics section with destination (%s)", t.name, t.Destination())
		}
		translators := &common.ComponentTranslators{
			Receivers:  common.NewTranslatorMap(otelprom.NewTranslator()),
			Processors: common.NewTranslatorMap(batchprocessor.NewTranslatorWithNameAndSection(t.name, common.MetricsKey)),
			Exporters:  common.NewTranslatorMap(prometheusremotewrite.NewTranslatorWithName(common.AMPKey)),
			Extensions: common.NewTranslatorMap(sigv4auth.NewTranslator()),
		}
		if conf.IsSet(common.MetricsAggregationDimensionsKey) {
			translators.Processors.Set(rollupprocessor.NewTranslator())
		}
		return translators, nil
	default:
		return nil, fmt.Errorf("pipeline (%s) does not support destination %s in configuration", t.name, t.Destination())
	}
}
