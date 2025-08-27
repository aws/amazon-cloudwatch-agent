// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package host

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/receiver/adapter"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	adaptertranslator "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/adapter"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/awsnvme"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/hostmetrics"
	otlpreceiver "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/otlp"
)

var (
	MetricsKey = common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey)
	LogsKey    = common.ConfigKey(common.LogsKey, common.MetricsCollectedKey)
)

func NewTranslators(conf *confmap.Conf, configSection, os string) (common.TranslatorMap[*common.ComponentTranslators, pipeline.ID], error) {
	translators := common.NewTranslatorMap[*common.ComponentTranslators, pipeline.ID]()
	hostReceivers := common.NewTranslatorMap[component.Config, component.ID]()
	hostCustomReceivers := common.NewTranslatorMap[component.Config, component.ID]()
	deltaReceivers := common.NewTranslatorMap[component.Config, component.ID]()
	otlpReceivers := common.NewTranslatorMap[component.Config, component.ID]()

	// Gather adapter receivers
	if configSection == MetricsKey {
		adapterReceivers, err := adaptertranslator.FindReceiversInConfig(conf, os)
		if err != nil {
			return nil, fmt.Errorf("error finding receivers in config: %w", err)
		}
		adapterReceivers.Range(func(translator common.ComponentTranslator) {
			if translator.ID().Type() == adapter.Type(common.DiskIOKey) || translator.ID().Type() == adapter.Type(common.NetKey) {
				deltaReceivers.Set(translator)
			} else if translator.ID().Type() == adapter.Type(common.StatsDMetricKey) || translator.ID().Type() == adapter.Type(common.CollectDPluginKey) {
				hostCustomReceivers.Set(translator)
			} else {
				hostReceivers.Set(translator)
			}
		})
	}

	if shouldAddNvmeReceiver(conf, configSection) {
		deltaReceivers.Set(awsnvme.NewTranslator())
	}

	if shouldAddHostmetricsReceiver(conf, configSection) {
		hostReceivers.Set(hostmetrics.NewTranslator())
	}

	// Gather OTLP receivers
	switch v := conf.Get(common.ConfigKey(configSection, common.OtlpKey)).(type) {
	case []any:
		for index := range v {
			otlpReceivers.Set(otlpreceiver.NewTranslator(
				otlpreceiver.WithSignal(pipeline.SignalMetrics),
				otlpreceiver.WithConfigKey(common.ConfigKey(configSection, common.OtlpKey)),
				common.WithIndex(index),
			))
		}
	case map[string]any:
		otlpReceivers.Set(otlpreceiver.NewTranslator(
			otlpreceiver.WithSignal(pipeline.SignalMetrics),
			otlpreceiver.WithConfigKey(common.ConfigKey(configSection, common.OtlpKey)),
		))
	}

	hasHostPipeline := hostReceivers.Len() != 0
	hasHostCustomPipeline := hostCustomReceivers.Len() != 0
	hasDeltaPipeline := deltaReceivers.Len() != 0
	hasOtlpPipeline := otlpReceivers.Len() != 0

	var destinations []string
	switch configSection {
	case LogsKey:
		destinations = common.GetLogsDestinations()
	case MetricsKey:
		destinations = common.GetMetricsDestinations(conf)
	}

	for _, destination := range destinations {
		switch destination {
		case common.AMPKey:
			// PRW exporter does not need the delta conversion.
			receivers := common.NewTranslatorMap[component.Config, component.ID]()
			receivers.Merge(hostReceivers)
			receivers.Merge(deltaReceivers)
			receivers.Merge(otlpReceivers)
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
			if hasHostCustomPipeline {
				translators.Set(NewTranslator(
					common.PipelineNameHostCustomMetrics,
					hostCustomReceivers,
					common.WithDestination(destination)))
			}
			if hasDeltaPipeline {
				translators.Set(NewTranslator(
					common.PipelineNameHostDeltaMetrics,
					deltaReceivers,
					common.WithDestination(destination),
				))
			}
			if hasOtlpPipeline {
				translators.Set(NewTranslator(
					common.PipelineNameHostOtlpMetrics,
					otlpReceivers,
					common.WithDestination(destination),
				))
			}
		}
	}

	return translators, nil
}

func shouldAddNvmeReceiver(conf *confmap.Conf, configSection string) bool {
	diskioMap := conf.Get(common.ConfigKey(configSection, common.DiskIOKey))
	if diskioMap == nil {
		return false
	}
	measurements := common.GetMeasurements(diskioMap.(map[string]any))
	for _, measurement := range measurements {
		if awsnvme.IsNVMEMetric(measurement) {
			return true
		}
	}
	return false
}

func shouldAddHostmetricsReceiver(conf *confmap.Conf, configSection string) bool {
	cpuMap := conf.Get(common.ConfigKey(configSection, common.CPUKey))
	if cpuMap == nil {
		return false
	}
	cpuMapTyped := cpuMap.(map[string]any)
	
	// Check measurements directly since common.GetMeasurements doesn't handle []string
	if measurementValue, ok := cpuMapTyped["measurement"]; ok {
		switch measurements := measurementValue.(type) {
		case []string:
			for _, measurement := range measurements {
				if hostmetrics.IsHostmetricsMetric(measurement) {
					return true
				}
			}
		case []any:
			for _, measurement := range measurements {
				if str, ok := measurement.(string); ok && hostmetrics.IsHostmetricsMetric(str) {
					return true
				}
			}
		}
	}
	return false
}
