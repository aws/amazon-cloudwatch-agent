// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package journald

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/journaldreceiver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"gopkg.in/yaml.v3"
)

func TestJSONToYAMLConversion(t *testing.T) {
	// Read the example JSON config
	jsonData, err := os.ReadFile("example_config.json")
	require.NoError(t, err)

	// Parse JSON into a map
	var jsonConfig map[string]interface{}
	err = json.Unmarshal(jsonData, &jsonConfig)
	require.NoError(t, err)

	// Create confmap from JSON
	conf := confmap.NewFromStringMap(jsonConfig)

	// Translate using our translator
	translator := NewTranslator()
	result, err := translator.Translate(conf)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify the translation worked correctly
	cfg, ok := result.(*journaldreceiver.JournaldConfig)
	require.True(t, ok)

	// Verify all fields are correctly mapped
	assert.Equal(t, []string{"nginx.service", "apache2.service", "docker.service"}, cfg.InputConfig.Units)
	assert.Equal(t, "info", cfg.InputConfig.Priority)
	assert.Equal(t, "/var/log/journal", *cfg.InputConfig.Directory)
	assert.Equal(t, []string{"/var/log/journal/system.journal"}, cfg.InputConfig.Files)
	assert.Equal(t, []string{"nginx", "apache", "docker"}, cfg.InputConfig.Identifiers)
	assert.Equal(t, "ERROR|WARN|CRITICAL", cfg.InputConfig.Grep)
	assert.True(t, cfg.InputConfig.Dmesg)
	assert.False(t, cfg.InputConfig.All)
	assert.Equal(t, "system", cfg.InputConfig.Namespace)
	assert.Equal(t, "beginning", cfg.InputConfig.StartAt) // since is set

	// Convert the result to YAML to verify it's serializable
	yamlData, err := yaml.Marshal(cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, yamlData)

	// Verify YAML contains expected fields
	yamlString := string(yamlData)
	assert.Contains(t, yamlString, "units:")
	assert.Contains(t, yamlString, "nginx.service")
	assert.Contains(t, yamlString, "priority: info")
	assert.Contains(t, yamlString, "directory: /var/log/journal")
	assert.Contains(t, yamlString, "grep: ERROR|WARN|CRITICAL")
	assert.Contains(t, yamlString, "dmesg: true")
	assert.Contains(t, yamlString, "all: false")
	assert.Contains(t, yamlString, "namespace: system")
	assert.Contains(t, yamlString, "startat: beginning") // journald uses camelCase in YAML

	t.Logf("Generated YAML:\n%s", yamlString)
}

func TestMinimalJSONToYAMLConversion(t *testing.T) {
	// Test with minimal configuration
	minimalConfig := map[string]interface{}{
		"logs": map[string]interface{}{
			"logs_collected": map[string]interface{}{
				"journald": map[string]interface{}{
					"units": []interface{}{"systemd-journald.service"},
				},
			},
		},
	}

	conf := confmap.NewFromStringMap(minimalConfig)
	translator := NewTranslator()
	result, err := translator.Translate(conf)
	require.NoError(t, err)

	cfg, ok := result.(*journaldreceiver.JournaldConfig)
	require.True(t, ok)

	// Convert to YAML
	yamlData, err := yaml.Marshal(cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, yamlData)

	yamlString := string(yamlData)
	assert.Contains(t, yamlString, "units:")
	assert.Contains(t, yamlString, "systemd-journald.service")
	assert.Contains(t, yamlString, "startat: end") // default value, camelCase in YAML

	t.Logf("Minimal YAML:\n%s", yamlString)
}

func TestComplexJSONToYAMLConversion(t *testing.T) {
	// Test with complex configuration including edge cases
	complexConfig := map[string]interface{}{
		"logs": map[string]interface{}{
			"logs_collected": map[string]interface{}{
				"journald": map[string]interface{}{
					"units": []interface{}{
						"multi-word-service.service",
						"service-with-numbers123.service",
						"service.with.dots.service",
					},
					"priority":    "debug",
					"identifiers": []interface{}{"app1", "app2", "app3"},
					"grep":        "^\\[ERROR\\].*|FATAL.*$",
					"dmesg":       false,
					"all":         true,
					"namespace":   "user-1000",
				},
			},
		},
	}

	conf := confmap.NewFromStringMap(complexConfig)
	translator := NewTranslator()
	result, err := translator.Translate(conf)
	require.NoError(t, err)

	cfg, ok := result.(*journaldreceiver.JournaldConfig)
	require.True(t, ok)

	// Verify complex values
	assert.Equal(t, []string{
		"multi-word-service.service",
		"service-with-numbers123.service",
		"service.with.dots.service",
	}, cfg.InputConfig.Units)
	assert.Equal(t, "debug", cfg.InputConfig.Priority)
	assert.Equal(t, "^\\[ERROR\\].*|FATAL.*$", cfg.InputConfig.Grep)
	assert.False(t, cfg.InputConfig.Dmesg)
	assert.True(t, cfg.InputConfig.All)
	assert.Equal(t, "user-1000", cfg.InputConfig.Namespace)

	// Convert to YAML
	yamlData, err := yaml.Marshal(cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, yamlData)

	yamlString := string(yamlData)
	assert.Contains(t, yamlString, "multi-word-service.service")
	assert.Contains(t, yamlString, "priority: debug")
	assert.Contains(t, yamlString, "grep: ^\\[ERROR\\].*|FATAL.*$")
	assert.Contains(t, yamlString, "dmesg: false")
	assert.Contains(t, yamlString, "all: true")
	assert.Contains(t, yamlString, "namespace: user-1000")

	t.Logf("Complex YAML:\n%s", yamlString)
}