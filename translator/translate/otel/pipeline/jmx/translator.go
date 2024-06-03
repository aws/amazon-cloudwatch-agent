// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package jmx

import (
	"fmt"
	"strconv"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/awscloudwatch"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/prometheusremotewrite"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/sigv4auth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/batchprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/ec2taggerprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/filterprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/metricsdecorator"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/resourceprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/rollupprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/jmx"
)

const (
	placeholderTarget = "<target-system>"
)

var (
	metricsDestinationsKey = common.ConfigKey(common.MetricsKey, common.MetricsDestinationsKey)
)

type translator struct {
	name  string
	index int
}

type Option func(any)

func WithIndex(index int) Option {
	return func(a any) {
		if t, ok := a.(*translator); ok {
			t.index = index
		}
	}
}

var _ common.Translator[*common.ComponentTranslators] = (*translator)(nil)

func NewTranslator(opts ...Option) common.Translator[*common.ComponentTranslators] {
	t := &translator{name: common.PipelineNameJmx, index: -1}
	for _, opt := range opts {
		opt(t)
	}
	if t.index != -1 {
		t.name += "/" + strconv.Itoa(t.index)
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

	if !hasMeasurements(conf, t.index) {
		baseKey := common.JmxConfigKey
		if t.index != -1 {
			baseKey = fmt.Sprintf("%s[%d]", baseKey, t.index)
		}
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: common.ConfigKey(baseKey, placeholderTarget, common.MeasurementKey)}
	}

	translators := common.ComponentTranslators{
		Receivers: common.NewTranslatorMap(jmx.NewTranslator(jmx.WithIndex(t.index))),
		Processors: common.NewTranslatorMap(
			filterprocessor.NewTranslator(filterprocessor.WithName(common.PipelineNameJmx), filterprocessor.WithIndex(t.index)),
			resourceprocessor.NewTranslator(resourceprocessor.WithName(common.PipelineNameJmx)),
		),
		Exporters:  common.NewTranslatorMap[component.Config](),
		Extensions: common.NewTranslatorMap[component.Config](),
	}

	if !conf.IsSet(metricsDestinationsKey) || conf.IsSet(common.ConfigKey(metricsDestinationsKey, "cloudwatch")) {
		translators.Exporters.Set(awscloudwatch.NewTranslator())
		translators.Extensions.Set(agenthealth.NewTranslator(component.DataTypeMetrics, []string{agenthealth.OperationPutMetricData}))
	}
	if conf.IsSet(metricsDestinationsKey) && conf.IsSet(common.ConfigKey(metricsDestinationsKey, common.AMPKey)) {
		translators.Exporters.Set(prometheusremotewrite.NewTranslatorWithName(common.AMPKey))
		translators.Processors.Set(batchprocessor.NewTranslatorWithNameAndSection(t.name, common.MetricsKey))
		if conf.IsSet(common.MetricsAggregationDimensionsKey) {
			translators.Processors.Set(rollupprocessor.NewTranslator())
		}
		translators.Extensions.Set(sigv4auth.NewTranslator())
	}

	mdt := metricsdecorator.NewTranslator(
		metricsdecorator.WithName(common.PipelineNameJmx),
		metricsdecorator.WithIndex(t.index),
		metricsdecorator.WithConfigKey(common.JmxConfigKey),
	)
	if mdt.IsSet(conf) {
		translators.Processors.Set(mdt)
	}

	if conf.IsSet(common.ConfigKey(common.MetricsKey, common.AppendDimensionsKey)) {
		translators.Processors.Set(ec2taggerprocessor.NewTranslator())
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
