// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsapmprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/aws/amazon-cloudwatch-agent/processor/awsapmprocessor/customconfiguration"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id           component.ID
		expected     component.Config
		errorMessage string
	}{
		{
			id: component.NewIDWithName("awsapm", ""),
			expected: &Config{
				Resolvers: []string{"eks"},
				Rules: []customconfiguration.Rule{
					{
						Selectors: []customconfiguration.Selector{
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
						Selectors: []customconfiguration.Selector{
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
						Selectors: []customconfiguration.Selector{
							{
								Dimension: "Operation",
								Match:     "* /api/visits/*",
							},
							{
								Dimension: "RemoteOperation",
								Match:     "*",
							},
						},
						Replacements: []customconfiguration.Replacement{
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
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()
			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			if tt.expected == nil {
				assert.EqualError(t, component.ValidateConfig(cfg), tt.errorMessage)
				return
			}
			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
