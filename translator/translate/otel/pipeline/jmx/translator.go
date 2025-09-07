// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package jmx

import (
	"fmt"
	"strconv"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/awscloudwatch"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/prometheusremotewrite"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/sigv4auth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/batchprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/cumulativetodeltaprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/deltatocumulativeprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/ec2taggerprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/filterprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/metricsdecorator"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/metricstransformprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/resourceprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/rollupprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/transformprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/jmx"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/otlp"
)

const (
	placeholderTarget = "<target-system>"
)

type translator struct {
	name string
	common.IndexProvider
	common.DestinationProvider
}

var _ common.PipelineTranslator = (*translator)(nil)

func NewTranslator(opts ...common.TranslatorOption) common.PipelineTranslator {
	t := &translator{name: common.PipelineNameJmx}
	t.SetIndex(-1)
	for _, opt := range opts {
		opt(t)
	}
	if t.Destination() != "" {
		t.name += "/" + t.Destination()
	}
	if t.Index() != -1 {
		t.name += "/" + strconv.Itoa(t.Index())
	}
	return t
}

func (t *translator) ID() pipeline.ID {
	return pipeline.NewIDWithName(pipeline.SignalMetrics, t.name)
}

// Translate creates a pipeline for jmx if jmx metrics are collected
// section is present.
func (t *translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if conf == nil || !conf.IsSet(common.JmxConfigKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: common.JmxConfigKey}
	}

	if !hasMeasurements(conf, t.Index()) {
		baseKey := common.JmxConfigKey
		if t.Index() != -1 {
			baseKey = fmt.Sprintf("%s[%d]", baseKey, t.Index())
		}
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: common.ConfigKey(baseKey, placeholderTarget, common.MeasurementKey)}
	}

	translators := common.ComponentTranslators{
		Receivers: common.NewTranslatorMap[component.Config, component.ID](),
		Processors: common.NewTranslatorMap(
			filterprocessor.NewTranslator(common.WithName(common.PipelineNameJmx), common.WithIndex(t.Index())),
		),
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](),
	}

	if context.CurrentContext().RunInContainer() {
		translators.Receivers.Merge(otlp.NewTranslators(conf, common.PipelineNameJmx, pipeline.SignalMetrics.String()))
		translators.Processors.Set(metricstransformprocessor.NewTranslatorWithName(common.JmxKey))
		if hasAppendDimensions(conf, t.Index()) {
			translators.Processors.Set(resourceprocessor.NewTranslator(common.WithName(common.PipelineNameJmx), common.WithIndex(t.Index())))
		}
		translators.Processors.Set(transformprocessor.NewTranslatorWithName(common.JmxKey + "/drop"))
	} else {
		translators.Receivers.Set(jmx.NewTranslator(jmx.WithIndex(t.Index())))
		translators.Processors.Set(resourceprocessor.NewTranslator(common.WithName(common.PipelineNameJmx)))
	}

	mdt := metricsdecorator.NewTranslator(
		metricsdecorator.WithName(common.PipelineNameJmx),
		metricsdecorator.WithIndex(t.Index()),
		metricsdecorator.WithConfigKey(common.JmxConfigKey),
	)
	if mdt.IsSet(conf) {
		translators.Processors.Set(mdt)
	}

	if conf.IsSet(common.ConfigKey(common.MetricsKey, common.AppendDimensionsKey)) {
		translators.Processors.Set(ec2taggerprocessor.NewTranslator())
	}

	switch t.Destination() {
	case common.DefaultDestination, common.CloudWatchKey:
		translators.Processors.Set(cumulativetodeltaprocessor.NewTranslator(common.WithName(common.PipelineNameJmx), cumulativetodeltaprocessor.WithConfigKeys(common.JmxConfigKey)))
		translators.Exporters.Set(awscloudwatch.NewTranslator())
		translators.Extensions.Set(agenthealth.NewTranslatorWithStatusCode(agenthealth.MetricsName, []string{agenthealth.OperationPutMetricData}, true))
	case common.AMPKey:
		translators.Processors.Set(batchprocessor.NewTranslatorWithNameAndSection(t.name, common.MetricsKey))
		if conf.IsSet(common.MetricsAggregationDimensionsKey) {
			translators.Processors.Set(rollupprocessor.NewTranslator())
		}
		// prometheusremotewrite doesn't support delta metrics so convert them to cumulative metrics
		translators.Processors.Set(deltatocumulativeprocessor.NewTranslator(common.WithName(t.name)))
		translators.Exporters.Set(prometheusremotewrite.NewTranslatorWithName(common.AMPKey))
		translators.Extensions.Set(sigv4auth.NewTranslator())
	default:
		return nil, fmt.Errorf("pipeline (%s) does not support destination (%s) in configuration", t.name, t.Destination())
	}

	return &translators, nil
}

func hasMeasurements(conf *confmap.Conf, index int) bool {
	jmxMap := common.GetIndexedMap(conf, common.JmxConfigKey, index)
	if len(jmxMap) == 0 {
		return false
	}
	var result bool
	for _, target := range common.JmxTargets {
		if targetMap, ok := jmxMap[target].(map[string]any); ok {
			if measurements, ok := targetMap[common.MeasurementKey].([]any); !ok || len(measurements) == 0 {
				return false
			}
			result = true
		}
	}
	return result
}

func hasAppendDimensions(conf *confmap.Conf, index int) bool {
	jmxMap := common.GetIndexedMap(conf, common.JmxConfigKey, index)
	if len(jmxMap) == 0 {
		return false
	}
	appendDimensions, ok := jmxMap[common.AppendDimensionsKey].(map[string]any)
	if !ok {
		return false
	}
	return len(appendDimensions) > 0
}
