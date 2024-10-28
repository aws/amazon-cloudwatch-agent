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
	destinations := common.GetMetricsDestinations(conf)

	for dataType, configKey := range common.PrometheusConfigKeys {
		if conf.IsSet(configKey) {
			for _, destination := range destinations {
				translators.Set(NewTranslator(WithDataType(dataType), common.WithDestination(destination)))
			}
		}
	}
	return translators
}
