// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package jmxpipeline

import (
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/pipeline"
)

func NewTranslators(conf *confmap.Conf) (pipeline.TranslatorMap, error) {

	translators := common.NewTranslatorMap[*common.ComponentTranslators]()
	switch v := conf.Get(common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.JmxKey)).(type) {
	case []interface{}:
		for index := range v {
			translators.Set(NewTranslator(index))
		}
	case map[string]interface{}:
		translators.Set(NewTranslator(-1))
	}

	return translators, nil
}
