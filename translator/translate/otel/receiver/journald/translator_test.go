// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package journald

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/journaldreceiver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator_ID(t *testing.T) {
	translator := NewTranslator()
	expected := component.NewID(component.MustNewType("journald"))
	assert.Equal(t, expected, translator.ID())

	translatorWithName := NewTranslatorWithName("test")
	expectedWithName := component.NewIDWithName(component.MustNewType("journald"), "test")
	assert.Equal(t, expectedWithName, translatorWithName.ID())
}

func TestTranslator_Translate_MissingKey(t *testing.T) {
	translator := NewTranslator()
	
	// Test with nil config
	_, err := translator.Translate(nil)
	require.Error(t, err)
	assert.IsType(t, &common.MissingKeyError{}, err)

	// Test with empty config
	conf := confmap.New()
	_, err = translator.Translate(conf)
	require.Error(t, err)
	assert.IsType(t, &common.MissingKeyError{}, err)
}

func TestTranslator_Translate_InvalidConfig(t *testing.T) {
	translator := NewTranslator()
	
	// Test with invalid config type (string instead of object)
	conf := confmap.NewFromStringMap(map[string]interface{}{
		"logs": map[string]interface{}{
			"logs_collected": map[string]interface{}{
				"journald": "invalid",
			},
		},
	})
	
	_, err := translator.Translate(conf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "journald configuration must be an object")
}

func TestTranslator_Translate_MinimalConfig(t *testing.T) {
	translator := NewTranslator()
	
	conf := confmap.NewFromStringMap(map[string]interface{}{
		"logs": map[string]interface{}{
			"logs_collected": map[string]interface{}{
				"journald": map[string]interface{}{},
			},
		},
	})
	
	result, err := translator.Translate(conf)
	require.NoError(t, err)
	require.NotNil(t, result)
	
	cfg, ok := result.(*journaldreceiver.JournaldConfig)
	require.True(t, ok)
	
	// Verify default values
	assert.Equal(t, "end", cfg.InputConfig.StartAt)
	assert.Empty(t, cfg.InputConfig.Units)
	assert.Equal(t, "info", cfg.InputConfig.Priority) // journald receiver has "info" as default priority
	assert.Nil(t, cfg.InputConfig.Directory)
}

func TestTranslator_Translate_FullConfig(t *testing.T) {
	translator := NewTranslator()
	
	conf := confmap.NewFromStringMap(map[string]interface{}{
		"logs": map[string]interface{}{
			"logs_collected": map[string]interface{}{
				"journald": map[string]interface{}{
					"units":       []interface{}{"nginx.service", "apache2.service"},
					"priority":    "info",
					"directory":   "/var/log/journal",
					"files":       []interface{}{"/var/log/journal/system.journal"},
					"identifiers": []interface{}{"nginx", "apache"},
					"grep":        "ERROR|WARN",
					"dmesg":       true,
					"all":         false,
					"namespace":   "system",
					"since":       "2023-01-01",
				},
			},
		},
	})
	
	result, err := translator.Translate(conf)
	require.NoError(t, err)
	require.NotNil(t, result)
	
	cfg, ok := result.(*journaldreceiver.JournaldConfig)
	require.True(t, ok)
	
	// Verify all fields are mapped correctly
	assert.Equal(t, []string{"nginx.service", "apache2.service"}, cfg.InputConfig.Units)
	assert.Equal(t, "info", cfg.InputConfig.Priority)
	assert.Equal(t, "/var/log/journal", *cfg.InputConfig.Directory)
	assert.Equal(t, []string{"/var/log/journal/system.journal"}, cfg.InputConfig.Files)
	assert.Equal(t, []string{"nginx", "apache"}, cfg.InputConfig.Identifiers)
	assert.Equal(t, "ERROR|WARN", cfg.InputConfig.Grep)
	assert.True(t, cfg.InputConfig.Dmesg)
	assert.False(t, cfg.InputConfig.All)
	assert.Equal(t, "system", cfg.InputConfig.Namespace)
	assert.Equal(t, "beginning", cfg.InputConfig.StartAt) // since is set
}

func TestTranslator_Translate_SinceHandling(t *testing.T) {
	translator := NewTranslator()
	
	testCases := []struct {
		name     string
		since    interface{}
		expected string
	}{
		{
			name:     "no since field",
			since:    nil,
			expected: "end",
		},
		{
			name:     "empty since",
			since:    "",
			expected: "end",
		},
		{
			name:     "since with value",
			since:    "2023-01-01",
			expected: "beginning",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configMap := map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"journald": map[string]interface{}{},
					},
				},
			}
			
			if tc.since != nil {
				journaldConfig := configMap["logs"].(map[string]interface{})["logs_collected"].(map[string]interface{})["journald"].(map[string]interface{})
				journaldConfig["since"] = tc.since
			}
			
			conf := confmap.NewFromStringMap(configMap)
			result, err := translator.Translate(conf)
			require.NoError(t, err)
			
			cfg, ok := result.(*journaldreceiver.JournaldConfig)
			require.True(t, ok)
			assert.Equal(t, tc.expected, cfg.InputConfig.StartAt)
		})
	}
}

func TestTranslator_Translate_ArrayFields(t *testing.T) {
	translator := NewTranslator()
	
	testCases := []struct {
		name     string
		field    string
		input    []interface{}
		expected []string
	}{
		{
			name:     "units array",
			field:    "units",
			input:    []interface{}{"service1.service", "service2.service"},
			expected: []string{"service1.service", "service2.service"},
		},
		{
			name:     "files array",
			field:    "files",
			input:    []interface{}{"/path/to/file1.journal", "/path/to/file2.journal"},
			expected: []string{"/path/to/file1.journal", "/path/to/file2.journal"},
		},
		{
			name:     "identifiers array",
			field:    "identifiers",
			input:    []interface{}{"id1", "id2", "id3"},
			expected: []string{"id1", "id2", "id3"},
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"journald": map[string]interface{}{
							tc.field: tc.input,
						},
					},
				},
			})
			
			result, err := translator.Translate(conf)
			require.NoError(t, err)
			
			cfg, ok := result.(*journaldreceiver.JournaldConfig)
			require.True(t, ok)
			
			switch tc.field {
			case "units":
				assert.Equal(t, tc.expected, cfg.InputConfig.Units)
			case "files":
				assert.Equal(t, tc.expected, cfg.InputConfig.Files)
			case "identifiers":
				assert.Equal(t, tc.expected, cfg.InputConfig.Identifiers)
			}
		})
	}
}