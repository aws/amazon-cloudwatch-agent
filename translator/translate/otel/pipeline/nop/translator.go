// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nop

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter/nopexporter"
	"go.opentelemetry.io/collector/receiver/nopreceiver"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver"
)

const (
	pipelineName = "nop"
)

var (
	traceKey    = common.ConfigKey(common.TracesKey)
	metricKey   = common.ConfigKey(common.MetricsKey)
	emfKey      = common.ConfigKey(common.LogsKey, common.MetricsCollectedKey)
	logAgentKey = common.ConfigKey(common.LogsKey, common.LogsCollectedKey)
)

type translator struct {
}

var _ common.Translator[*common.ComponentTranslators] = (*translator)(nil)

func NewTranslator() common.Translator[*common.ComponentTranslators] {
	return &translator{}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(component.DataTypeMetrics, pipelineName)
}

func (t *translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if conf == nil || !conf.IsSet(logAgentKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: fmt.Sprint(logAgentKey)}
	}

	translators := &common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap(receiver.NewDefaultTranslator(nopreceiver.NewFactory())),
		Processors: common.NewTranslatorMap[component.Config](),
		Exporters:  common.NewTranslatorMap(exporter.NewDefaultTranslator(nopexporter.NewFactory())),
		Extensions: common.NewTranslatorMap[component.Config](),
	}
	return translators, nil
}
