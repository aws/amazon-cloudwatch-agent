// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsentity

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsentity/entityattributes"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsentity/internal/entitytransformer"
)

func TestUnmarshalDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NoError(t, confmap.New().Unmarshal(cfg))
	assert.Equal(t, factory.CreateDefaultConfig(), cfg)
}

func TestUnmarshalConfig(t *testing.T) {
	tests := []struct {
		name        string
		conf        *confmap.Conf
		expected    *Config
		expectError bool
	}{
		{
			name: "TestValidEntityTransform",
			conf: confmap.NewFromStringMap(map[string]interface{}{
				"entity_type": entityattributes.Service,
				"platform":    "ec2",
				"transform_entity": map[string]interface{}{
					"key_attributes": []interface{}{
						map[string]interface{}{
							"key":   entityattributes.ServiceName,
							"value": "config-service-name",
						},
						map[string]interface{}{
							"key":   entityattributes.DeploymentEnvironment,
							"value": "config-environment-name",
						},
					},
					"attributes": []interface{}{
						map[string]interface{}{
							"key":   entityattributes.ServiceNameSource,
							"value": "UserConfiguration",
						},
					},
				},
			}),
			expected: &Config{
				EntityType: entityattributes.Service,
				Platform:   "ec2",
				TransformEntity: &entitytransformer.EntityTransform{
					KeyAttributes: []entitytransformer.KeyPair{
						{
							Key:   entityattributes.ServiceName,
							Value: "config-service-name",
						},
						{
							Key:   entityattributes.DeploymentEnvironment,
							Value: "config-environment-name",
						},
					},
					Attributes: []entitytransformer.KeyPair{
						{
							Key:   entityattributes.ServiceNameSource,
							Value: "UserConfiguration",
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "TestInvalidEntityTransform",
			conf: confmap.NewFromStringMap(map[string]interface{}{
				"entity_type": entityattributes.Service,
				"platform":    "ec2",
				"transform_entity": map[string]interface{}{
					"key_attributes": []interface{}{
						map[string]interface{}{
							"key":   "InvalidKey",
							"value": "some-value",
						},
					},
				},
			}),
			expectError: true,
		},
		{
			name: "TestEmptyEntityTransform",
			conf: confmap.NewFromStringMap(map[string]interface{}{
				"entity_type": entityattributes.Service,
				"platform":    "ec2",
			}),
			expected: &Config{
				EntityType: entityattributes.Service,
				Platform:   "ec2",
			},
			expectError: false,
		},
		{
			name: "TestMissingRequiredFieldEntityTransform",
			conf: confmap.NewFromStringMap(map[string]interface{}{
				"transform_entity": map[string]interface{}{
					"key_attributes": []interface{}{
						map[string]interface{}{
							"key":   entityattributes.ServiceName,
							"value": "",
						},
					},
				},
			}),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			err := tt.conf.Unmarshal(cfg)

			assert.NoError(t, err)

			// Validate the configuration
			err = cfg.(*Config).Validate()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if err == nil {
				assert.Equal(t, tt.expected, cfg)
			}
		})
	}
}
