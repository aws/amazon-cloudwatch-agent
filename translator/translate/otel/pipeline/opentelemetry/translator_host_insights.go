// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package opentelemetry

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/otlphttp"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/resourcedetection"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/systemmetrics"
)

const (
	pipelineNameHostInsights = "host_insights"
)

var hostInsightsKey = common.ConfigKey(common.OpenTelemetryKey, common.CollectKey, common.HostInsightsKey)

type hostInsightsTranslator struct{}

var _ common.PipelineTranslator = (*hostInsightsTranslator)(nil)

func NewHostInsightsTranslator() common.PipelineTranslator {
	return &hostInsightsTranslator{}
}

func (t *hostInsightsTranslator) ID() pipeline.ID {
	return pipeline.NewIDWithName(pipeline.SignalMetrics, pipelineNameHostInsights)
}

func (t *hostInsightsTranslator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if conf == nil || !conf.IsSet(hostInsightsKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: hostInsightsKey}
	}

	defaults, err := newPipelineDefaults(pipelineNameHostInsights)
	if err != nil {
		return nil, err
	}

	return &common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap[component.Config, component.ID](systemmetrics.NewTranslator()),
		Processors: common.NewTranslatorMap[component.Config, component.ID](resourcedetection.NewTranslator()),
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](otlphttp.NewTranslatorWithName(pipelineNameHostInsights, defaults.Endpoint, otlphttp.WithAuthenticator(defaults.AuthID))),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](defaults.SigV4Ext),
	}, nil
}
