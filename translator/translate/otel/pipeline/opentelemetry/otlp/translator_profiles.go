// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otlp

import (
	"fmt"

	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/pipeline/xpipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/otlphttp"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/sigv4auth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/resourceprocessor"
	otlpreceiver "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/otlp"
)

const (
	profilesExporterName = "profiles"
	monitoringService    = "monitoring"
	profilesPath         = "/v1development/profiles"
	serviceNameAttribute = "service.name"
	fallbackServiceName  = "unknown_service"
)

type profilesPipelineTranslator struct {
}

var _ common.PipelineTranslator = (*profilesPipelineTranslator)(nil)

func (t *profilesPipelineTranslator) ID() pipeline.ID {
	return pipeline.NewIDWithName(xpipeline.SignalProfiles, pipelineName)
}

func (t *profilesPipelineTranslator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if conf == nil || !conf.IsSet(otlpKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: otlpKey}
	}
	region := agent.Global_Config.Region
	if region == "" {
		return nil, fmt.Errorf("region is required for %s profiles pipeline", common.OpenTelemetryKey)
	}

	sigv4Ext := sigv4auth.NewTranslatorWithService(monitoringService)
	agentHealthExt := agenthealth.NewTranslator(agenthealth.OtelProfilesName, []string{"*"}, agenthealth.WithAdditionalAuth(sigv4Ext.ID()))
	return &common.ComponentTranslators{
		Receivers: otlpreceiver.NewTranslators(conf, pipelineName, otlpKey),
		Processors: common.NewTranslatorMap(resourceprocessor.NewTranslator(
			resourceprocessor.WithAttributes(map[string]string{serviceNameAttribute: fallbackServiceName}),
			resourceprocessor.WithAttributesAction(resourceprocessor.ActionInsert),
			common.WithName(profilesExporterName),
		)),
		Exporters: common.NewTranslatorMap(otlphttp.NewTranslatorWithName(profilesExporterName,
			otlphttp.EndpointConfig{ProfilesEndpoint: common.ServiceEndpoint(monitoringService, region, profilesPath)},
			otlphttp.WithAuthenticator(agentHealthExt.ID()),
		)),
		Extensions: common.NewTranslatorMap(sigv4Ext, agentHealthExt),
	}, nil
}
