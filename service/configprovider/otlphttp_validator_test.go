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
			name: "valid metrics endpoint",
			config: map[string]any{
				"exporters": map[string]any{
					"otlphttp": map[string]any{
						"metrics_endpoint": "https://monitoring.us-east-1.amazonaws.com/v1/metrics",
					},
				},
			},
		},
		{
			name: "valid traces endpoint",
			config: map[string]any{
				"exporters": map[string]any{
					"otlphttp": map[string]any{
						"traces_endpoint": "https://xray.us-west-2.amazonaws.com/v1/traces",
					},
				},
			},
		},
		{
			name: "valid logs endpoint",
			config: map[string]any{
				"exporters": map[string]any{
					"otlphttp": map[string]any{
						"logs_endpoint": "https://logs.eu-west-1.amazonaws.com/v1/logs",
					},
				},
			},
		},
		{
			name: "valid generic endpoint",
			config: map[string]any{
				"exporters": map[string]any{
					"otlphttp": map[string]any{
						"endpoint": "https://monitoring.us-east-1.amazonaws.com",
					},
				},
			},
		},
		{
			name: "invalid cross-signal endpoint",
			config: map[string]any{
				"exporters": map[string]any{
					"otlphttp": map[string]any{
						"metrics_endpoint": "https://xray.us-east-1.amazonaws.com/v1/traces",
					},
				},
			},
			wantErr: true,
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
			name: "invalid wrong path",
			config: map[string]any{
				"exporters": map[string]any{
					"otlphttp": map[string]any{
						"metrics_endpoint": "https://monitoring.us-east-1.amazonaws.com/v2/metrics",
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
				assert.Contains(t, err.Error(), "does not support 3rd party")
			} else {
				require.NoError(t, err)
			}
		})
	}
}
