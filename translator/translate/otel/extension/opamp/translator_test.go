// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package opampextension

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampextension"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
)

func TestTranslator(t *testing.T) {
	translator := NewTranslator()
	assert.Equal(t, component.NewIDWithName(component.MustNewType("opamp"), "opamp"), translator.ID())
}

func TestTranslate(t *testing.T) {
	t.Run("Empty config", func(t *testing.T) {
		translator := NewTranslator()
		conf := confmap.New()
		
		cfg, err := translator.Translate(conf)
		require.NoError(t, err)
		assert.NotNil(t, cfg)
	})

	t.Run("Basic config", func(t *testing.T) {
		translator := NewTranslator()
		conf := confmap.NewFromStringMap(map[string]any{
			"agent": map[string]any{
				"opamp": map[string]any{
					"instance_uid": "test-instance",
					"ppid":         1234,
					"server": map[string]any{
						"ws": map[string]any{
							"endpoint": "ws://localhost:4320/v1/opamp",
						},
						"http": map[string]any{
							"endpoint":         "http://localhost:4320/v1/opamp",
							"polling_interval": "30s",
						},
					},
				},
			},
		})

		cfg, err := translator.Translate(conf)
		require.NoError(t, err)
		
		// Verify the config is the correct type
		opampCfg, ok := cfg.(*opampextension.Config)
		require.True(t, ok, "Expected *opampextension.Config")
		assert.NotNil(t, opampCfg)
		
		// Verify the config contains the expected values
		assert.Equal(t, "test-instance", opampCfg.InstanceUID)
		assert.Equal(t, int32(1234), opampCfg.PPID)
		assert.NotNil(t, opampCfg.Server)
	})
}