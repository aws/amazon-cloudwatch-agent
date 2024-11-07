// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package containerinsights

import (
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/pipeline"
)

var (
	LogsKey = common.ConfigKey(common.LogsKey, common.MetricsCollectedKey)
)

func NewTranslators(conf *confmap.Conf) pipeline.TranslatorMap {
	translators := common.NewTranslatorMap[*common.ComponentTranslators]()
	// create default container insights translator
	ciTranslator := NewTranslatorWithName(ciPipelineName)
	translators.Set(ciTranslator)
	// create kueue container insights translator
	KueueContainerInsightsEnabled := common.KueueContainerInsightsEnabled(conf)
	if KueueContainerInsightsEnabled {
		kueueTranslator := NewTranslatorWithName(kueuePipelineName)
		translators.Set(kueueTranslator)
	}
	// return the translator map
	return translators
}
