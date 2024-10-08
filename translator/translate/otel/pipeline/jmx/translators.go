// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package jmx

import (
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/pipeline"
)

const (
	// defaultDestination if not defined
	defaultDestination = ""
)

func NewTranslators(conf *confmap.Conf) pipeline.TranslatorMap {
	translators := common.NewTranslatorMap[*common.ComponentTranslators]()
	var destinations []string
	if conf.IsSet(common.ConfigKey(metricsDestinationsKey, common.CloudWatchKey)) {
		destinations = append(destinations, common.CloudWatchKey)
	}
	if conf.IsSet(common.ConfigKey(metricsDestinationsKey, common.AMPKey)) {
		destinations = append(destinations, common.AMPKey)
	}
	if len(destinations) == 0 {
		destinations = append(destinations, defaultDestination)
	}
	switch v := conf.Get(common.JmxConfigKey).(type) {
	case []any:
		for index := range v {
			for _, destination := range destinations {
				translators.Set(NewTranslator(common.WithIndex(index), WithDestination(destination)))
			}
		}
	case map[string]any:
		for _, destination := range destinations {
			translators.Set(NewTranslator(WithDestination(destination)))
		}
	}
	return translators
}
