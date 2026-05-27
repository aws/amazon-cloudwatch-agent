// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package opentelemetry

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/connector/forward"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/otlphttp"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/batchprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/resourcedetection"
)

const pipelineNameOtelRoot = "opentelemetry_root"

var otelCollectKey = common.ConfigKey(common.OpenTelemetryKey, common.CollectKey)

type otelRootTranslator struct{}

var _ common.PipelineTranslator = (*otelRootTranslator)(nil)

func NewOtelRootTranslator() common.PipelineTranslator {
	return &otelRootTranslator{}
}

func (t *otelRootTranslator) ID() pipeline.ID {
	return pipeline.NewIDWithName(pipeline.SignalMetrics, pipelineNameOtelRoot)
}

func (t *otelRootTranslator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if conf == nil || !conf.IsSet(otelCollectKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: otelCollectKey}
	}

	defaults, err := newPipelineDefaults(pipelineNameOtelRoot)
	if err != nil {
		return nil, err
	}

	fwdConnector := forward.NewTranslator("otel")

	return &common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap[component.Config, component.ID](fwdConnector),
		Processors: common.NewTranslatorMap[component.Config, component.ID](resourcedetection.NewTranslator(), batchprocessor.NewTranslator(common.WithName(pipelineNameOtelRoot))),
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](otlphttp.NewTranslatorWithName(pipelineNameOtelRoot, defaults.Endpoint, otlphttp.WithAuthenticator(defaults.AuthID))),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](defaults.SigV4Ext),
		Connectors: common.NewTranslatorMap[component.Config, component.ID](fwdConnector),
	}, nil
}
