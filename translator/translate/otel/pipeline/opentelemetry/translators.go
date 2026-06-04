// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package opentelemetry

import (
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	ci "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/pipeline/opentelemetry/containerinsights"
	dbi "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/pipeline/opentelemetry/databaseinsights"
)

func NewTranslators(conf *confmap.Conf) common.PipelineTranslatorMap {
	translators := common.NewTranslatorMap[*common.ComponentTranslators, pipeline.ID]()
	translators.Set(NewBaseMetricsTranslator())
	translators.Set(NewBaseLogsTranslator())
	translators.Set(NewHostInsightsTranslator())
	translators.Merge(dbi.NewTranslators(conf))
	translators.Merge(ci.NewTranslators(conf))
	return translators
}
