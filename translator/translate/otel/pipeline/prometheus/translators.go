// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/pipeline"
)

func NewTranslators(conf *confmap.Conf) pipeline.TranslatorMap {
	translators := common.NewTranslatorMap[*common.ComponentTranslators]()
	var destinations []string
	if conf.IsSet(common.ConfigKey(common.MetricsDestinations, common.CloudWatchKey)) {
		destinations = append(destinations, common.CloudWatchKey)
	}
	if conf.IsSet(common.ConfigKey(common.MetricsDestinations, common.AMPKey)) {
		destinations = append(destinations, common.AMPKey)
	}
	if len(destinations) == 0 {
		destinations = append(destinations, "")
	}

	for dataType, configKey := range common.PrometheusConfigKeys {
		if conf.IsSet(configKey) {
			for _, destination := range destinations {
				translators.Set(NewTranslator(WithDataType(dataType), WithDestination(destination)))
			}
		}
	}
	return translators
}
