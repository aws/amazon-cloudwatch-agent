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
		name:         "system",
		receiverName: "system",
		channel:      "System",
		raw:          false,
		resource:     map[string]string{"aws.log.source": "windows_events"},
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
		name:         "system",
		receiverName: "system",
		channel:      "System",
		raw:          false,
		resource:     map[string]string{"aws.log.source": "windows_events"},
		eventLevels:  []string{"ERROR", "WARNING"},
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
						map[string]any{"event_name": "Microsoft-Windows-PowerShell/Operational", "event_levels": []any{"WARNING"}},
					},
				},
			},
		},
	})
	entries := parseEntries(conf)
	require.Len(t, entries, 3)

	assert.Equal(t, "system", entries[0].name)
	assert.Equal(t, "System", entries[0].channel)
	assert.True(t, entries[0].raw)
	assert.Equal(t, "/custom/system", entries[0].resource["aws.log.group.name"])
	assert.Equal(t, []string{"ERROR"}, entries[0].eventLevels)

	assert.Equal(t, "application", entries[1].name)
	assert.Equal(t, "Application", entries[1].channel)
	assert.False(t, entries[1].raw)
	assert.Equal(t, []int{1001}, entries[1].eventIDs)

	assert.Equal(t, "microsoft-windows-powershell_operational", entries[2].name)
	assert.Equal(t, "Microsoft-Windows-PowerShell/Operational", entries[2].channel)
	assert.Equal(t, []string{"WARNING"}, entries[2].eventLevels)
}

func TestPipelineTranslator_DuplicateChannels_SharedReceiver(t *testing.T) {
	pt1 := &windowsEventsPipelineTranslator{entry: eventEntry{
		name:         "system",
		receiverName: "system",
		channel:      "System",
		resource:     map[string]string{"aws.log.source": "windows_events", "aws.log.channel": "System"},
		eventLevels:  []string{"ERROR"},
	}}
	pt2 := &windowsEventsPipelineTranslator{entry: eventEntry{
		name:         "system_1",
		receiverName: "system",
		channel:      "System",
		resource:     map[string]string{"aws.log.source": "windows_events", "aws.log.channel": "System"},
		eventLevels:  []string{"WARNING"},
	}}

	r1, err := pt1.Translate(nil)
	require.NoError(t, err)
	r2, err := pt2.Translate(nil)
	require.NoError(t, err)

	// Different pipeline IDs
	assert.NotEqual(t, pt1.ID(), pt2.ID())

	// Same receiver ID (shared checkpoint)
	assert.Equal(t, r1.Receivers.Keys(), r2.Receivers.Keys())
}

func TestParseEntries_DuplicateChannels(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"opentelemetry": map[string]any{
			"collect": map[string]any{
				"windows_events": map[string]any{
					"collect_list": []any{
						map[string]any{"event_name": "System", "event_levels": []any{"ERROR"}},
						map[string]any{"event_name": "System", "event_levels": []any{"WARNING"}},
						map[string]any{"event_name": "System", "event_ids": []any{float64(1001)}},
					},
				},
			},
		},
	})
	entries := parseEntries(conf)
	require.Len(t, entries, 3)
	assert.Equal(t, "system", entries[0].name)
	assert.Equal(t, "system_1", entries[1].name)
	assert.Equal(t, "system_2", entries[2].name)

	// All entries share the same receiver (same channel = same checkpoint)
	assert.Equal(t, "system", entries[0].receiverName)
	assert.Equal(t, "system", entries[1].receiverName)
	assert.Equal(t, "system", entries[2].receiverName)
}

func TestPipelineTranslator_Translate_XmlWithEventIDs_Error(t *testing.T) {
	pt := &windowsEventsPipelineTranslator{entry: eventEntry{
		name:     "security",
		channel:  "Security",
		raw:      true,
		eventIDs: []int{4624},
	}}
	_, err := pt.Translate(nil)
	assert.EqualError(t, err, `event_ids filtering is not supported with event_format "xml" for channel "Security"`)
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
			name:     "verbose only",
			entry:    eventEntry{name: "system_0", eventLevels: []string{"VERBOSE"}},
			expected: `not((severity_number == 0))`,
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
