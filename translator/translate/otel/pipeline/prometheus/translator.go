// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"fmt"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/prometheusremotewrite"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/sigv4auth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/rollupprocessor"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected/prometheus"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/awsemf"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/batchprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/adapter"
	otelprom "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/prometheus"
)

type translator struct {
	name        string
	dataType    component.DataType
	destination string
}

type Option func(any)

func WithDestination(destination string) Option {
	return func(a any) {
		if t, ok := a.(*translator); ok {
			t.destination = destination
		}
	}
}

func WithDataType(dataType component.DataType) Option {
	return func(a any) {
		if t, ok := a.(*translator); ok {
			t.dataType = dataType
		}
	}
}

var _ common.Translator[*common.ComponentTranslators] = (*translator)(nil)

func NewTranslator(opts ...Option) common.Translator[*common.ComponentTranslators] {
	t := &translator{name: common.PipelineNamePrometheus}
	for _, opt := range opts {
		opt(t)
	}
	if t.destination != "" {
		t.name += "/" + t.destination
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.dataType, t.name)
}

// Translate creates a pipeline for prometheus if the logs.metrics_collected.prometheus
// section is present.
func (t *translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if conf == nil || !conf.IsSet(common.PrometheusConfigKeys[t.dataType]) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: fmt.Sprint(common.PrometheusConfigKeys[t.dataType])}
	}

	if t.dataType == component.DataTypeMetrics && !conf.IsSet(common.ConfigKey(common.PrometheusConfigKeys[t.dataType], common.PrometheusConfigPathKey)) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: fmt.Sprint(common.ConfigKey(common.PrometheusConfigKeys[t.dataType], common.PrometheusConfigPathKey))}
	}

	// return pipeline based on destination to keep source/destination combinations clearly separated
	// telegraf_prometheus - cloudwatch
	// otel_prometheus - AMP
	switch t.destination {
	case "", common.CloudWatchKey:
		if !conf.IsSet(common.MetricsDestinations) || conf.IsSet(common.ConfigKey(common.MetricsDestinations, common.CloudWatchKey)) {
			return &common.ComponentTranslators{
				Receivers: common.NewTranslatorMap(adapter.NewTranslator(prometheus.SectionKey, common.LogsKey, time.Minute)),
				Processors: common.NewTranslatorMap(
					batchprocessor.NewTranslatorWithNameAndSection(t.name, common.LogsKey), // prometheus sits under metrics_collected in "logs"
				),
				Exporters:  common.NewTranslatorMap(awsemf.NewTranslatorWithName(common.PipelineNamePrometheus)),
				Extensions: common.NewTranslatorMap(agenthealth.NewTranslator(component.DataTypeLogs, []string{agenthealth.OperationPutLogEvents})),
			}, nil
		} else {
			return nil, fmt.Errorf("pipeline (%s) does not have destination (%s) in configuration", t.name, t.destination)
		}
	case common.AMPKey:
		if conf.IsSet(common.MetricsDestinations) && conf.IsSet(common.ConfigKey(common.MetricsDestinations, common.AMPKey)) {
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
		} else {
			return nil, fmt.Errorf("pipeline (%s) does not have destination (%s) in configuration", t.name, t.destination)
		}
	default:
		return nil, fmt.Errorf("pipeline (%s) does not support destination (%s) in configuration", t.name, t.destination)
	}
}
