// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2tagger

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
)

func TestUnmarshalDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NoError(t, component.UnmarshalConfig(confmap.New(), cfg))
	assert.Equal(t, factory.CreateDefaultConfig(), cfg)
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  component.Config
	}{
		{
			name: "Without_supported_kv",
			cfg:  &Config{},
		},
		{
			name: "Invalid_dimension",
			cfg: &Config{
				EC2MetadataTags: []string{"ImageId", "foo"},
			},
		},
		{
			name: "Valid_config",
			cfg: &Config{
				EC2MetadataTags:    []string{"ImageId", "InstanceId", "InstanceType"},
				EC2InstanceTagKeys: []string{"AutoScalingGroupName"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NoError(t, component.UnmarshalConfig(confmap.New(), tt.cfg))
			err := tt.cfg.(*Config).Validate()
			assert.Nil(t, err, "Empty or invalid dimension tags should be silently ignored")
		})
	}
}
