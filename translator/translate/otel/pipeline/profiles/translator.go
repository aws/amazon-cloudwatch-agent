// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package profiles

import (
	"fmt"
	"log"

	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/pipeline/xpipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/otlphttp"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/sigv4auth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/resourceprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/otlp"
)

const (
	pipelineName          = "otlp"
	exporterName          = "profiles"
	monitoringService     = "monitoring"
	profilesPath          = "/v1development/profiles"
	serviceNameAttribute  = "service.name"
	fallbackServiceName   = "unknown_service"
	profilesSupportGateID = "service.profilesSupport"
)

var otlpKey = common.ConfigKey(common.OpenTelemetryKey, common.CollectKey, common.OtlpKey)

func init() {
	if err := featuregate.GlobalRegistry().Set(profilesSupportGateID, true); err != nil {
		log.Printf("W! failed to enable the %s feature gate: %v", profilesSupportGateID, err)
	}
}

type translator struct {
}

var _ common.PipelineTranslator = (*translator)(nil)

func NewTranslator() common.PipelineTranslator {
	return &translator{}
}

func (t *translator) ID() pipeline.ID {
	return pipeline.NewIDWithName(xpipeline.SignalProfiles, pipelineName)
}

func (t *translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if conf == nil || !conf.IsSet(otlpKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: otlpKey}
	}
	region := agent.Global_Config.Region
	if region == "" {
		return nil, fmt.Errorf("region is required for the profiles pipeline")
	}

	sigv4Ext := sigv4auth.NewTranslatorWithService(monitoringService)
	agentHealthExt := agenthealth.NewTranslator(agenthealth.ProfilesName, []string{"*"}, agenthealth.WithAdditionalAuth(sigv4Ext.ID()))
	return &common.ComponentTranslators{
		Receivers: otlp.NewTranslators(conf, pipelineName, otlpKey),
		Processors: common.NewTranslatorMap(resourceprocessor.NewTranslator(
			resourceprocessor.WithAttributes(map[string]string{serviceNameAttribute: fallbackServiceName}),
			resourceprocessor.WithAttributesAction("insert"),
			common.WithName(exporterName),
		)),
		Exporters: common.NewTranslatorMap(otlphttp.NewTranslatorWithName(exporterName,
			otlphttp.EndpointConfig{ProfilesEndpoint: common.ServiceEndpoint(monitoringService, region, profilesPath)},
			otlphttp.WithAuthenticator(agentHealthExt.ID()),
		)),
		Extensions: common.NewTranslatorMap(sigv4Ext, agentHealthExt),
	}, nil
}
