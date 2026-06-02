// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package applicationsignals

import (
	"log"

	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/sigv4auth"
)

func isLogsDisabled(conf *confmap.Conf) bool {
	v, ok := common.GetBool(conf, common.ConfigKey(common.AppSignalsLogs, "disabled"))
	return ok && v
}

// NewTranslators returns pipeline translators for Application Signals.
// For traces, returns a single pipeline. For metrics/logs, returns 3 pipelines
// (receive, export_1, export_2) connected via a routing connector.
// If sigv4auth credentials are unavailable, the OTLP metrics pipeline and
// logs pipelines are skipped to avoid blocking startup for on-prem customers.
func NewTranslators(conf *confmap.Conf, signal pipeline.Signal) common.PipelineTranslatorMap {
	translators := common.NewTranslatorMap[*common.ComponentTranslators, pipeline.ID]()

	switch signal {
	case pipeline.SignalTraces:
		translators.Set(newTranslator(signal))
	case pipeline.SignalMetrics:
		if sigv4auth.CanResolveCredentials() {
			translators.Set(newTranslator(signal, setVariant(metricsVariantRoute)))
			translators.Set(newTranslator(signal, setVariant(metricsVariantLogDest)))
			translators.Set(newTranslator(signal, setVariant(metricsVariantOtlpDest)))
		} else {
			log.Println("W! Skipping Application Signals OTLP metrics pipeline: AWS credentials unavailable for sigv4auth")
			translators.Set(newTranslator(signal, setVariant(metricsVariantDefault)))
		}
	case pipeline.SignalLogs:
		if conf == nil || isLogsDisabled(conf) {
			break
		}
		if !sigv4auth.CanResolveCredentials() {
			log.Println("W! Skipping Application Signals logs pipeline: AWS credentials unavailable for sigv4auth")
			break
		}
		translators.Set(newTranslator(signal, setVariant(logsVariantRoute)))
		translators.Set(newTranslator(signal, setVariant(logsVariantBatch)))
		translators.Set(newTranslator(signal, setVariant(logsVariantNoBatch)))
	}

	return translators
}
