// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package opampextension

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
)

func TestTranslator(t *testing.T) {
	translator := NewTranslator()
	assert.Equal(t, component.MustNewID("opamp"), translator.ID())
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
		
		// Verify the config contains the expected values
		cfgMap := cfg.(*confmap.Conf)
		assert.Equal(t, "test-instance", cfgMap.Get("instance_uid"))
		assert.Equal(t, 1234, cfgMap.Get("ppid"))
		assert.NotNil(t, cfgMap.Get("server"))
		
		// Verify both WS and HTTP server configs are present
		server := cfgMap.Get("server").(map[string]any)
		assert.NotNil(t, server["ws"])
		assert.NotNil(t, server["http"])
		
		ws := server["ws"].(map[string]any)
		assert.Equal(t, "ws://localhost:4320/v1/opamp", ws["endpoint"])
		
		http := server["http"].(map[string]any)
		assert.Equal(t, "http://localhost:4320/v1/opamp", http["endpoint"])
		assert.Equal(t, "30s", http["polling_interval"])
	})
}