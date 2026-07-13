// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package configprovider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
)

func TestOTLPHTTPValidator(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]any
		wantErr bool
	}{
		{
			name: "valid amazonaws.com endpoint",
			config: map[string]any{
				"exporters": map[string]any{
					"otlphttp": map[string]any{
						"endpoint": "https://monitoring.us-east-1.amazonaws.com",
					},
				},
			},
		},
		{
			name: "valid api.aws endpoint",
			config: map[string]any{
				"exporters": map[string]any{
					"otlphttp": map[string]any{
						"endpoint": "https://monitoring.us-east-1.api.aws",
					},
				},
			},
		},
		{
			name: "valid metrics_endpoint",
			config: map[string]any{
				"exporters": map[string]any{
					"otlphttp": map[string]any{
						"metrics_endpoint": "https://monitoring.us-east-1.amazonaws.com/v1/metrics",
					},
				},
			},
		},
		{
			name: "valid traces_endpoint",
			config: map[string]any{
				"exporters": map[string]any{
					"otlphttp": map[string]any{
						"traces_endpoint": "https://xray.us-west-2.amazonaws.com/v1/traces",
					},
				},
			},
		},
		{
			name: "valid logs_endpoint",
			config: map[string]any{
				"exporters": map[string]any{
					"otlphttp": map[string]any{
						"logs_endpoint": "https://logs.eu-west-1.amazonaws.com/v1/logs",
					},
				},
			},
		},
		{
			name: "valid endpoint without scheme",
			config: map[string]any{
				"exporters": map[string]any{
					"otlphttp": map[string]any{
						"endpoint": "xray.us-west-2.amazonaws.com",
					},
				},
			},
		},
		{
			name: "valid china partition",
			config: map[string]any{
				"exporters": map[string]any{
					"otlphttp": map[string]any{
						"endpoint": "https://monitoring.cn-north-1.amazonaws.com.cn",
					},
				},
			},
		},
		{
			name: "invalid third party endpoint",
			config: map[string]any{
				"exporters": map[string]any{
					"otlphttp": map[string]any{
						"endpoint": "https://example.com/v1/metrics",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid third party metrics_endpoint",
			config: map[string]any{
				"exporters": map[string]any{
					"otlphttp": map[string]any{
						"metrics_endpoint": "https://otel-collector.example.com/v1/metrics",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid third party traces_endpoint",
			config: map[string]any{
				"exporters": map[string]any{
					"otlphttp": map[string]any{
						"traces_endpoint": "https://jaeger.example.com/v1/traces",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid third party logs_endpoint",
			config: map[string]any{
				"exporters": map[string]any{
					"otlphttp": map[string]any{
						"logs_endpoint": "https://splunk.example.com/v1/logs",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "non-otlphttp exporter is ignored",
			config: map[string]any{
				"exporters": map[string]any{
					"prometheus": map[string]any{
						"endpoint": "https://example.com/metrics",
					},
				},
			},
		},
		{
			name: "invalid spoofed suffix",
			config: map[string]any{
				"exporters": map[string]any{
					"otlphttp": map[string]any{
						"endpoint": "https://evil-amazonaws.com",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewOTLPHTTPValidatorFactory().Create(confmap.ConverterSettings{})
			err := validator.Convert(context.Background(), confmap.NewFromStringMap(tt.config))
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "invalid AWS endpoint")
			} else {
				require.NoError(t, err)
			}
		})
	}
}
