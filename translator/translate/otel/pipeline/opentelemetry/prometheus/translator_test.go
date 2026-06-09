// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
)

func TestPrometheusTranslator(t *testing.T) {
	tt := NewTranslator()
	assert.EqualValues(t, "metrics/prometheus", tt.ID().String())

	testCases := map[string]struct {
		input   map[string]interface{}
		wantErr bool
	}{
		"WithNilConf": {
			input:   nil,
			wantErr: true,
		},
		"WithoutPrometheusKey": {
			input:   map[string]interface{}{},
			wantErr: true,
		},
		"WithValidConfig": {
			input: map[string]interface{}{
				"opentelemetry": map[string]interface{}{
					"collect": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"config_path": createTempPromConfig(t),
						},
					},
				},
			},
			wantErr: false,
		},
		"WithClusterName": {
			input: map[string]interface{}{
				"opentelemetry": map[string]interface{}{
					"collect": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"config_path":  createTempPromConfig(t),
							"cluster_name": "my-cluster",
						},
					},
				},
			},
			wantErr: false,
		},
		"WithInvalidClusterName": {
			input: map[string]interface{}{
				"opentelemetry": map[string]interface{}{
					"collect": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"config_path":  createTempPromConfig(t),
							"cluster_name": `bad"name`,
						},
					},
				},
			},
			wantErr: true,
		},
		"WithMissingConfigFile": {
			input: map[string]interface{}{
				"opentelemetry": map[string]interface{}{
					"collect": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"config_path": "/nonexistent/path.yml",
						},
					},
				},
			},
			wantErr: false, // pipeline translates fine; file error surfaces at receiver Translate time
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var conf *confmap.Conf
			if tc.input != nil {
				conf = confmap.NewFromStringMap(tc.input)
			}
			got, err := tt.Translate(conf)
			if tc.wantErr {
				require.Error(t, err)
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, got)
				assert.Equal(t, 1, got.Receivers.Len())
				assert.Equal(t, 1, got.Exporters.Len())
				assert.Equal(t, 1, got.Connectors.Len())
				assert.Equal(t, "prometheus", got.Receivers.Keys()[0].String())
				assert.Equal(t, "forward/opentelemetry", got.Exporters.Keys()[0].String())
			}
		})
	}
}

func TestPrometheusTranslatorClusterNameProcessor(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"prometheus": map[string]interface{}{
					"config_path":  createTempPromConfig(t),
					"cluster_name": "test-cluster",
				},
			},
		},
	})

	tt := NewTranslator()
	got, err := tt.Translate(conf)
	require.NoError(t, err)
	assert.Equal(t, 1, got.Processors.Len())
	assert.Equal(t, "transform/set_cluster_name", got.Processors.Keys()[0].String())
}

func TestPrometheusTranslatorNoClusterNameProcessor(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"prometheus": map[string]interface{}{
					"config_path": createTempPromConfig(t),
				},
			},
		},
	})

	tt := NewTranslator()
	got, err := tt.Translate(conf)
	require.NoError(t, err)
	assert.Equal(t, 0, got.Processors.Len())
}

func TestPrometheusReceiverTranslator(t *testing.T) {
	configPath := createTempPromConfig(t)
	conf := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"prometheus": map[string]interface{}{
					"config_path": configPath,
				},
			},
		},
	})

	receiver := &prometheusReceiverTranslator{}
	cfg, err := receiver.Translate(conf)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
}

func TestPrometheusReceiverTranslatorMissingFile(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"prometheus": map[string]interface{}{
					"config_path": "/nonexistent/path.yml",
				},
			},
		},
	})

	receiver := &prometheusReceiverTranslator{}
	cfg, err := receiver.Translate(conf)
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "unable to read prometheus config")
}

func createTempPromConfig(t *testing.T) string {
	t.Helper()
	content := []byte(`config:
  scrape_configs:
    - job_name: test
      static_configs:
        - targets: ['localhost:9090']
`)
	dir := t.TempDir()
	path := filepath.Join(dir, "prometheus.yml")
	require.NoError(t, os.WriteFile(path, content, 0600))
	return path
}
