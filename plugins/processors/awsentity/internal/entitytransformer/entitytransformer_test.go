// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entitytransformer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/internal/entity"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsentity/entityattributes"
)

func TestNewEntityTransformer(t *testing.T) {
	entityTransform := &entity.Transform{
		KeyAttributes: []entity.KeyPair{
			{
				Key:   entityattributes.ServiceName,
				Value: "test-service",
			},
		},
	}
	entityTransformer := NewEntityTransformer(entityTransform, zap.NewNop())
	assert.Equal(t, entityTransform, entityTransformer.transform)
}

func TestEntityTransformer_ApplyTransformer(t *testing.T) {
	tests := []struct {
		name       string
		transforms *entity.Transform
		initial    map[string]string
		expected   map[string]string
	}{
		{
			name: "TestValidEntityAttributes",
			transforms: &entity.Transform{
				KeyAttributes: []entity.KeyPair{
					{
						Key:   entityattributes.ServiceName,
						Value: "test-service",
					},
					{
						Key:   entityattributes.DeploymentEnvironment,
						Value: "test-env",
					},
				},
				Attributes: []entity.KeyPair{
					{
						Key:   entityattributes.ServiceNameSource,
						Value: "UserConfiguration",
					},
				},
			},
			initial: map[string]string{
				"existing.attribute": "value",
			},
			expected: map[string]string{
				"existing.attribute":                                  "value",
				entityattributes.AttributeEntityServiceName:           "test-service",
				entityattributes.AttributeEntityDeploymentEnvironment: "test-env",
				entityattributes.AttributeEntityServiceNameSource:     "UserConfiguration",
			},
		},
		{
			name:       "TestNilTransform",
			transforms: nil,
			initial: map[string]string{
				"existing.attribute": "value",
			},
			expected: map[string]string{
				"existing.attribute": "value",
			},
		},
		{
			name: "TestInvalidKeyAttributeTransform",
			transforms: &entity.Transform{
				KeyAttributes: []entity.KeyPair{
					{
						Key:   "InvalidKey",
						Value: "test-value",
					},
				},
			},
			initial: map[string]string{
				"existing.attribute": "value",
			},
			expected: map[string]string{
				"existing.attribute": "value",
			},
		},
		{
			name: "TestInvalidAttributeTransform",
			transforms: &entity.Transform{
				Attributes: []entity.KeyPair{
					{
						Key:   "InvalidAttribute",
						Value: "test-value",
					},
				},
			},
			initial: map[string]string{
				"existing.attribute": "value",
			},
			expected: map[string]string{
				"existing.attribute": "value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer := NewEntityTransformer(tt.transforms, zap.NewNop())
			resourceAttrs := pcommon.NewMap()
			for k, v := range tt.initial {
				resourceAttrs.PutStr(k, v)
			}

			transformer.ApplyTransforms(resourceAttrs)

			assert.Equal(t, len(tt.expected), resourceAttrs.Len())
			for k, v := range tt.expected {
				val, ok := resourceAttrs.Get(k)
				assert.True(t, ok)
				assert.Equal(t, v, val.Str())
			}
		})
	}
}

func TestEntityTransformer_GetTransformedServiceName(t *testing.T) {
	tests := []struct {
		name        string
		transform   *entity.Transform
		wantService string
		wantSource  string
	}{
		{
			name: "TestServiceNameAndSourceTransform",
			transform: &entity.Transform{
				KeyAttributes: []entity.KeyPair{
					{
						Key:   entityattributes.ServiceName,
						Value: "test-service",
					},
				},
				Attributes: []entity.KeyPair{
					{
						Key:   entityattributes.ServiceNameSource,
						Value: "UserConfiguration",
					},
				},
			},
			wantService: "test-service",
			wantSource:  "UserConfiguration",
		},
		{
			name: "TestServiceNameTransform",
			transform: &entity.Transform{
				KeyAttributes: []entity.KeyPair{
					{
						Key:   entityattributes.ServiceName,
						Value: "test-service",
					},
				},
			},
			wantService: "test-service",
			wantSource:  "",
		},
		{
			name: "TestServiceSourceTransform",
			transform: &entity.Transform{
				Attributes: []entity.KeyPair{
					{
						Key:   entityattributes.ServiceNameSource,
						Value: "UserConfiguration",
					},
				},
			},
			wantService: "",
			wantSource:  "UserConfiguration",
		},
		{
			name:        "TestNilTransform",
			transform:   nil,
			wantService: "",
			wantSource:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer := NewEntityTransformer(tt.transform, zap.NewNop())
			service, source := transformer.GetOverriddenServiceName()
			assert.Equal(t, tt.wantService, service)
			assert.Equal(t, tt.wantSource, source)
		})
	}
}
