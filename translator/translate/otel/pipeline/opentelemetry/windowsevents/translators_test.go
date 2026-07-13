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

	assert.Equal(t, "system_0", entries[0].name())
	assert.Equal(t, "System", entries[0].channel)
	assert.True(t, entries[0].raw())
	assert.Equal(t, "/custom/system", entries[0].logGroupName)
	assert.Equal(t, []string{"ERROR"}, entries[0].eventLevels)

	assert.Equal(t, "application_1", entries[1].name())
	assert.Equal(t, "Application", entries[1].channel)
	assert.False(t, entries[1].raw())
	assert.Equal(t, []int{1001}, entries[1].eventIDs)

	assert.Equal(t, "microsoft-windows-powershell_operational_2", entries[2].name())
	assert.Equal(t, "Microsoft-Windows-PowerShell/Operational", entries[2].channel)
	assert.Equal(t, []string{"WARNING"}, entries[2].eventLevels)
}

func TestParseEntries_ReceiverNames(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"opentelemetry": map[string]any{
			"collect": map[string]any{
				"windows_events": map[string]any{
					"collect_list": []any{
						map[string]any{"event_name": "System", "event_levels": []any{"ERROR", "WARNING", "INFORMATION"}, "log_group_name": "/aws/cwagent/windows-events/System"},
						map[string]any{"event_name": "Application", "event_levels": []any{"ERROR", "WARNING"}},
					},
				},
			},
		},
	})
	entries := parseEntries(conf)
	require.Len(t, entries, 2)

	// Receiver names are hash-based on (channel, format) and stable
	assert.Equal(t, "system_2384140678", entries[0].receiverName())
	assert.Equal(t, "application_3497854245", entries[1].receiverName())
}

func TestParseEntries_DuplicateChannelsDifferentFilters(t *testing.T) {
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
	assert.Equal(t, "system_0", entries[0].name())
	assert.Equal(t, "system_1", entries[1].name())
	assert.Equal(t, "system_2", entries[2].name())

	// Different filters produce different receivers (different query XMLs)
	assert.NotEqual(t, entries[0].receiverName(), entries[1].receiverName())
	assert.NotEqual(t, entries[1].receiverName(), entries[2].receiverName())
}

func TestParseEntries_DuplicateChannelsSameFiltersShareReceiver(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"opentelemetry": map[string]any{
			"collect": map[string]any{
				"windows_events": map[string]any{
					"collect_list": []any{
						map[string]any{"event_name": "System", "event_levels": []any{"ERROR"}, "log_group_name": "/group-a"},
						map[string]any{"event_name": "System", "event_levels": []any{"ERROR"}, "log_group_name": "/group-b"},
					},
				},
			},
		},
	})
	entries := parseEntries(conf)
	require.Len(t, entries, 2)

	// Same channel + same format + same filters = shared receiver
	assert.Equal(t, entries[0].receiverName(), entries[1].receiverName())
}

func TestParseEntries_ReceiverNameStableAcrossReorder(t *testing.T) {
	confAB := confmap.NewFromStringMap(map[string]any{
		"opentelemetry": map[string]any{
			"collect": map[string]any{
				"windows_events": map[string]any{
					"collect_list": []any{
						map[string]any{"event_name": "System", "event_levels": []any{"ERROR"}},
						map[string]any{"event_name": "Application"},
					},
				},
			},
		},
	})
	confBA := confmap.NewFromStringMap(map[string]any{
		"opentelemetry": map[string]any{
			"collect": map[string]any{
				"windows_events": map[string]any{
					"collect_list": []any{
						map[string]any{"event_name": "Application"},
						map[string]any{"event_name": "System", "event_levels": []any{"ERROR"}},
					},
				},
			},
		},
	})

	entriesAB := parseEntries(confAB)
	entriesBA := parseEntries(confBA)
	require.Len(t, entriesAB, 2)
	require.Len(t, entriesBA, 2)

	// Receiver name is order-independent (hash-based)
	assert.Equal(t, entriesAB[0].receiverName(), entriesBA[1].receiverName())
	assert.Equal(t, entriesAB[1].receiverName(), entriesBA[0].receiverName())
}

func TestParseEntries_DuplicateChannelsDifferentFormat(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"opentelemetry": map[string]any{
			"collect": map[string]any{
				"windows_events": map[string]any{
					"collect_list": []any{
						map[string]any{"event_name": "System", "event_format": "xml"},
						map[string]any{"event_name": "System", "event_levels": []any{"ERROR"}},
					},
				},
			},
		},
	})
	entries := parseEntries(conf)
	require.Len(t, entries, 2)

	// Different formats get separate receivers even for the same channel
	assert.NotEqual(t, entries[0].receiverName(), entries[1].receiverName())
	assert.True(t, entries[0].raw())
	assert.False(t, entries[1].raw())
}

func TestParseEntries_DuplicateChannelsSameFormatShareReceiver(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"opentelemetry": map[string]any{
			"collect": map[string]any{
				"windows_events": map[string]any{
					"collect_list": []any{
						map[string]any{"event_name": "System", "event_format": "xml"},
						map[string]any{"event_name": "System", "event_format": "xml"},
					},
				},
			},
		},
	})
	entries := parseEntries(conf)
	require.Len(t, entries, 2)

	// Same format + same channel still share a receiver
	assert.Equal(t, entries[0].receiverName(), entries[1].receiverName())
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
	assert.NotEqual(t, entries[0].receiverName(), entries[1].receiverName())
}

func TestParseEntries_InvalidItems(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"opentelemetry": map[string]any{
			"collect": map[string]any{
				"windows_events": map[string]any{
					"collect_list": []any{
						"not a map",
						map[string]any{"event_name": ""},
						map[string]any{"event_name": "System"},
					},
				},
			},
		},
	})
	entries := parseEntries(conf)
	require.Len(t, entries, 1)
	assert.Equal(t, "System", entries[0].channel)
	assert.Equal(t, 2, entries[0].index)
}
