// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package configprovider

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"
)

func TestExtractRegionFromLogsEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		expected string
	}{
		{
			name:     "valid us-east-1",
			endpoint: "https://logs.us-east-1.amazonaws.com/v1/logs",
			expected: "us-east-1",
		},
		{
			name:     "valid eu-west-1",
			endpoint: "https://logs.eu-west-1.amazonaws.com/v1/logs",
			expected: "eu-west-1",
		},
		{
			name:     "not a logs endpoint",
			endpoint: "https://xray.us-east-1.amazonaws.com/v1/traces",
			expected: "",
		},
		{
			name:     "empty",
			endpoint: "",
			expected: "",
		},
		{
			name:     "non-AWS endpoint",
			endpoint: "http://localhost:4318",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractRegionFromLogsEndpoint(tt.endpoint)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFindLogTargets(t *testing.T) {
	p := &otlphttpLogProvisioner{}

	tests := []struct {
		name     string
		config   map[string]any
		expected []logTarget
	}{
		{
			name: "single otlphttp exporter in logs pipeline",
			config: map[string]any{
				"exporters": map[string]any{
					"otlphttp/cw-logs": map[string]any{
						"logs_endpoint": "https://logs.us-east-1.amazonaws.com/v1/logs",
						"headers": map[string]any{
							"x-aws-log-group":  "/test/telemetry",
							"x-aws-log-stream": "default",
						},
					},
				},
				"service": map[string]any{
					"pipelines": map[string]any{
						"logs/test": map[string]any{
							"exporters": []any{"otlphttp/cw-logs"},
						},
					},
				},
			},
			expected: []logTarget{
				{logGroupName: "/test/telemetry", logStreamName: "default", region: "us-east-1"},
			},
		},
		{
			name: "otlphttp exporter not in logs pipeline",
			config: map[string]any{
				"exporters": map[string]any{
					"otlphttp/metrics": map[string]any{
						"endpoint": "https://logs.us-east-1.amazonaws.com/v1/logs",
						"headers": map[string]any{
							"x-aws-log-group": "/some/group",
						},
					},
				},
				"service": map[string]any{
					"pipelines": map[string]any{
						"metrics/something": map[string]any{
							"exporters": []any{"otlphttp/metrics"},
						},
					},
				},
			},
			expected: nil,
		},
		{
			name: "non-CW endpoint ignored",
			config: map[string]any{
				"exporters": map[string]any{
					"otlphttp": map[string]any{
						"endpoint": "http://localhost:4318",
						"headers": map[string]any{
							"x-aws-log-group": "/some/group",
						},
					},
				},
				"service": map[string]any{
					"pipelines": map[string]any{
						"logs/local": map[string]any{
							"exporters": []any{"otlphttp"},
						},
					},
				},
			},
			expected: nil,
		},
		{
			name: "missing log group header",
			config: map[string]any{
				"exporters": map[string]any{
					"otlphttp/cw": map[string]any{
						"logs_endpoint": "https://logs.us-west-2.amazonaws.com/v1/logs",
						"headers":       map[string]any{},
					},
				},
				"service": map[string]any{
					"pipelines": map[string]any{
						"logs/test": map[string]any{
							"exporters": []any{"otlphttp/cw"},
						},
					},
				},
			},
			expected: nil,
		},
		{
			name: "default log stream when not specified",
			config: map[string]any{
				"exporters": map[string]any{
					"otlphttp/cw": map[string]any{
						"logs_endpoint": "https://logs.ap-southeast-1.amazonaws.com/v1/logs",
						"headers": map[string]any{
							"x-aws-log-group": "/my/logs",
						},
					},
				},
				"service": map[string]any{
					"pipelines": map[string]any{
						"logs/app": map[string]any{
							"exporters": []any{"otlphttp/cw"},
						},
					},
				},
			},
			expected: []logTarget{
				{logGroupName: "/my/logs", logStreamName: "default", region: "ap-southeast-1"},
			},
		},
		{
			name: "multiple exporters in multiple pipelines",
			config: map[string]any{
				"exporters": map[string]any{
					"otlphttp/test": map[string]any{
						"logs_endpoint": "https://logs.us-east-1.amazonaws.com/v1/logs",
						"headers": map[string]any{
							"x-aws-log-group":  "/test/telemetry",
							"x-aws-log-stream": "test",
						},
					},
					"otlphttp/app-logs": map[string]any{
						"logs_endpoint": "https://logs.us-east-1.amazonaws.com/v1/logs",
						"headers": map[string]any{
							"x-aws-log-group":  "/app/logs",
							"x-aws-log-stream": "app",
						},
					},
				},
				"service": map[string]any{
					"pipelines": map[string]any{
						"logs/test": map[string]any{
							"exporters": []any{"otlphttp/test"},
						},
						"logs/app": map[string]any{
							"exporters": []any{"otlphttp/app-logs"},
						},
					},
				},
			},
			expected: []logTarget{
				{logGroupName: "/test/telemetry", logStreamName: "test", region: "us-east-1"},
				{logGroupName: "/app/logs", logStreamName: "app", region: "us-east-1"},
			},
		},
		{
			name:     "no exporters",
			config:   map[string]any{},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(tt.config)
			result := p.findLogTargets(conf)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.ElementsMatch(t, tt.expected, result)
			}
		})
	}
}
