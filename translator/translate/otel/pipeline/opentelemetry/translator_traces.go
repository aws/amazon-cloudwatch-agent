// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package opentelemetry

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/connector/forward"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/otlphttp"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/sigv4auth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/batchprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/resourcedetection"
)

type baseTracesTranslator struct{}

var _ common.PipelineTranslator = (*baseTracesTranslator)(nil)

func NewBaseTracesTranslator() common.PipelineTranslator {
	return &baseTracesTranslator{}
}

func (t *baseTracesTranslator) ID() pipeline.ID {
	return pipeline.NewIDWithName(pipeline.SignalTraces, common.OpenTelemetryKey)
}

// Translate creates the shared traces export pipeline. It activates when a
// traces source (otlp) is configured under opentelemetry.collect.
func (t *baseTracesTranslator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	otlpTracesKey := common.ConfigKey(common.OpenTelemetryKey, common.CollectKey, common.OtlpKey)
	if conf == nil || !conf.IsSet(otlpTracesKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: otlpTracesKey}
	}

	region := agent.Global_Config.Region
	if region == "" {
		return nil, fmt.Errorf("region is required for %s traces pipeline", common.OpenTelemetryKey)
	}
	tracesEndpoint := common.ServiceEndpoint("xray", region, "/v1/traces")
	sigv4Ext := sigv4auth.NewTranslatorWithService("xray")
	agentHealthExt := agenthealth.NewTranslator(agenthealth.TracesName, []string{"*"}, agenthealth.WithAdditionalAuth(sigv4Ext.ID()))

	fwdConnector := forward.NewTranslator(common.OpenTelemetryKey)

	return &common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap[component.Config, component.ID](fwdConnector),
		Processors: common.NewTranslatorMap[component.Config, component.ID](resourcedetection.NewTranslator(), batchprocessor.NewTranslator(common.WithName(common.OpenTelemetryKey))),
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](otlphttp.NewTranslatorWithName("traces", otlphttp.EndpointConfig{TracesEndpoint: tracesEndpoint}, otlphttp.WithAuthenticator(agentHealthExt.ID()))),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](sigv4Ext, agentHealthExt),
		Connectors: common.NewTranslatorMap[component.Config, component.ID](fwdConnector),
	}, nil
}
