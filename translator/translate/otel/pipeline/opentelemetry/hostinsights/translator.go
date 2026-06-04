// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package hostinsights

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/connector/forward"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/hostmetrics"
)

const pipelineNameHostInsights = "host_insights"

var hostInsightsKey = common.ConfigKey(common.OpenTelemetryKey, common.CollectKey, common.HostInsightsKey)

type hostInsightsTranslator struct{}

var _ common.PipelineTranslator = (*hostInsightsTranslator)(nil)

func NewTranslator() common.PipelineTranslator {
	return &hostInsightsTranslator{}
}

func (t *hostInsightsTranslator) ID() pipeline.ID {
	return pipeline.NewIDWithName(pipeline.SignalMetrics, pipelineNameHostInsights)
}

func (t *hostInsightsTranslator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if conf == nil || !conf.IsSet(hostInsightsKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: hostInsightsKey}
	}

	fwdConnector := forward.NewTranslator(common.OpenTelemetryKey)

	return &common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap[component.Config, component.ID](hostmetrics.NewTranslator()),
		Processors: common.NewTranslatorMap[component.Config, component.ID](),
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](fwdConnector),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](),
		Connectors: common.NewTranslatorMap[component.Config, component.ID](fwdConnector),
	}, nil
}
