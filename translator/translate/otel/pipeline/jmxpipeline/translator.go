// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package jmxpipeline

import (
	"log"
	"strconv"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/awscloudwatch"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/ec2taggerprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/jmxfilterprocessor"
)

var (
	jmxKey = common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.JmxKey)
)

type translator struct {
	name      string
	index     int
	receivers common.TranslatorMap[component.Config]
}

var _ common.Translator[*common.ComponentTranslators] = (*translator)(nil)

func NewTranslator(name string, index int, receivers common.TranslatorMap[component.Config]) common.Translator[*common.ComponentTranslators] {
	return &translator{name, index, receivers}
}

func (t *translator) ID() component.ID {
	pipelineName := common.PipelineNameJmx + "/" + strconv.Itoa(t.index)
	return component.NewIDWithName(component.DataTypeMetrics, pipelineName)
}

// Translate creates a pipeline for jmx if jmx metrics are collected
// section is present.
func (t *translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if conf == nil || !(conf.IsSet(jmxKey)) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: jmxKey}
	}

	translators := common.ComponentTranslators{
		Receivers:  t.receivers,
		Processors: common.NewTranslatorMap[component.Config](),
		Exporters:  common.NewTranslatorMap(awscloudwatch.NewTranslator()),
		Extensions: common.NewTranslatorMap(agenthealth.NewTranslator(component.DataTypeMetrics, []string{agenthealth.OperationPutMetricData})),
	}

	if conf.IsSet(common.ConfigKey(common.MetricsKey, common.AppendDimensionsKey)) {
		log.Printf("D! ec2tagger processor required because append_dimensions is set")
		translators.Processors.Set(ec2taggerprocessor.NewTranslator())
	}

	if jmxfilterprocessor.IsSet(conf, t.index) {
		log.Printf("D! jmx filter processor required for pipeline %s because target names are set", t.ID())
		translators.Processors.Set(jmxfilterprocessor.NewTranslator(jmxfilterprocessor.WithIndex(t.index)))
	}

	return &translators, nil
}
