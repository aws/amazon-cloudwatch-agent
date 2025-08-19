// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package journald_logs

import (
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/awscloudwatchlogs"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/batchprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/journald"
)

var (
	journaldKey = common.ConfigKey(common.LogsKey, common.LogsCollectedKey, common.JournaldKey)
)



type translator struct{}

var _ common.PipelineTranslator = (*translator)(nil)

func NewTranslator() common.PipelineTranslator {
	return &translator{}
}

func (t *translator) ID() pipeline.ID {
	return pipeline.NewIDWithName(pipeline.SignalLogs, common.PipelineNameJournaldLogs)
}

// Translate creates a pipeline for journald logs if the journald section is present.
func (t *translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if conf == nil || !conf.IsSet(journaldKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: journaldKey}
	}

	translators := common.ComponentTranslators{
		Receivers: common.NewTranslatorMap(
			journald.NewTranslatorWithName(common.PipelineNameJournaldLogs),
		),
		Processors: common.NewTranslatorMap(
			batchprocessor.NewTranslatorWithNameAndSection(common.PipelineNameJournaldLogs, common.LogsKey),
		),
		Exporters: common.NewTranslatorMap(
			awscloudwatchlogs.NewTranslatorWithName(common.PipelineNameJournaldLogs),
		),
		Extensions: common.NewTranslatorMap(
			agenthealth.NewTranslator(agenthealth.LogsName, []string{agenthealth.OperationPutLogEvents}),
			agenthealth.NewTranslatorWithStatusCode(agenthealth.StatusCodeName, nil, true),
		),
	}

	return &translators, nil
}