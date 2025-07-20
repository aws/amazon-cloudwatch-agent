// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package opampextension

import (
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampextension"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
)

func TestTranslator(t *testing.T) {
	translator := NewTranslator()
	assert.Equal(t, component.NewIDWithName(component.MustNewType("opamp"), ""), translator.ID())
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
					"server": map[string]any{
						"ws": map[string]any{
							"endpoint": "ws://localhost:4320/v1/opamp",
						},
						"http": map[string]any{
							"endpoint":         "http://localhost:4320/v1/opamp",
							"polling_interval": "30s",
						},
					},
					"ppid":         1234,
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
		assert.Equal(t, int32(1234), opampCfg.PPID)
		assert.NotNil(t, opampCfg.Server)

    
    // Check that exactly one of HTTP or WebSocket is configured
   		if opampCfg.Server.HTTP != nil {
        	assert.Nil(t, opampCfg.Server.WS, "When HTTP is configured, WebSocket should be nil")
        	assert.Equal(t, "http://localhost:4320/v1/opamp", opampCfg.Server.HTTP.Endpoint)
        	assert.Equal(t, 30*time.Second, opampCfg.Server.HTTP.PollingInterval)
    	} else if opampCfg.Server.WS != nil {
        	assert.Nil(t, opampCfg.Server.HTTP, "When WebSocket is configured, HTTP should be nil")
        	assert.Equal(t, "ws://localhost:4320/v1/opamp", opampCfg.Server.WS.Endpoint)
    	} else {
        	t.Error("Neither HTTP nor WebSocket is configured")
    	}
	})

	t.Run("Full config", func(t *testing.T) {
    translator := NewTranslator()
    conf := confmap.NewFromStringMap(map[string]any{
        "agent": map[string]any{
            "opamp": map[string]any{ 
                "server": map[string]any{
                    "ws": map[string]any{
                        "endpoint": "ws://localhost:4320/v1/opamp",
                    },
                    "http": map[string]any{
                        "endpoint":         "http://localhost:4320/v1/opamp",
                        "polling_interval": "30s",
                    },
                },
				"ppid":         1234,
                "agent_description": map[string]any{
                    "non_identifying_attributes": map[string]any{
                        "description": "A description here...",
                        "foo":        "bar",
                        "agent.name": "Sample Collector",
                    },
                },
                "capabilities": map[string]any{
                    "reports_effective_config":      true,
                    "reports_health":               true,
                    "reports_available_components": true,
                },
            },
        },
    })

    // Rest of the test remains the same...

		cfg, err := translator.Translate(conf)
		require.NoError(t, err)
		
		// Verify the config is the correct type
		opampCfg, ok := cfg.(*opampextension.Config)
		require.True(t, ok, "Expected *opampextension.Config")
		assert.NotNil(t, opampCfg)
		
		// Verify the config contains the expected values
		assert.Equal(t, int32(1234), opampCfg.PPID)
		assert.NotNil(t, opampCfg.Server)

    
    // Check that exactly one of HTTP or WebSocket is configured
    if opampCfg.Server.HTTP != nil {
        assert.Nil(t, opampCfg.Server.WS, "When HTTP is configured, WebSocket should be nil")
        assert.Equal(t, "http://localhost:4320/v1/opamp", opampCfg.Server.HTTP.Endpoint)
        assert.Equal(t, 30*time.Second, opampCfg.Server.HTTP.PollingInterval)
    } else if opampCfg.Server.WS != nil {
        assert.Nil(t, opampCfg.Server.HTTP, "When WebSocket is configured, HTTP should be nil")
        assert.Equal(t, "ws://localhost:4320/v1/opamp", opampCfg.Server.WS.Endpoint)
    } else {
        t.Error("Neither HTTP nor WebSocket is configured")
    }

		// Verify agent description
    	require.NotNil(t, opampCfg.AgentDescription.NonIdentifyingAttributes)
    	assert.Equal(t, "A description here...", opampCfg.AgentDescription.NonIdentifyingAttributes["description"])
    	assert.Equal(t, "bar", opampCfg.AgentDescription.NonIdentifyingAttributes["foo"])
    
    	// Verify capabilities
    	assert.True(t, opampCfg.Capabilities.ReportsEffectiveConfig)
    	assert.True(t, opampCfg.Capabilities.ReportsHealth)
    	assert.True(t, opampCfg.Capabilities.ReportsAvailableComponents)
	})
}