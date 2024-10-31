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

	AMPSectionKey = common.ConfigKey(common.MetricsKey, common.MetricsDestinationsKey, common.AMPKey)
)

type translator struct {
	name          string
	configSection string
}

var _ common.Translator[*common.ComponentTranslators] = (*translator)(nil)

func NewTranslator(configSection string) common.Translator[*common.ComponentTranslators] {
	t := &translator{
		name:          common.PipelineNamePrometheus,
		configSection: configSection,
	}
	switch t.configSection {
	case LogsKey:
		t.name += "/" + common.CloudWatchKey
	case MetricsKey:
		t.name += "/" + common.AMPKey
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
	var destinations []string
	if t.configSection == LogsKey {
		destinations = append(destinations, common.GetLogsDestinations()...)
	} else if t.configSection == MetricsKey {
		destinations = append(destinations, common.GetMetricsDestinations(conf)...)
	}
	// each matching case returns 1 component translator
	// but this could also follow the translators pattern that handles destinations then merge all returned translators
	for _, destination := range destinations {
		switch destination {
		case common.CloudWatchLogsKey:
			if !conf.IsSet(LogsKey) {
				return nil, fmt.Errorf("pipeline (%s) is missing prometheus configuration under logs section with destination (%s)", t.name, destination)
			}
			return &common.ComponentTranslators{
				Receivers: common.NewTranslatorMap(adapter.NewTranslator(prometheus.SectionKey, LogsKey, time.Minute)),
				Processors: common.NewTranslatorMap(
					batchprocessor.NewTranslatorWithNameAndSection(t.name, common.LogsKey), // prometheus sits under metrics_collected in "logs"
				),
				Exporters:  common.NewTranslatorMap(awsemf.NewTranslatorWithName(common.PipelineNamePrometheus)),
				Extensions: common.NewTranslatorMap(agenthealth.NewTranslator(component.DataTypeLogs, []string{agenthealth.OperationPutLogEvents})),
			}, nil
		case common.AMPKey:
			if !conf.IsSet(MetricsKey) {
				return nil, fmt.Errorf("pipeline (%s) is missing prometheus configuration under metrics section with destination (%s)", t.name, destination)
			}
			if !conf.IsSet(common.ConfigKey(MetricsKey, common.PrometheusConfigPathKey)) {
				return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: fmt.Sprint(common.ConfigKey(MetricsKey, common.PrometheusConfigPathKey))}
			}
			if !conf.IsSet(AMPSectionKey) || !conf.IsSet(common.ConfigKey(AMPSectionKey, common.WorkspaceIDKey)) {
				return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: fmt.Sprint(AMPSectionKey + " or " + common.ConfigKey(AMPSectionKey, common.WorkspaceIDKey))}
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
		}
	}
	return nil, fmt.Errorf("pipeline (%s) does not include supported destination in configuration", t.name)
}
