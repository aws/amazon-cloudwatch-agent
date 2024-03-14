// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package jmxpipeline

import (
	"strconv"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/pipeline"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/jmx"
)

func NewTranslators(conf *confmap.Conf) (pipeline.TranslatorMap, error) {

	translators := common.NewTranslatorMap[*common.ComponentTranslators]()
	switch v := conf.Get(common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.JmxKey)).(type) {
	case []interface{}:
		for index, _ := range v {
			jmxReceivers := common.NewTranslatorMap[component.Config]()

			jmxReceivers.Set(jmx.NewTranslator(jmx.WithIndex(index)))
			name := common.PipelineNameJmx + "/" + strconv.Itoa(index)

			translators.Set(NewTranslator(name, index, jmxReceivers))
		}
	case map[string]interface{}:
		jmxReceivers := common.NewTranslatorMap[component.Config]()
		jmxReceivers.Set(jmx.NewTranslator())
		translators.Set(NewTranslator(common.PipelineNameJmx, -1, jmxReceivers))
	}

	return translators, nil
}
