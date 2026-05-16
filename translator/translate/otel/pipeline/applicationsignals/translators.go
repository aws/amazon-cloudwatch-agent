// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package applicationsignals

import (
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func isLogsDisabled(conf *confmap.Conf, configKeys []string) bool {
	for _, key := range configKeys {
		if v, ok := common.GetBool(conf, common.ConfigKey(key, "disable")); ok && v {
			return true
		}
	}
	return false
}

// NewTranslators returns pipeline translators for Application Signals.
// For traces, returns a single pipeline. For metrics/logs, returns 3 pipelines
// (receive, export_1, export_2) connected via a routing connector.
func NewTranslators(_ *confmap.Conf, signal pipeline.Signal) common.PipelineTranslatorMap {
	translators := common.NewTranslatorMap[*common.ComponentTranslators, pipeline.ID]()

	switch signal {
	case pipeline.SignalTraces:
		translators.Set(NewTranslator(signal))
	case pipeline.SignalMetrics:
		translators.Set(NewTranslator(signal, SetVariant(metricsVariantRoute)))
		translators.Set(NewTranslator(signal, SetVariant(metricsVariantLogDest)))
		translators.Set(NewTranslator(signal, SetVariant(metricsVariantOtlpDest)))
	case pipeline.SignalLogs:
		translators.Set(NewTranslator(signal, SetVariant(logsVariantRoute)))
		translators.Set(NewTranslator(signal, SetVariant(logsVariantBatch)))
		translators.Set(NewTranslator(signal, SetVariant(logsVariantNoBatch)))
	}

	return translators
}
