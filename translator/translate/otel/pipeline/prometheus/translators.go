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
	if conf.IsSet(LogsKey) {
		destinations = append(destinations, common.CloudWatchLogsKey)
	}
	if conf.IsSet(MetricsKey) {
		destinations = append(destinations, common.AMPKey)
	}

	for _, destination := range destinations {
		translators.Set(NewTranslator(common.WithDestination(destination)))
	}
	return translators
}
