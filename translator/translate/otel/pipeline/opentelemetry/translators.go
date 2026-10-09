// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package opentelemetry

import (
	"log"

	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	dbi "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/pipeline/opentelemetry/databaseinsights"
	fl "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/pipeline/opentelemetry/files"
	hi "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/pipeline/opentelemetry/hostmetrics"
	otelotlp "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/pipeline/opentelemetry/otlp"
	prom "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/pipeline/opentelemetry/prometheus"
	we "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/pipeline/opentelemetry/windowsevents"
)

// containerInsightsConfigKey is the deprecated opentelemetry.collect.container_insights
// section. It is still accepted by the schema so existing configs keep loading,
// but it no longer produces any pipelines: OTel Container Insights is configured
// by the amazon-cloudwatch-observability Helm chart (and EKS add-on), which owns
// the collector config together with the node-exporter, DCGM, Neuron and
// kube-state-metrics components it depends on.
var containerInsightsConfigKey = common.ConfigKey(common.OpenTelemetryKey, common.CollectKey, common.OtelContainerInsightsKey)

func NewTranslators(conf *confmap.Conf) common.PipelineTranslatorMap {
	if conf != nil && conf.IsSet(containerInsightsConfigKey) {
		log.Printf("W! opentelemetry.collect.container_insights is deprecated and has no effect. Use the amazon-cloudwatch-observability Helm chart or the Amazon CloudWatch Observability EKS add-on to enable Container Insights.")
	}
	translators := common.NewTranslatorMap[*common.ComponentTranslators, pipeline.ID]()
	translators.Set(NewBaseMetricsTranslator())
	translators.Set(NewBaseLogsTranslator())
	translators.Set(NewBaseTracesTranslator())
	translators.Set(hi.NewTranslator())
	translators.Set(prom.NewTranslator())
	translators.Merge(dbi.NewTranslators(conf))
	translators.Merge(otelotlp.NewTranslators(conf))
	translators.Merge(we.NewTranslators(conf))
	translators.Merge(fl.NewTranslators(conf))
	return translators
}
