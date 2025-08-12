// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package hostmetrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
)

func TestTranslator_Translate(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]interface{}
		want        map[string]interface{}
		wantErr     bool
		expectedErr string
	}{
		{
			name: "valid_load_config",
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"load": map[string]interface{}{
							"measurement":                   []string{"load_average_1m", "load_average_5m", "load_average_15m"},
							"metrics_collection_interval": 60,
						},
					},
				},
			},
			want: map[string]interface{}{
				"collection_interval": "1m0s",
				"scrapers": map[string]interface{}{
					"load": struct{}{},
				},
			},
			wantErr: false,
		},
		{
			name: "missing_load_config",
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"cpu": map[string]interface{}{
							"measurement": []string{"cpu_usage_idle"},
						},
					},
				},
			},
			want:        nil,
			wantErr:     true,
			expectedErr: "missing key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			translator := NewTranslator()
			conf := confmap.NewFromStringMap(tt.input)

			got, err := translator.Translate(conf)

			if tt.wantErr {
				require.Error(t, err)
				if tt.expectedErr != "" {
					assert.Contains(t, err.Error(), tt.expectedErr)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, got)

			gotConfig, ok := got.(map[string]interface{})
			require.True(t, ok, "expected map[string]interface{}, got %T", got)

			assert.Equal(t, tt.want, gotConfig)
		})
	}
}

func TestTranslator_ID(t *testing.T) {
	translator := NewTranslator()
	id := translator.ID()
	
	assert.Equal(t, "hostmetrics", id.Type().String())
	assert.Equal(t, "", id.Name())
}