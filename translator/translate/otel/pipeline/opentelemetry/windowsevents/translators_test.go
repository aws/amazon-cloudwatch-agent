// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package windowsevents

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

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
	assert.Equal(t, "/custom/system", entries[0].logGroupName)
	assert.Equal(t, []string{"ERROR"}, entries[0].eventLevels)

	assert.Equal(t, "application", entries[1].name)
	assert.Equal(t, "Application", entries[1].channel)
	assert.False(t, entries[1].raw)
	assert.Equal(t, []int{1001}, entries[1].eventIDs)

	assert.Equal(t, "microsoft-windows-powershell_operational", entries[2].name)
	assert.Equal(t, "Microsoft-Windows-PowerShell/Operational", entries[2].channel)
	assert.Equal(t, []string{"WARNING"}, entries[2].eventLevels)
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

func TestParseEntries_DifferentChannelsSanitizeCollision(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"opentelemetry": map[string]any{
			"collect": map[string]any{
				"windows_events": map[string]any{
					"collect_list": []any{
						map[string]any{"event_name": "A/B", "event_levels": []any{"ERROR"}},
						map[string]any{"event_name": "A_B", "event_levels": []any{"WARNING"}},
					},
				},
			},
		},
	})
	entries := parseEntries(conf)
	require.Len(t, entries, 2)

	// Different channels get different receiver names even if they collide after sanitization
	assert.NotEqual(t, entries[0].receiverName, entries[1].receiverName)
	assert.Equal(t, "a_b", entries[0].receiverName)
	assert.Equal(t, "a_b_1", entries[1].receiverName)
}
