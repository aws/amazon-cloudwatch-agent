// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nop

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter/nopexporter"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/receiver/nopreceiver"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/selftelemetry"
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

var _ common.PipelineTranslator = (*translator)(nil)

func NewTranslator() common.PipelineTranslator {
	return &translator{}
}

func (t *translator) ID() pipeline.ID {
	return pipeline.NewIDWithName(pipeline.SignalMetrics, pipelineName)
}

func (t *translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	// Self telemetry needs this placeholder too: a config carrying only service::telemetry has no
	// pipeline, and a pipeline-less service fails validation, so nothing would be emitted at all.
	if conf == nil || !(conf.IsSet(logAgentKey) || selftelemetry.Enabled(conf)) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: fmt.Sprint(logAgentKey)}
	}

	translators := &common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap(receiver.NewDefaultTranslator(nopreceiver.NewFactory())),
		Processors: common.NewTranslatorMap[component.Config, component.ID](),
		Exporters:  common.NewTranslatorMap(exporter.NewDefaultTranslator(nopexporter.NewFactory())),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](),
	}
	return translators, nil
}
