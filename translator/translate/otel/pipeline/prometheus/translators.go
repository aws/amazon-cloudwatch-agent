// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func NewTranslators(conf *confmap.Conf) common.PipelineTranslatorMap {
	translators := common.NewTranslatorMap[*common.ComponentTranslators, pipeline.ID]()
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
