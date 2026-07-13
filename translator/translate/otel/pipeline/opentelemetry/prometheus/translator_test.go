// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
)

func TestPrometheusTranslator(t *testing.T) {
	tt := NewTranslator()
	assert.EqualValues(t, "metrics/otel_prometheus", tt.ID().String())

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
				assert.Equal(t, "prometheus/opentelemetry", got.Receivers.Keys()[0].String())
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
	assert.Equal(t, 2, got.Processors.Len())
	assert.Equal(t, "transform/prometheus_scope", got.Processors.Keys()[0].String())
	assert.Equal(t, "transform/set_cluster_name", got.Processors.Keys()[1].String())
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
	assert.Equal(t, 1, got.Processors.Len())
	assert.Equal(t, "transform/prometheus_scope", got.Processors.Keys()[0].String())
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

func TestEscapeDollarDigit(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "bare $1", in: "replacement: $1", want: "replacement: $$$$1"},
		{name: "bare $0", in: "$0", want: "$$$$0"},
		{name: "bare $9", in: "value=$9", want: "value=$$$$9"},
		{name: "multiple refs", in: "$1 and $2 and $3", want: "$$$$1 and $$$$2 and $$$$3"},
		{name: "dollar non-digit", in: "$HOME and $PATH", want: "$HOME and $PATH"},
		{name: "no dollar", in: "no dollars here", want: "no dollars here"},
		{name: "empty string", in: "", want: ""},
		{name: "dollar at end", in: "trailing$", want: "trailing$"},
		{name: "multi-digit $10", in: "$10", want: "$$$$10"},
		{name: "mixed text", in: "tag: k8s.label.$1 and $FOO", want: "tag: k8s.label.$$$$1 and $FOO"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeDollarDigit(tt.in)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPrometheusReceiverTranslatorDollarEscape(t *testing.T) {
	// Create a prometheus config with relabel_configs that use $1 capture group references.
	// Without escaping, the OTel expandconverter would treat $1 as an env var and blank it out.
	content := []byte(`scrape_configs:
  - job_name: k8s_pods
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_name]
        regex: (.*)
        target_label: pod
        replacement: $1
      - source_labels: [__address__, __meta_kubernetes_pod_annotation_prometheus_io_port]
        regex: ([^:]+)(?::\d+)?;(\d+)
        target_label: __address__
        replacement: $1:$2
`)
	dir := t.TempDir()
	path := filepath.Join(dir, "prometheus_relabel.yml")
	require.NoError(t, os.WriteFile(path, content, 0600))

	conf := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"prometheus": map[string]interface{}{
					"config_path": path,
				},
			},
		},
	})

	receiver := &prometheusReceiverTranslator{}
	cfg, err := receiver.Translate(conf)
	require.NoError(t, err)

	promCfg := cfg.(*prometheusreceiver.Config)
	require.Len(t, promCfg.PrometheusConfig.ScrapeConfigs, 1)

	relabelCfgs := promCfg.PrometheusConfig.ScrapeConfigs[0].RelabelConfigs
	require.Len(t, relabelCfgs, 2)
	// $$$$1 survives the resolver's escapeDollarSigns ($$$$→$$) and the expandconverter ($$→$)
	// to produce the final $1 at runtime.
	assert.Equal(t, "$$$$1", relabelCfgs[0].Replacement, "first relabel $1 should be escaped to $$$$1 for resolver+expandconverter")
	assert.Equal(t, "$$$$1:$$$$2", relabelCfgs[1].Replacement, "second relabel $1:$2 should be escaped to $$$$1:$$$$2 for resolver+expandconverter")
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
	content := []byte(`scrape_configs:
  - job_name: test
    static_configs:
      - targets: ['localhost:9090']
`)
	dir := t.TempDir()
	path := filepath.Join(dir, "prometheus.yml")
	require.NoError(t, os.WriteFile(path, content, 0600))
	return path
}

func createTempPlainPromConfig(t *testing.T) string {
	t.Helper()
	content := []byte(`global:
  scrape_interval: 15s
scrape_configs:
  - job_name: plain_test
    static_configs:
      - targets: ['localhost:9090']
`)
	dir := t.TempDir()
	path := filepath.Join(dir, "prometheus.yml")
	require.NoError(t, os.WriteFile(path, content, 0600))
	return path
}

func TestPrometheusReceiverTranslatorPlainFormat(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"prometheus": map[string]interface{}{
					"config_path": createTempPlainPromConfig(t),
				},
			},
		},
	})

	receiver := &prometheusReceiverTranslator{}
	cfg, err := receiver.Translate(conf)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
}

func TestPrometheusTranslatorK8sMode(t *testing.T) {
	context.CurrentContext().SetKubernetesMode(config.ModeEKS)
	defer context.CurrentContext().SetKubernetesMode("")

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
	assert.Equal(t, 2, got.Processors.Len()) // scope + set_cluster_name
	keys := make([]string, 0, got.Processors.Len())
	for _, k := range got.Processors.Keys() {
		keys = append(keys, k.String())
	}
	assert.Contains(t, keys, "transform/prometheus_scope")
	assert.Contains(t, keys, "transform/set_cluster_name")
}
