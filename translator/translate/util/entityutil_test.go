// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"
	semconv "go.opentelemetry.io/collector/semconv/v1.5.0"

	"github.com/aws/amazon-cloudwatch-agent/internal/entity"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsentity/entityattributes"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/awsentity"
)

func TestCreateEntityProcessorFromConfig(t *testing.T) {
	originalServiceName := agent.Global_Config.ServiceName
	originalEnvironment := agent.Global_Config.DeploymentEnvironment
	originalNewTranslatorWithEntityType := NewTranslatorWithEntityType
	originalNewTranslatorWithEntityTypeAndTransform := NewTranslatorWithEntityTypeAndTransform

	defer func() {
		agent.Global_Config.ServiceName = originalServiceName
		agent.Global_Config.DeploymentEnvironment = originalEnvironment
		NewTranslatorWithEntityType = originalNewTranslatorWithEntityType
		NewTranslatorWithEntityTypeAndTransform = originalNewTranslatorWithEntityTypeAndTransform
	}()

	testCases := []struct {
		name              string
		globalServiceName string
		globalEnvironment string
		configServiceName string
		configEnvironment string
		expectedName      string
		expectedTransform *entity.Transform
	}{
		{
			name:              "Empty config and global values",
			globalServiceName: "",
			globalEnvironment: "",
			configServiceName: "",
			configEnvironment: "",
			expectedName:      common.OtlpKey,
			expectedTransform: nil,
		},
		{
			name:              "Only global service name",
			globalServiceName: "global-service",
			globalEnvironment: "",
			configServiceName: "",
			configEnvironment: "",
			expectedName:      "test-name",
			expectedTransform: &entity.Transform{
				KeyAttributes: []entity.KeyPair{
					{
						Key:   entityattributes.ServiceName,
						Value: "global-service",
					},
				},
				Attributes: []entity.KeyPair{
					{
						Key:   entityattributes.ServiceNameSource,
						Value: entityattributes.AttributeServiceNameSourceUserConfig,
					},
				},
			},
		},
		{
			name:              "Only global environment",
			globalServiceName: "",
			globalEnvironment: "global-env",
			configServiceName: "",
			configEnvironment: "",
			expectedName:      "test-name",
			expectedTransform: &entity.Transform{
				KeyAttributes: []entity.KeyPair{
					{
						Key:   entityattributes.DeploymentEnvironment,
						Value: "global-env",
					},
				},
				Attributes: []entity.KeyPair{},
			},
		},
		{
			name:              "Global values with config overrides",
			globalServiceName: "global-service",
			globalEnvironment: "global-env",
			configServiceName: "config-service",
			configEnvironment: "config-env",
			expectedName:      "test-name",
			expectedTransform: &entity.Transform{
				KeyAttributes: []entity.KeyPair{
					{
						Key:   entityattributes.ServiceName,
						Value: "config-service",
					},
					{
						Key:   entityattributes.DeploymentEnvironment,
						Value: "config-env",
					},
				},
				Attributes: []entity.KeyPair{
					{
						Key:   entityattributes.ServiceNameSource,
						Value: entityattributes.AttributeServiceNameSourceUserConfig,
					},
				},
			},
		},
		{
			name:              "Config service name only",
			globalServiceName: "",
			globalEnvironment: "",
			configServiceName: "config-service",
			configEnvironment: "",
			expectedName:      "test-name",
			expectedTransform: &entity.Transform{
				KeyAttributes: []entity.KeyPair{
					{
						Key:   entityattributes.ServiceName,
						Value: "config-service",
					},
				},
				Attributes: []entity.KeyPair{
					{
						Key:   entityattributes.ServiceNameSource,
						Value: entityattributes.AttributeServiceNameSourceUserConfig,
					},
				},
			},
		},
		{
			name:              "Config environment only",
			globalServiceName: "",
			globalEnvironment: "",
			configServiceName: "",
			configEnvironment: "config-env",
			expectedName:      "test-name",
			expectedTransform: &entity.Transform{
				KeyAttributes: []entity.KeyPair{
					{
						Key:   entityattributes.DeploymentEnvironment,
						Value: "config-env",
					},
				},
				Attributes: []entity.KeyPair{},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			agent.Global_Config.ServiceName = tc.globalServiceName
			agent.Global_Config.DeploymentEnvironment = tc.globalEnvironment

			configMap := make(map[string]interface{})
			const configSection = "metrics"
			if tc.configServiceName != "" {
				configMap[common.ConfigKey(configSection, semconv.AttributeServiceName)] = tc.configServiceName
			}
			if tc.configEnvironment != "" {
				configMap[common.ConfigKey(configSection, semconv.AttributeDeploymentEnvironment)] = tc.configEnvironment
			}
			conf := confmap.NewFromStringMap(configMap)

			var capturedEntityType string
			var capturedName string
			var capturedTransform *entity.Transform
			NewTranslatorWithEntityType = func(entityType string, name string, scrapeDatapointAttribute bool) common.ComponentTranslator {
				capturedEntityType = entityType
				capturedName = name
				return originalNewTranslatorWithEntityType(entityType, name, scrapeDatapointAttribute)
			}
			NewTranslatorWithEntityTypeAndTransform = func(entityType string, name string, scrapeDatapointAttribute bool, transform *entity.Transform) common.ComponentTranslator {
				capturedEntityType = entityType
				capturedName = name
				capturedTransform = transform
				return originalNewTranslatorWithEntityTypeAndTransform(entityType, name, scrapeDatapointAttribute, transform)
			}

			CreateEntityProcessorFromConfig("test-name", configSection, conf)

			assert.Equal(t, awsentity.Service, capturedEntityType, "Entity type should match")
			assert.Equal(t, tc.expectedName, capturedName, "Name should match")

			if tc.expectedTransform == nil {
				assert.Nil(t, capturedTransform, "Transform should be nil")
			} else {
				assert.NotNil(t, capturedTransform, "Transform should not be nil")

				// Verify key attributes
				assert.Equal(t, len(tc.expectedTransform.KeyAttributes), len(capturedTransform.KeyAttributes),
					"Number of key attributes should match")
				for i, expectedKeyAttr := range tc.expectedTransform.KeyAttributes {
					assert.Equal(t, expectedKeyAttr.Key, capturedTransform.KeyAttributes[i].Key,
						"Key attribute key should match")
					assert.Equal(t, expectedKeyAttr.Value, capturedTransform.KeyAttributes[i].Value,
						"Key attribute value should match")
				}

				// Verify attributes
				assert.Equal(t, len(tc.expectedTransform.Attributes), len(capturedTransform.Attributes),
					"Number of attributes should match")
				for i, expectedAttr := range tc.expectedTransform.Attributes {
					assert.Equal(t, expectedAttr.Key, capturedTransform.Attributes[i].Key,
						"Attribute key should match")
					assert.Equal(t, expectedAttr.Value, capturedTransform.Attributes[i].Value,
						"Attribute value should match")
				}
			}
		})
	}
}
