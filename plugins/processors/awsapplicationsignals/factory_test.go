// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsapplicationsignals

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/config"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/rules"
)

var expectedRules = []rules.Rule{
	{
		Selectors: []rules.Selector{
			{
				Dimension: "Operation",
				Match:     "* /api/visits/*",
			},
			{
				Dimension: "RemoteOperation",
				Match:     "*",
			},
		},
		Action:   "keep",
		RuleName: "keep01",
	},
	{
		Selectors: []rules.Selector{
			{
				Dimension: "RemoteService",
				Match:     "UnknownRemoteService",
			},
			{
				Dimension: "RemoteOperation",
				Match:     "GetShardIterator",
			},
		},
		Action: "drop",
	},
	{
		Selectors: []rules.Selector{
			{
				Dimension: "Operation",
				Match:     "* /api/visits/*",
			},
			{
				Dimension: "RemoteOperation",
				Match:     "*",
			},
		},
		Replacements: []rules.Replacement{
			{
				TargetDimension: "RemoteOperation",
				Value:           "ListPetsByCustomer",
			},
			{
				TargetDimension: "ResourceTarget",
				Value:           " ",
			},
		},
		Action: "replace",
	},
}

func TestLoadEKSConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		expected     component.Config
		errorMessage string
	}{
		{
			name: "awsapplicationsignals",
			expected: &config.Config{
				Resolvers: []config.Resolver{config.NewEKSResolver("test")},
				Rules:     expectedRules,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newType, _ := component.NewType(tt.name)
			id := component.NewIDWithName(newType, "")
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_eks.yaml"))
			require.NoError(t, err)

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*config.Config)

			sub, err := cm.Sub(id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			if tt.expected == nil {
				assert.EqualError(t, component.ValidateConfig(cfg), tt.errorMessage)
				return
			}
			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)

			validateErr := cfg.Validate()
			assert.Nil(t, validateErr, validateErr)
		})
	}
}

func TestLoadGenericConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		expected     component.Config
		errorMessage string
	}{
		{
			name: "awsapplicationsignals",
			expected: &config.Config{
				Resolvers: []config.Resolver{config.NewGenericResolver("")},
				Rules:     expectedRules,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newType, _ := component.NewType(tt.name)
			id := component.NewIDWithName(newType, "")
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_generic.yaml"))
			require.NoError(t, err)

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*config.Config)

			sub, err := cm.Sub(id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			if tt.expected == nil {
				assert.EqualError(t, component.ValidateConfig(cfg), tt.errorMessage)
				return
			}
			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)

			validateErr := cfg.Validate()
			assert.Nil(t, validateErr, validateErr)
		})
	}
}
