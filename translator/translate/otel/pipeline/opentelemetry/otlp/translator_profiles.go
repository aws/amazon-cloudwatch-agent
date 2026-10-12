// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otlp

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/pipeline/xpipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/otlphttp"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/sigv4auth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/k8sattributesprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/resourcedetection"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/resourceprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/transformprocessor"
	otlpreceiver "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/otlp"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
)

const (
	profilesExporterName   = "profiles"
	monitoringService      = "monitoring"
	profilesPath           = "/v1development/profiles"
	serviceNameAttribute   = "service.name"
	ecsTaskFamilyAttribute = "aws.ecs.task.family"
	fallbackServiceName    = "unknown_service"

	platformKubernetes = "kubernetes"
	platformECS        = "ecs"
	platformEC2        = "ec2"
	platformOther      = ""
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

	platform := inferencePlatform()
	processors := common.NewTranslatorMap[component.Config, component.ID]()
	extensions := common.NewTranslatorMap[component.Config, component.ID](sigv4Ext, agentHealthExt)
	processors.Set(newOtlpScopeTranslator())
	if resourceAttrs := resourceprocessor.NewResourceAttributesTranslator(conf); resourceAttrs != nil {
		processors.Set(resourceAttrs)
	}
	if platform == platformECS {
		processors.Set(resourcedetection.NewTranslator(resourcedetection.WithName(profilesExporterName), resourcedetection.WithECSConfig(resourcedetection.ProfilesECSResourceDetectionConfig)))
		extensions.Set(agenthealth.NewTranslatorWithStatusCode(agenthealth.StatusCodeName, nil, true))
	} else {
		processors.Set(resourcedetection.NewTranslator(resourcedetection.WithName(common.OpenTelemetryKey)))
	}
	if platform == platformKubernetes {
		processors.Set(k8sattributesprocessor.NewTranslator(profilesExporterName, k8sattributesprocessor.WithPodIPAssociation()))
		setClusterName, err := transformprocessor.NewSetClusterNameTranslator(conf)
		if err != nil {
			return nil, err
		}
		if setClusterName != nil {
			processors.Set(setClusterName)
		}
	}
	processors.Set(transformprocessor.NewTranslatorWithName(common.Identity))
	processors.Set(resourceprocessor.NewTranslator(
		resourceprocessor.WithOrderedActions(profilesServiceNameActions(platform)),
		common.WithName(profilesExporterName),
	))

	return &common.ComponentTranslators{
		Receivers:  otlpreceiver.NewTranslators(conf, pipelineName, otlpKey),
		Processors: processors,
		Exporters: common.NewTranslatorMap(otlphttp.NewTranslatorWithName(profilesExporterName,
			otlphttp.EndpointConfig{ProfilesEndpoint: common.ServiceEndpoint(monitoringService, region, profilesPath)},
			otlphttp.WithAuthenticator(agentHealthExt.ID()),
		)),
		Extensions: extensions,
	}, nil
}

func inferencePlatform() string {
	if context.CurrentContext().KubernetesMode() != "" {
		return platformKubernetes
	}
	if context.CurrentContext().Mode() != config.ModeEC2 {
		return platformOther
	}
	if ecsutil.GetECSUtilSingleton().IsECS() {
		return platformECS
	}
	return platformEC2
}

func profilesServiceNameActions(platform string) []resourceprocessor.AttributeAction {
	actions := make([]resourceprocessor.AttributeAction, 0, 2)
	switch platform {
	case platformECS:
		actions = append(actions, resourceprocessor.AttributeAction{Action: resourceprocessor.ActionInsert, Key: serviceNameAttribute, FromAttribute: ecsTaskFamilyAttribute})
	case platformEC2:
		if inferred := EC2ServiceNameProvider(); inferred != "" {
			actions = append(actions, resourceprocessor.AttributeAction{Action: resourceprocessor.ActionInsert, Key: serviceNameAttribute, Value: inferred})
		}
	}
	return append(actions, resourceprocessor.AttributeAction{Action: resourceprocessor.ActionInsert, Key: serviceNameAttribute, Value: fallbackServiceName})
}
