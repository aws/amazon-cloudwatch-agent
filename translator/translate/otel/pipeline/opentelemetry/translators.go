// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package opentelemetry

import (
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	ci "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/pipeline/opentelemetry/containerinsights"
	dbi "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/pipeline/opentelemetry/databaseinsights"
	fl "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/pipeline/opentelemetry/files"
	hi "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/pipeline/opentelemetry/hostmetrics"
	otelotlp "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/pipeline/opentelemetry/otlp"
	prom "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/pipeline/opentelemetry/prometheus"
	we "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/pipeline/opentelemetry/windowsevents"
)

func NewTranslators(conf *confmap.Conf) common.PipelineTranslatorMap {
	translators := common.NewTranslatorMap[*common.ComponentTranslators, pipeline.ID]()
	translators.Set(NewBaseMetricsTranslator())
	translators.Set(NewBaseLogsTranslator())
	translators.Set(NewBaseTracesTranslator())
	translators.Set(hi.NewTranslator())
	translators.Set(prom.NewTranslator())
	translators.Merge(dbi.NewTranslators(conf))
	translators.Merge(ci.NewTranslators(conf))
	translators.Merge(otelotlp.NewTranslators(conf))
	translators.Merge(we.NewTranslators(conf))
	translators.Merge(fl.NewTranslators(conf))
	return translators
}
