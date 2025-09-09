// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otlp

import (
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

// NewTranslators creates OTLP receiver translators based on configuration
func NewTranslators(conf *confmap.Conf, pipelineName string, configKey string) common.TranslatorMap[component.Config, component.ID] {
	translators := common.NewTranslatorMap[component.Config, component.ID]()

	// signal can be extracted from configKey
	// two possible cases: 1) just the signal [eg. logs] 2) configKey contains full OR partial config path [eg. logs>metrics_collected>otlp]
	var signal pipeline.Signal
	switch {
	case strings.HasPrefix(configKey, common.LogsKey):
		signal = pipeline.SignalLogs
	case strings.HasPrefix(configKey, common.TracesKey):
		signal = pipeline.SignalTraces
	default:
		signal = pipeline.SignalMetrics
	}

	// this is for appsignals backword compatibility but can potentially be removed now
	if pipelineName == common.AppSignals {
		appSignalsConfigKeys := common.AppSignalsConfigKeys[signal]
		if conf.IsSet(appSignalsConfigKeys[0]) {
			configKey = appSignalsConfigKeys[0]
		} else {
			configKey = appSignalsConfigKeys[1]
		}
	}

	switch v := conf.Get(configKey).(type) {
	case []any:
		for index := range v {
			epConfigs := TranslateToEndpointConfig(conf, pipelineName, configKey, index)
			for _, epConfig := range epConfigs {
				translators.Set(NewTranslator(epConfig))
			}
		}
	default: // default handles an empty otlp section as well
		if conf.IsSet(configKey) {
			epConfigs := TranslateToEndpointConfig(conf, pipelineName, configKey, -1)
			for _, epConfig := range epConfigs {
				translators.Set(NewTranslator(epConfig))
			}
		}
	}

	return translators
}
