// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package profiles

import (
	"fmt"
	"log"
	"strings"

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
	pipelineName          = "profiles"
	monitoringService     = "monitoring"
	profilesPath          = "/v1development/profiles"
	defaultHTTPEndpoint   = "127.0.0.1:4318"
	serviceNameAttribute  = "service.name"
	profilesSupportGateID = "service.profilesSupport"
)

var (
	serviceNameKey      = common.ConfigKey(common.ProfilesKey, common.ServiceNameKey)
	httpEndpointKey     = common.ConfigKey(common.ProfilesKey, common.OtlpKey, common.HTTPEndpointKey)
	endpointOverrideKey = common.ConfigKey(common.ProfilesKey, common.EndpointOverrideKey)
)

type translator struct {
}

var _ common.PipelineTranslator = (*translator)(nil)

func NewTranslator() common.PipelineTranslator {
	return &translator{}
}

func (t *translator) ID() pipeline.ID {
	return pipeline.NewIDWithName(xpipeline.SignalProfiles, pipelineName)
}

// Translate creates the profiles pipeline. Never add the batch processor to
// it: batch does not support profiles and fails collector startup.
func (t *translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if conf == nil || !conf.IsSet(common.ProfilesKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: common.ProfilesKey}
	}
	serviceName, _ := common.GetString(conf, serviceNameKey)
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		return nil, fmt.Errorf("%q is required for the profiles pipeline", serviceNameKey)
	}
	region := agent.Global_Config.Region
	if region == "" {
		return nil, fmt.Errorf("region is required for the profiles pipeline")
	}

	enableProfilesSupportGate()

	httpEndpoint, _ := common.GetString(conf, httpEndpointKey)
	if httpEndpoint == "" {
		httpEndpoint = defaultHTTPEndpoint
	}

	sigv4Ext := sigv4auth.NewTranslatorWithService(monitoringService)
	agentHealthExt := agenthealth.NewTranslator(agenthealth.ProfilesName, []string{"*"}, agenthealth.WithAdditionalAuth(sigv4Ext.ID()))
	return &common.ComponentTranslators{
		Receivers: common.NewTranslatorMap(otlp.NewHTTPTranslator(httpEndpoint, common.WithName(pipelineName))),
		Processors: common.NewTranslatorMap(resourceprocessor.NewTranslator(
			resourceprocessor.WithAttributes(map[string]string{serviceNameAttribute: serviceName}),
			common.WithName(pipelineName),
		)),
		Exporters: common.NewTranslatorMap(otlphttp.NewTranslatorWithName(pipelineName,
			otlphttp.EndpointConfig{ProfilesEndpoint: profilesEndpoint(conf, region)},
			otlphttp.WithAuthenticator(agentHealthExt.ID()),
		)),
		Extensions: common.NewTranslatorMap(sigv4Ext, agentHealthExt),
	}, nil
}

// profilesEndpoint returns the profiles_endpoint URL, honoring the endpoint_override host.
func profilesEndpoint(conf *confmap.Conf, region string) string {
	if override, _ := common.GetString(conf, endpointOverrideKey); override != "" {
		if !strings.Contains(override, "://") {
			override = "https://" + override
		}
		return strings.TrimRight(override, "/") + profilesPath
	}
	return common.ServiceEndpoint(monitoringService, region, profilesPath)
}

// enableProfilesSupportGate enables the profiles gate in the translator's process for
// translation-time validation; the collector runtime enables it separately at startup.
// Once profiles graduates upstream the gate is deleted, Set fails, and it is safely skipped.
func enableProfilesSupportGate() {
	if err := featuregate.GlobalRegistry().Set(profilesSupportGateID, true); err != nil {
		log.Printf("W! failed to enable the %s feature gate: %v", profilesSupportGateID, err)
	}
}
