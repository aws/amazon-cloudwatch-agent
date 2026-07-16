// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestNewTranslators_NilConf(t *testing.T) {
	translators := NewTranslators(nil)
	assert.Equal(t, 0, translators.Len())
}

func TestNewTranslators_NoFilesKey(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"opentelemetry": map[string]any{
			"collect": map[string]any{},
		},
	})
	translators := NewTranslators(conf)
	assert.Equal(t, 0, translators.Len())
}

func TestNewTranslators_SingleEntry(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"opentelemetry": map[string]any{
			"collect": map[string]any{
				"files": map[string]any{
					"collect_list": []any{
						map[string]any{
							"file_path":      "/var/log/app.log",
							"log_group_name": "/aws/app/logs",
						},
					},
				},
			},
		},
	})
	translators := NewTranslators(conf)
	assert.Equal(t, 1, translators.Len())
}

func TestNewTranslators_MultipleEntries(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"opentelemetry": map[string]any{
			"collect": map[string]any{
				"files": map[string]any{
					"collect_list": []any{
						map[string]any{"file_path": "/var/log/app.log"},
						map[string]any{"file_path": "/var/log/syslog"},
						map[string]any{"file_path": "/var/log/auth.log"},
					},
				},
			},
		},
	})
	translators := NewTranslators(conf)
	assert.Equal(t, 3, translators.Len())
}

func TestNewTranslators_SkipsEmptyFilePath(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"opentelemetry": map[string]any{
			"collect": map[string]any{
				"files": map[string]any{
					"collect_list": []any{
						map[string]any{"file_path": ""},
						map[string]any{"file_path": "/var/log/app.log"},
					},
				},
			},
		},
	})
	translators := NewTranslators(conf)
	assert.Equal(t, 1, translators.Len())
}

func TestParseEntries_AllFields(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"opentelemetry": map[string]any{
			"collect": map[string]any{
				"files": map[string]any{
					"collect_list": []any{
						map[string]any{
							"file_path":                "/var/log/app.log",
							"log_group_name":           "/aws/app",
							"log_stream_name":          "{hostname}",
							"multi_line_start_pattern": `^\d{4}`,
							"timestamp_format":         "%Y-%m-%d %H:%M:%S",
							"timezone":                 "UTC",
							"encoding":                 "utf-16",
						},
					},
				},
			},
		},
	})
	entries := parseEntries(conf)
	require.Len(t, entries, 1)

	e := entries[0]
	assert.Equal(t, "/var/log/app.log", e.filePath)
	assert.Equal(t, "/aws/app", e.logGroupName)
	assert.Equal(t, "{hostname}", e.logStreamName)
	assert.Equal(t, `^\d{4}`, e.multilinePattern)
	assert.Equal(t, "%Y-%m-%d %H:%M:%S", e.timestampFormat)
	assert.Equal(t, "UTC", e.timezone)
	assert.Equal(t, "utf-16", e.encoding)
	assert.Equal(t, common.FilesKey, e.resource["aws.log.source"])
}

func TestParseEntries_DefaultEncoding(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"opentelemetry": map[string]any{
			"collect": map[string]any{
				"files": map[string]any{
					"collect_list": []any{
						map[string]any{"file_path": "/var/log/app.log"},
					},
				},
			},
		},
	})
	entries := parseEntries(conf)
	require.Len(t, entries, 1)
	assert.Equal(t, "utf-8", entries[0].encoding)
}

func TestParseEntries_TimestampFormatMagicValue(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"opentelemetry": map[string]any{
			"collect": map[string]any{
				"files": map[string]any{
					"collect_list": []any{
						map[string]any{
							"file_path":                "/var/log/app.log",
							"multi_line_start_pattern": "{timestamp_format}",
							"timestamp_format":         "%Y-%m-%d %H:%M:%S",
						},
					},
				},
			},
		},
	})
	entries := parseEntries(conf)
	require.Len(t, entries, 1)

	assert.NotEqual(t, "{timestamp_format}", entries[0].multilinePattern)
	assert.Contains(t, entries[0].multilinePattern, `\d{4}`)
}

func TestReceiverDedup_SameConfig(t *testing.T) {
	e1 := fileEntry{filePath: "/var/log/app.log", encoding: "utf-8", timestampFormat: "%Y-%m-%d"}
	e2 := fileEntry{filePath: "/var/log/app.log", encoding: "utf-8", timestampFormat: "%Y-%m-%d"}
	assert.Equal(t, e1.receiverHash(), e2.receiverHash())
	assert.Equal(t, e1.receiverName(), e2.receiverName())
}

func TestReceiverDedup_DifferentConfig(t *testing.T) {
	e1 := fileEntry{filePath: "/var/log/app.log", encoding: "utf-8", timestampFormat: "%Y-%m-%d"}
	e2 := fileEntry{filePath: "/var/log/app.log", encoding: "utf-16", timestampFormat: "%Y-%m-%d"}
	assert.NotEqual(t, e1.receiverHash(), e2.receiverHash())
}

func TestRoutingAttributes(t *testing.T) {
	tests := []struct {
		name      string
		entry     fileEntry
		expectNil bool
		expectLen int
	}{
		{
			name:      "no routing",
			entry:     fileEntry{},
			expectNil: true,
		},
		{
			name:      "log group only",
			entry:     fileEntry{logGroupName: "/aws/app"},
			expectLen: 1,
		},
		{
			name:      "log stream only",
			entry:     fileEntry{logStreamName: "{hostname}"},
			expectLen: 1,
		},
		{
			name:      "both",
			entry:     fileEntry{logGroupName: "/aws/app", logStreamName: "{hostname}"},
			expectLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attrs := tt.entry.routingAttributes()
			if tt.expectNil {
				assert.Nil(t, attrs)
			} else {
				assert.Len(t, attrs, tt.expectLen)
			}
		})
	}
}
