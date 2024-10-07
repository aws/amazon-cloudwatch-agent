// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package host

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/receiver/adapter"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/awscloudwatch"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/awsemf"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/prometheusremotewrite"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/sigv4auth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/pipeline"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/batchprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/rollupprocessor"
	adaptertranslator "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/adapter"
	otlpreceiver "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/otlp"
)

var (
	metricsDestinationsKey = common.ConfigKey(common.MetricsKey, common.MetricsDestinationsKey)
	pipelineSuffix         = map[string]string{
		common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey): "",
		common.ConfigKey(common.LogsKey, common.MetricsCollectedKey):    "/emf",
	}
	pipelineExtensions = map[string]common.TranslatorMap[component.Config]{
		common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey): common.NewTranslatorMap(agenthealth.NewTranslator(component.DataTypeMetrics, []string{agenthealth.OperationPutMetricData})),
		common.ConfigKey(common.LogsKey, common.MetricsCollectedKey):    common.NewTranslatorMap(agenthealth.NewTranslator(component.DataTypeLogs, []string{agenthealth.OperationPutLogEvents})),
	}
	pipelineProcessors = map[string]common.TranslatorMap[component.Config]{
		common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey): common.NewTranslatorMap[component.Config](),
		common.ConfigKey(common.LogsKey, common.MetricsCollectedKey):    common.NewTranslatorMap(batchprocessor.NewTranslatorWithNameAndSection(common.PipelineNameHostDeltaMetrics+"/emf", common.LogsKey)),
	}
	pipelineExporters = map[string]common.TranslatorMap[component.Config]{
		common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey): common.NewTranslatorMap(awscloudwatch.NewTranslator()),
		common.ConfigKey(common.LogsKey, common.MetricsCollectedKey):    common.NewTranslatorMap(awsemf.NewTranslator()),
	}
)

func NewTranslators(conf *confmap.Conf, configSection, os string) (pipeline.TranslatorMap, error) {

	translators := common.NewTranslatorMap[*common.ComponentTranslators]()
	hostReceivers := common.NewTranslatorMap[component.Config]()
	deltaReceivers := common.NewTranslatorMap[component.Config]()

	// Gather adapter receivers
	if configSection == common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey) {
		adapterReceivers, err := adaptertranslator.FindReceiversInConfig(conf, os)
		if err != nil {
			return nil, fmt.Errorf("error finding receivers in config: %w", err)
		}
		adapterReceivers.Range(func(translator common.Translator[component.Config]) {
			if translator.ID().Type() == adapter.Type(common.DiskIOKey) || translator.ID().Type() == adapter.Type(common.NetKey) {
				deltaReceivers.Set(translator)
			} else {
				hostReceivers.Set(translator)
			}
		})
	}

	// Gather OTLP receivers
	switch v := conf.Get(common.ConfigKey(configSection, common.OtlpKey)).(type) {
	case []any:
		for index := range v {
			deltaReceivers.Set(otlpreceiver.NewTranslator(
				otlpreceiver.WithDataType(component.DataTypeMetrics),
				otlpreceiver.WithConfigKey(common.ConfigKey(configSection, common.OtlpKey)),
				common.WithIndex(index),
			))
		}
	case map[string]any:
		deltaReceivers.Set(otlpreceiver.NewTranslator(
			otlpreceiver.WithDataType(component.DataTypeMetrics),
			otlpreceiver.WithConfigKey(common.ConfigKey(configSection, common.OtlpKey)),
		))
	}

	// Publishing to CloudWatch
	if !conf.IsSet(metricsDestinationsKey) ||
		conf.IsSet(common.ConfigKey(metricsDestinationsKey, common.CloudWatchKey)) ||
		configSection == common.ConfigKey(common.LogsKey, common.MetricsCollectedKey) {
		if hostReceivers.Len() != 0 {
			translators.Set(NewTranslator(
				common.PipelineNameHost+pipelineSuffix[configSection],
				hostReceivers,
				pipelineProcessors[configSection],
				pipelineExporters[configSection],
				pipelineExtensions[configSection],
			))
		}
		if deltaReceivers.Len() != 0 {
			translators.Set(NewTranslator(
				common.PipelineNameHostDeltaMetrics+pipelineSuffix[configSection],
				deltaReceivers,
				pipelineProcessors[configSection],
				pipelineExporters[configSection],
				pipelineExtensions[configSection],
			))
		}
	}

	// Publishing to AMP
	if conf.IsSet(metricsDestinationsKey) &&
		conf.IsSet(common.ConfigKey(metricsDestinationsKey, common.AMPKey)) &&
		configSection == common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey) {
		exporters := common.NewTranslatorMap[component.Config](prometheusremotewrite.NewTranslatorWithName(common.AMPKey))
		// PRW exporter does not need the delta conversion.
		receivers := common.NewTranslatorMap[component.Config]()
		receivers.Merge(hostReceivers)
		receivers.Merge(deltaReceivers)
		processors := common.NewTranslatorMap(batchprocessor.NewTranslatorWithNameAndSection(common.PipelineNameHost+"/amp", common.MetricsKey))
		if conf.IsSet(common.MetricsAggregationDimensionsKey) {
			processors.Set(rollupprocessor.NewTranslator())
		}
		translators.Set(NewTranslator(
			common.PipelineNameHost+"/amp",
			receivers,
			processors,
			exporters,
			common.NewTranslatorMap(sigv4auth.NewTranslator()),
		))
	}

	return translators, nil
}
