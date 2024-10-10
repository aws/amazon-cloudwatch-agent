// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package jmx

import (
	"fmt"
	"strconv"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/awscloudwatch"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/prometheusremotewrite"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/sigv4auth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/batchprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/cumulativetodeltaprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/ec2taggerprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/filterprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/metricsdecorator"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/metricstransformprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/resourceprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/rollupprocessor"
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

var _ common.Translator[*common.ComponentTranslators] = (*translator)(nil)

func NewTranslator(opts ...common.TranslatorOption) common.Translator[*common.ComponentTranslators] {
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

func (t *translator) ID() component.ID {
	return component.NewIDWithName(component.DataTypeMetrics, t.name)
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
		Receivers: common.NewTranslatorMap[component.Config](),
		Processors: common.NewTranslatorMap(
			filterprocessor.NewTranslator(common.WithName(common.PipelineNameJmx), common.WithIndex(t.Index())),
		),
		Exporters:  common.NewTranslatorMap[component.Config](),
		Extensions: common.NewTranslatorMap[component.Config](),
	}

	if context.CurrentContext().RunInContainer() {
		translators.Receivers.Set(otlp.NewTranslator(common.WithName(common.PipelineNameJmx)))
		translators.Processors.Set(metricstransformprocessor.NewTranslatorWithName(common.JmxKey))
		if hasAppendDimensions(conf, t.Index()) {
			translators.Processors.Set(resourceprocessor.NewTranslator(common.WithName(common.PipelineNameJmx), common.WithIndex(t.Index())))
		}
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
		translators.Extensions.Set(agenthealth.NewTranslator(component.DataTypeMetrics, []string{agenthealth.OperationPutMetricData}))
	case common.AMPKey:
		translators.Processors.Set(batchprocessor.NewTranslatorWithNameAndSection(t.name, common.MetricsKey))
		if conf.IsSet(common.MetricsAggregationDimensionsKey) {
			translators.Processors.Set(rollupprocessor.NewTranslator())
		}
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
