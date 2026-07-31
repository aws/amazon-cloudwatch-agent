// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	globallogs "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs"
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
	globallogs.GlobalLogConfig.MetadataInfo = map[string]string{
		"{hostname}":    "test-host",
		"{instance_id}": "i-1234567890abcdef0",
	}
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
	assert.Equal(t, "test-host", e.logStreamName)
	assert.Equal(t, `^\d{4}`, e.multilinePattern)
	assert.Equal(t, "%Y-%m-%d %H:%M:%S", e.timestampFormat)
	assert.Equal(t, "UTC", e.timezone)
	assert.Equal(t, "utf-16", e.encoding)
	assert.Equal(t, common.FilesKey, e.resource["aws.log.source"])
}

func TestParseEntries_ResolvesPlaceholders(t *testing.T) {
	globallogs.GlobalLogConfig.MetadataInfo = map[string]string{
		"{hostname}":    "ip-172-31-0-1",
		"{instance_id}": "i-abcdef1234567890",
		"{ip_address}":  "172.31.0.1",
	}
	conf := confmap.NewFromStringMap(map[string]any{
		"opentelemetry": map[string]any{
			"collect": map[string]any{
				"files": map[string]any{
					"collect_list": []any{
						map[string]any{
							"file_path":       "/var/log/app.log",
							"log_group_name":  "logs-{instance_id}",
							"log_stream_name": "{hostname}/{ip_address}",
						},
					},
				},
			},
		},
	})
	entries := parseEntries(conf)
	require.Len(t, entries, 1)
	assert.Equal(t, "logs-i-abcdef1234567890", entries[0].logGroupName)
	assert.Equal(t, "ip-172-31-0-1/172.31.0.1", entries[0].logStreamName)
}

func TestParseEntries_EmptyNamesSkipPlaceholderResolution(t *testing.T) {
	globallogs.GlobalLogConfig.MetadataInfo = map[string]string{
		"{instance_id}": "i-abcdef1234567890",
	}
	conf := confmap.NewFromStringMap(map[string]any{
		"opentelemetry": map[string]any{
			"collect": map[string]any{
				"files": map[string]any{
					"collect_list": []any{
						map[string]any{
							"file_path": "/var/log/app.log",
						},
					},
				},
			},
		},
	})
	entries := parseEntries(conf)
	require.Len(t, entries, 1)
	assert.Equal(t, "", entries[0].logGroupName)
	assert.Equal(t, "", entries[0].logStreamName)
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
	assert.Equal(t, "{timestamp_format}", entries[0].multilinePattern)
}

func TestResolveMultilinePattern(t *testing.T) {
	e := fileEntry{
		multilinePattern: "{timestamp_format}",
		timestampFormat:  "%Y-%m-%d %H:%M:%S",
	}
	pattern, err := e.resolveMultilinePattern()
	require.NoError(t, err)
	assert.Contains(t, pattern, `\d{4}`)
}

func TestResolveMultilinePattern_MissingTimestampFormat(t *testing.T) {
	e := fileEntry{
		filePath:         "/var/log/app.log",
		multilinePattern: "{timestamp_format}",
		timestampFormat:  "",
	}
	_, err := e.resolveMultilinePattern()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timestamp_format is not set")
}

func TestResolveMultilinePattern_LiteralPattern(t *testing.T) {
	e := fileEntry{
		multilinePattern: `^\d{4}-\d{2}-\d{2}`,
	}
	pattern, err := e.resolveMultilinePattern()
	require.NoError(t, err)
	assert.Equal(t, `^\d{4}-\d{2}-\d{2}`, pattern)
}

func TestResolveMultilinePattern_InvalidRegex(t *testing.T) {
	e := fileEntry{
		filePath:         "/var/log/app.log",
		multilinePattern: "[invalid",
	}
	_, err := e.resolveMultilinePattern()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid multi_line_start_pattern")
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
