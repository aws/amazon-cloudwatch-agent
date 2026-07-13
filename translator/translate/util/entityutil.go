// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"go.opentelemetry.io/collector/confmap"
	semconv "go.opentelemetry.io/collector/semconv/v1.5.0"

	"github.com/aws/amazon-cloudwatch-agent/internal/entity"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsentity/entityattributes"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/awsentity"
)

// For unit testing
var NewTranslatorWithEntityType = awsentity.NewTranslatorWithEntityType
var NewTranslatorWithEntityTypeAndTransform = awsentity.NewTranslatorWithEntityTypeAndTransform

func CreateEntityProcessorFromConfig(name string, configSection string, conf *confmap.Conf) common.ComponentTranslator {
	// Prioritize agent global config first
	serviceName := agent.Global_Config.ServiceName
	environment := agent.Global_Config.DeploymentEnvironment

	if val, _ := common.GetString(conf, common.ConfigKey(configSection, semconv.AttributeServiceName)); val != "" {
		serviceName = val
	}
	if val, _ := common.GetString(conf, common.ConfigKey(configSection, semconv.AttributeDeploymentEnvironment)); val != "" {
		environment = val
	}

	// Only create transform if at least one attribute is present
	if serviceName != "" || environment != "" {
		transform := &entity.Transform{
			KeyAttributes: make([]entity.KeyPair, 0),
			Attributes:    make([]entity.KeyPair, 0),
		}

		if serviceName != "" {
			transform.KeyAttributes = append(transform.KeyAttributes, entity.KeyPair{
				Key:   entityattributes.ServiceName,
				Value: serviceName,
			})
			transform.Attributes = append(transform.Attributes, entity.KeyPair{
				Key:   entityattributes.ServiceNameSource,
				Value: entityattributes.AttributeServiceNameSourceUserConfig,
			})
		}

		if environment != "" {
			transform.KeyAttributes = append(transform.KeyAttributes, entity.KeyPair{
				Key:   entityattributes.DeploymentEnvironment,
				Value: environment,
			})
		}

		return NewTranslatorWithEntityTypeAndTransform(awsentity.Service, name, false, transform)
	}

	return NewTranslatorWithEntityType(awsentity.Service, common.OtlpKey, false)
}
