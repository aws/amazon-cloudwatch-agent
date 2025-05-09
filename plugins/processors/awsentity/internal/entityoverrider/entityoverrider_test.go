// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entityoverrider

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsentity/entityattributes"
)

func TestNewEntityOverrider(t *testing.T) {
	overrides := &EntityOverride{
		KeyAttributes: []KeyPair{
			{
				Key:   entityattributes.ServiceName,
				Value: "test-service",
			},
		},
	}
	overrider := NewEntityOverrider(overrides, zap.NewNop())
	assert.Equal(t, overrides, overrider.overrides)
}

func TestEntityOverrider_ApplyOverrides(t *testing.T) {
	tests := []struct {
		name      string
		overrides *EntityOverride
		initial   map[string]string
		expected  map[string]string
	}{
		{
			name: "TestValidEntityAttributes",
			overrides: &EntityOverride{
				KeyAttributes: []KeyPair{
					{
						Key:   entityattributes.ServiceName,
						Value: "test-service",
					},
					{
						Key:   entityattributes.DeploymentEnvironment,
						Value: "test-env",
					},
				},
				Attributes: []KeyPair{
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
			name:      "TestNilOverride",
			overrides: nil,
			initial: map[string]string{
				"existing.attribute": "value",
			},
			expected: map[string]string{
				"existing.attribute": "value",
			},
		},
		{
			name: "TestInvalidKeyAttributeOverride",
			overrides: &EntityOverride{
				KeyAttributes: []KeyPair{
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
			name: "TestInvalidAttributeOverride",
			overrides: &EntityOverride{
				Attributes: []KeyPair{
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
			overrider := NewEntityOverrider(tt.overrides, zap.NewNop())
			resourceAttrs := pcommon.NewMap()
			for k, v := range tt.initial {
				resourceAttrs.PutStr(k, v)
			}

			overrider.ApplyOverrides(resourceAttrs)

			assert.Equal(t, len(tt.expected), resourceAttrs.Len())
			for k, v := range tt.expected {
				val, ok := resourceAttrs.Get(k)
				assert.True(t, ok)
				assert.Equal(t, v, val.Str())
			}
		})
	}
}

func TestEntityOverrider_GetOverriddenServiceName(t *testing.T) {
	tests := []struct {
		name        string
		overrides   *EntityOverride
		wantService string
		wantSource  string
	}{
		{
			name: "TestServiceNameAndSourceOverride",
			overrides: &EntityOverride{
				KeyAttributes: []KeyPair{
					{
						Key:   entityattributes.ServiceName,
						Value: "test-service",
					},
				},
				Attributes: []KeyPair{
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
			name: "TestServiceNameOverride",
			overrides: &EntityOverride{
				KeyAttributes: []KeyPair{
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
			name: "TestServiceSourceOverride",
			overrides: &EntityOverride{
				Attributes: []KeyPair{
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
			name:        "TestNilOverride",
			overrides:   nil,
			wantService: "",
			wantSource:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			overrider := NewEntityOverrider(tt.overrides, zap.NewNop())
			service, source := overrider.GetOverriddenServiceName()
			assert.Equal(t, tt.wantService, service)
			assert.Equal(t, tt.wantSource, source)
		})
	}
}
