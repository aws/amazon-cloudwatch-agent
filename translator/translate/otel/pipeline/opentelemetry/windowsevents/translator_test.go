// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package windowsevents

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	translatorconfig "github.com/aws/amazon-cloudwatch-agent/translator/config"
	translatorcontext "github.com/aws/amazon-cloudwatch-agent/translator/context"
)

func TestNewTranslators_Disabled(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"opentelemetry": map[string]any{"collect": map[string]any{}},
	})
	translators := NewTranslators(conf)
	assert.Equal(t, 0, translators.Len())
}

func TestNewTranslators_TwoEntries(t *testing.T) {
	translatorcontext.CurrentContext().SetOs(translatorconfig.OS_TYPE_WINDOWS)
	defer translatorcontext.CurrentContext().SetOs("")
	conf := confmap.NewFromStringMap(map[string]any{
		"opentelemetry": map[string]any{
			"collect": map[string]any{
				"windows_events": map[string]any{
					"collect_list": []any{
						map[string]any{"event_name": "System"},
						map[string]any{"event_name": "Application"},
					},
				},
			},
		},
	})
	translators := NewTranslators(conf)
	assert.Equal(t, 2, translators.Len())
}

func TestNewTranslators_NonWindows(t *testing.T) {
	translatorcontext.CurrentContext().SetOs(translatorconfig.OS_TYPE_LINUX)
	defer translatorcontext.CurrentContext().SetOs("")
	conf := confmap.NewFromStringMap(map[string]any{
		"opentelemetry": map[string]any{
			"collect": map[string]any{
				"windows_events": map[string]any{
					"collect_list": []any{
						map[string]any{"event_name": "System"},
					},
				},
			},
		},
	})
	translators := NewTranslators(conf)
	assert.Equal(t, 0, translators.Len())
}

func TestPipelineTranslator_ID(t *testing.T) {
	pt := &windowsEventsPipelineTranslator{entry: eventEntry{name: "system_0"}}
	assert.Equal(t, pipeline.NewIDWithName(pipeline.SignalLogs, "windows_events_system_0"), pt.ID())
}

func TestPipelineTranslator_Translate_NoFilter(t *testing.T) {
	pt := &windowsEventsPipelineTranslator{entry: eventEntry{
		name:     "system_0",
		channel:  "System",
		raw:      false,
		resource: map[string]string{"aws.log.source": "windows_events"},
	}}
	result, err := pt.Translate(nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 1, result.Receivers.Len())
	assert.Equal(t, 1, result.Processors.Len())
	assert.Equal(t, 1, result.Exporters.Len())
	assert.Equal(t, 1, result.Extensions.Len())
	assert.Equal(t, 1, result.Connectors.Len())
}

func TestPipelineTranslator_Translate_WithFilter(t *testing.T) {
	pt := &windowsEventsPipelineTranslator{entry: eventEntry{
		name:        "system_0",
		channel:     "System",
		raw:         false,
		resource:    map[string]string{"aws.log.source": "windows_events"},
		eventLevels: []string{"ERROR", "WARNING"},
	}}
	result, err := pt.Translate(nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 1, result.Extensions.Len())
	assert.Equal(t, 2, result.Processors.Len())
}

func TestParseEntries(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"opentelemetry": map[string]any{
			"collect": map[string]any{
				"windows_events": map[string]any{
					"collect_list": []any{
						map[string]any{"event_name": "System", "event_levels": []any{"ERROR"}, "event_format": "xml", "log_group_name": "/custom/system"},
						map[string]any{"event_name": "Application", "event_ids": []any{float64(1001)}},
					},
				},
			},
		},
	})
	entries := parseEntries(conf)
	require.Len(t, entries, 2)

	assert.Equal(t, "system_0", entries[0].name)
	assert.Equal(t, "System", entries[0].channel)
	assert.True(t, entries[0].raw)
	assert.Equal(t, "/custom/system", entries[0].resource["aws.log.group.name"])
	assert.Equal(t, []string{"ERROR"}, entries[0].eventLevels)

	assert.Equal(t, "application_1", entries[1].name)
	assert.Equal(t, "Application", entries[1].channel)
	assert.False(t, entries[1].raw)
	assert.Equal(t, []int{1001}, entries[1].eventIDs)
}

func TestBuildFilterCondition(t *testing.T) {
	tests := []struct {
		name     string
		entry    eventEntry
		expected string
	}{
		{
			name:     "no filter",
			entry:    eventEntry{name: "system_0", channel: "System"},
			expected: "",
		},
		{
			name:     "levels only",
			entry:    eventEntry{name: "system_0", eventLevels: []string{"ERROR", "WARNING"}},
			expected: `not((severity_number == 17 or severity_number == 13))`,
		},
		{
			name:     "ids only",
			entry:    eventEntry{name: "security_0", eventIDs: []int{4624, 4625}},
			expected: `not((body["event_id"]["id"] == 4624 or body["event_id"]["id"] == 4625))`,
		},
		{
			name:     "levels and ids",
			entry:    eventEntry{name: "system_0", eventLevels: []string{"ERROR"}, eventIDs: []int{1001}},
			expected: `not((severity_number == 17) and (body["event_id"]["id"] == 1001))`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.entry.filterCondition())
		})
	}
}
