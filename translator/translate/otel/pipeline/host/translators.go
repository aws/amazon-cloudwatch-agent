// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package host

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/receiver/adapter"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/pipeline"
	adaptertranslator "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/adapter"
	otlpreceiver "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/otlp"
)

var (
	MetricsKey = common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey)
	LogsKey    = common.ConfigKey(common.LogsKey, common.MetricsCollectedKey)
)

func NewTranslators(conf *confmap.Conf, configSection, os string) (pipeline.TranslatorMap, error) {
	translators := common.NewTranslatorMap[*common.ComponentTranslators]()
	hostReceivers := common.NewTranslatorMap[component.Config]()
	deltaReceivers := common.NewTranslatorMap[component.Config]()

	// Gather adapter receivers
	if configSection == MetricsKey {
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

	hasHostPipeline := hostReceivers.Len() != 0
	hasDeltaPipeline := deltaReceivers.Len() != 0

	var destinations []string
	switch configSection {
	case LogsKey:
		destinations = []string{common.CloudWatchLogsKey}
	case MetricsKey:
		destinations = common.GetMetricsDestinations(conf)
	}

	for _, destination := range destinations {
		switch destination {
		case common.AMPKey:
			// PRW exporter does not need the delta conversion.
			receivers := common.NewTranslatorMap[component.Config]()
			receivers.Merge(hostReceivers)
			receivers.Merge(deltaReceivers)
			translators.Set(NewTranslator(
				common.PipelineNameHost,
				receivers,
				common.WithDestination(destination),
			))
		default:
			if hasHostPipeline {
				translators.Set(NewTranslator(
					common.PipelineNameHost,
					hostReceivers,
					common.WithDestination(destination),
				))
			}
			if hasDeltaPipeline {
				translators.Set(NewTranslator(
					common.PipelineNameHostDeltaMetrics,
					deltaReceivers,
					common.WithDestination(destination),
				))
			}
		}
	}

	return translators, nil
}
