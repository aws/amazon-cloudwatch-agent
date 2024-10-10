// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"fmt"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/prometheusremotewrite"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/sigv4auth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/rollupprocessor"
	"strconv"
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

var (
	logsKey    = common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.PrometheusKey)
	metricsKey = common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.PrometheusKey)
)

type translator struct {
	name        string
	index       int
	destination string
}

type Option func(any)

func WithIndex(index int) Option {
	return func(a any) {
		if t, ok := a.(*translator); ok {
			t.index = index
		}
	}
}

func WithDestination(destination string) Option {
	return func(a any) {
		if t, ok := a.(*translator); ok {
			t.destination = destination
		}
	}
}

var _ common.Translator[*common.ComponentTranslators] = (*translator)(nil)

func NewTranslator(opts ...Option) common.Translator[*common.ComponentTranslators] {
	t := &translator{name: common.PipelineNamePrometheus, index: -1}
	for _, opt := range opts {
		opt(t)
	}
	if t.destination != "" {
		t.name += "/" + t.destination
	}
	if t.index != -1 {
		t.name += "/" + strconv.Itoa(t.index)
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(component.DataTypeMetrics, common.PipelineNamePrometheus)
}

// Translate creates a pipeline for prometheus if the logs.metrics_collected.prometheus
// section is present.
func (t *translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if conf == nil || (!conf.IsSet(logsKey) && !conf.IsSet(metricsKey)) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: fmt.Sprint(logsKey, " or ", metricsKey)}
	}

	switch t.destination {
	case "", common.CloudWatchKey:
		if !conf.IsSet(common.MetricsDestinations) || conf.IsSet(common.ConfigKey(common.MetricsDestinations, common.CloudWatchKey)) {
			return &common.ComponentTranslators{
				Receivers: common.NewTranslatorMap(adapter.NewTranslator(prometheus.SectionKey, logsKey, time.Minute)),
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
