// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package jmx

import (
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func NewTranslators(conf *confmap.Conf) common.PipelineTranslatorMap {
	translators := common.NewTranslatorMap[*common.ComponentTranslators, pipeline.ID]()
	destinations := common.GetMetricsDestinations(conf)
	switch v := conf.Get(common.JmxConfigKey).(type) {
	case []any:
		for index := range v {
			for _, destination := range destinations {
				translators.Set(NewTranslator(common.WithIndex(index), common.WithDestination(destination)))
			}
		}
	case map[string]any:
		for _, destination := range destinations {
			translators.Set(NewTranslator(common.WithDestination(destination)))
		}
	}
	return translators
}
