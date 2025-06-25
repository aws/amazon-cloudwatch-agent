// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"fmt"
	"log"
	"time"

	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected/prometheus"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/awscloudwatch"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/awsemf"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/debug"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/prometheusremotewrite"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/sigv4auth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/batchprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/cumulativetodeltaprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/deltatocumulativeprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/ec2taggerprocessor"
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

var _ common.PipelineTranslator = (*translator)(nil)

func NewTranslator(opts ...common.TranslatorOption) common.PipelineTranslator {
	t := &translator{name: common.PipelineNamePrometheus}
	for _, opt := range opts {
		opt(t)
	}
	if t.Destination() != "" {
		t.name += "/" + t.Destination()
	}
	return t
}

func (t *translator) ID() pipeline.ID {
	return pipeline.NewIDWithName(pipeline.SignalMetrics, t.name)
}

// Translate creates a pipeline for prometheus if the logs.metrics_collected.prometheus
// section is present.
func (t *translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if conf == nil || !(conf.IsSet(MetricsKey) || conf.IsSet(LogsKey)) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: fmt.Sprint(MetricsKey + " or " + LogsKey)}
	}

	// return pipeline based on destination to keep source/destination combinations clearly separated
	// otel_prometheus - cloudwatch
	// telegraf_prometheus - cloudwatch
	// otel_prometheus - AMP
	// this could change in future releases to support different source/destination combinations
	switch t.Destination() {
	case common.CloudWatchKey:
		if !conf.IsSet(MetricsKey) {
			return nil, fmt.Errorf("pipeline (%s) is missing prometheus configuration under metrics section with destination (%s)", t.name, t.Destination())
		}
		translators := &common.ComponentTranslators{
			Receivers: common.NewTranslatorMap(otelprom.NewTranslator()),
			Processors: common.NewTranslatorMap(
				batchprocessor.NewTranslatorWithNameAndSection(t.name, common.MetricsKey),
				cumulativetodeltaprocessor.NewTranslator(common.WithName(t.name), cumulativetodeltaprocessor.WithDefaultKeys()),
			),
			Exporters: common.NewTranslatorMap(
				awscloudwatch.NewTranslator(),
				debug.NewTranslator(),
			),
			Extensions: common.NewTranslatorMap(
				agenthealth.NewTranslator(agenthealth.MetricsName, []string{agenthealth.OperationPutMetricData}),
				agenthealth.NewTranslatorWithStatusCode(agenthealth.StatusCodeName, nil, true),
			),
		}

		if conf.IsSet(common.MetricsAggregationDimensionsKey) {
			translators.Processors.Set(rollupprocessor.NewTranslator())
		}

		if conf.IsSet(common.ConfigKey(common.MetricsKey, common.AppendDimensionsKey)) {
			log.Printf("D! ec2tagger processor required because append_dimensions is set")
			translators.Processors.Set(ec2taggerprocessor.NewTranslator())
		}

		return translators, nil
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
			Extensions: common.NewTranslatorMap(agenthealth.NewTranslator(agenthealth.LogsName, []string{agenthealth.OperationPutLogEvents}),
				agenthealth.NewTranslatorWithStatusCode(agenthealth.StatusCodeName, nil, true)),
		}, nil
	case common.AMPKey:
		if !conf.IsSet(MetricsKey) {
			return nil, fmt.Errorf("pipeline (%s) is missing prometheus configuration under metrics section with destination (%s)", t.name, t.Destination())
		}
		translators := &common.ComponentTranslators{
			Receivers: common.NewTranslatorMap(otelprom.NewTranslator()),
			Processors: common.NewTranslatorMap(
				batchprocessor.NewTranslatorWithNameAndSection(t.name, common.MetricsKey),
				// prometheusremotewrite doesn't support delta metrics so convert them to cumulative metrics
				deltatocumulativeprocessor.NewTranslator(common.WithName(t.name)),
			),
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
