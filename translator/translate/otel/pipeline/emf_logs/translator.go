// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package emf_logs

import (
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor/batchprocessor"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/exporter/otel_aws_cloudwatch_logs"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/processor"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/receiver/tcp_logs"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/receiver/udp_logs"
)

var (
	key               = common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.Emf)
	serviceAddressKey = common.ConfigKey(key, common.ServiceAddress)
)

type translator struct {
	id component.ID
}

var _ common.Translator[*common.ComponentTranslators] = (*translator)(nil)

func NewTranslator() common.Translator[*common.ComponentTranslators] {
	return &translator{}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(component.DataTypeLogs, common.PipelineNameEmfLogs)
}

// Translate creates a pipeline for emf if emf logs are collected
// section is present.
func (t *translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if conf == nil || !conf.IsSet(key) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: key}
	}
	translators := common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap[component.Config](),
		Processors: common.NewTranslatorMap(processor.NewDefaultTranslatorWithName(common.PipelineNameEmfLogs, batchprocessor.NewFactory())),
		Exporters:  common.NewTranslatorMap(otel_aws_cloudwatch_logs.NewTranslatorWithName(common.PipelineNameEmfLogs)),
	}
	if serviceAddress, ok := common.GetString(conf, serviceAddressKey); ok {
		if strings.Contains(serviceAddress, common.Udp) {
			translators.Receivers.Add(udp_logs.NewTranslatorWithName(common.PipelineNameEmfLogs))
		} else {
			translators.Receivers.Add(tcp_logs.NewTranslatorWithName(common.PipelineNameEmfLogs))
		}
	} else {
		translators.Receivers = common.NewTranslatorMap(
			udp_logs.NewTranslatorWithName(common.PipelineNameEmfLogs),
			tcp_logs.NewTranslatorWithName(common.PipelineNameEmfLogs),
		)
	}
	return &translators, nil
}
