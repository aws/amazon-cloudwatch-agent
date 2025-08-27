// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/targetallocator"
	promcommon "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap"
	"gopkg.in/yaml.v3"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/testutil"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
)

const prometheusYamlFile = `
scrape_configs:
  - job_name: test-scrape-configs-job
    file_sd_configs:
      - files: ["/tmp/cwagent_ecs_auto_sd.yaml"]
`

func TestMetricsTranslator(t *testing.T) {
	testCases := map[string]struct {
		input   map[string]any
		wantID  string
		want    *prometheusreceiver.Config
		wantErr error
	}{
		"WithOtelConfig": {
			input:  testutil.GetJson(t, filepath.Join("testdata", "metrics", "config.json")),
			wantID: "prometheus",
			want: &prometheusreceiver.Config{
				PrometheusConfig: &prometheusreceiver.PromConfig{
					GlobalConfig: config.GlobalConfig{
						ScrapeInterval:     model.Duration(1 * time.Minute),
						ScrapeTimeout:      model.Duration(30 * time.Second),
						ScrapeProtocols:    config.DefaultScrapeProtocols,
						EvaluationInterval: model.Duration(1 * time.Minute),
					},
					ScrapeConfigs: []*config.ScrapeConfig{
						{
							ScrapeInterval:    model.Duration(1 * time.Minute),
							ScrapeTimeout:     model.Duration(30 * time.Second),
							JobName:           "prometheus_test_job",
							HonorTimestamps:   true,
							ScrapeProtocols:   config.DefaultScrapeProtocols,
							MetricsPath:       "/metrics",
							Scheme:            "http",
							EnableCompression: true,
							ServiceDiscoveryConfigs: discovery.Configs{
								discovery.StaticConfig{
									&targetgroup.Group{
										Targets: []model.LabelSet{
											{
												model.AddressLabel: "localhost:8000",
											},
										},
										Labels: map[model.LabelName]model.LabelValue{
											"label1": "test1",
										},
										Source: "0",
									},
								},
							},
							HTTPClientConfig: promcommon.HTTPClientConfig{
								FollowRedirects: true,
								EnableHTTP2:     true,
							},
						},
					},
				},
				TargetAllocator: &targetallocator.Config{
					CollectorID: "col-1234",
					ClientConfig: confighttp.ClientConfig{
						TLSSetting: configtls.ClientConfig{
							Config: configtls.Config{
								CAFile:         defaultTLSCaPath,
								CertFile:       defaultTLSCertPath,
								KeyFile:        defaultTLSKeyPath,
								ReloadInterval: 10 * time.Second,
							},
						},
					},
				},
			},
		},
		"WithPromConfig": {
			input:  testutil.GetJson(t, filepath.Join("testdata", "metrics", "config_prom.json")),
			wantID: "prometheus",
			want: &prometheusreceiver.Config{
				PrometheusConfig: &prometheusreceiver.PromConfig{
					GlobalConfig: config.GlobalConfig{
						ScrapeInterval:     model.Duration(1 * time.Minute),
						ScrapeTimeout:      model.Duration(30 * time.Second),
						ScrapeProtocols:    config.DefaultScrapeProtocols,
						EvaluationInterval: model.Duration(1 * time.Minute),
					},
					ScrapeConfigs: []*config.ScrapeConfig{
						{
							ScrapeInterval:    model.Duration(1 * time.Minute),
							ScrapeTimeout:     model.Duration(30 * time.Second),
							JobName:           "prometheus_test_job",
							HonorTimestamps:   true,
							ScrapeProtocols:   config.DefaultScrapeProtocols,
							MetricsPath:       "/metrics",
							Scheme:            "http",
							EnableCompression: true,
							ServiceDiscoveryConfigs: discovery.Configs{
								discovery.StaticConfig{
									&targetgroup.Group{
										Targets: []model.LabelSet{
											{
												model.AddressLabel: "localhost:8000",
											},
										},
										Labels: map[model.LabelName]model.LabelValue{
											"label1": "test1",
										},
										Source: "0",
									},
								},
							},
							HTTPClientConfig: promcommon.HTTPClientConfig{
								FollowRedirects: true,
								EnableHTTP2:     true,
							},
						},
					},
				},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			tt := NewTranslator(WithConfigKey(common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.PrometheusKey)))
			assert.EqualValues(t, testCase.wantID, tt.ID().String())
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*prometheusreceiver.Config)
				require.True(t, ok)

				assert.NoError(t, err)
				assert.Equal(t, testCase.want.PrometheusConfig.ScrapeConfigs, gotCfg.PrometheusConfig.ScrapeConfigs)
				assert.Equal(t, testCase.want.PrometheusConfig.GlobalConfig, gotCfg.PrometheusConfig.GlobalConfig)
				assert.Equal(t, testCase.want.TargetAllocator, gotCfg.TargetAllocator)
			}
		})
	}
}

func TestMetricsEmfTranslator(t *testing.T) {
	testCases := map[string]struct {
		input   map[string]any
		wantID  string
		want    *prometheusreceiver.Config
		wantErr error
	}{
		"WithOtelConfig": {
			input:  testutil.GetJson(t, filepath.Join("testdata", "metrics_emf", "config.json")),
			wantID: "prometheus",
			want: &prometheusreceiver.Config{
				PrometheusConfig: &prometheusreceiver.PromConfig{
					GlobalConfig: config.GlobalConfig{
						ScrapeInterval:     model.Duration(1 * time.Minute),
						ScrapeTimeout:      model.Duration(30 * time.Second),
						ScrapeProtocols:    config.DefaultScrapeProtocols,
						EvaluationInterval: model.Duration(1 * time.Minute),
					},
					ScrapeConfigs: []*config.ScrapeConfig{
						{
							ScrapeInterval:    model.Duration(1 * time.Minute),
							ScrapeTimeout:     model.Duration(30 * time.Second),
							JobName:           "prometheus_test_job",
							HonorTimestamps:   true,
							ScrapeProtocols:   config.DefaultScrapeProtocols,
							MetricsPath:       "/metrics",
							Scheme:            "http",
							EnableCompression: true,
							ServiceDiscoveryConfigs: discovery.Configs{
								discovery.StaticConfig{
									&targetgroup.Group{
										Targets: []model.LabelSet{
											{
												model.AddressLabel: "localhost:8000",
											},
										},
										Labels: map[model.LabelName]model.LabelValue{
											"label1": "test1",
										},
										Source: "0",
									},
								},
							},
							HTTPClientConfig: promcommon.HTTPClientConfig{
								FollowRedirects: true,
								EnableHTTP2:     true,
							},
						},
					},
				},
				TargetAllocator: &targetallocator.Config{
					CollectorID: "col-1234",
					ClientConfig: confighttp.ClientConfig{
						TLSSetting: configtls.ClientConfig{
							Config: configtls.Config{
								CAFile:         defaultTLSCaPath,
								CertFile:       defaultTLSCertPath,
								KeyFile:        defaultTLSKeyPath,
								ReloadInterval: 10 * time.Second,
							},
						},
					},
				},
			},
		},
		"WithPromConfig": {
			input:  testutil.GetJson(t, filepath.Join("testdata", "metrics_emf", "config_prom.json")),
			wantID: "prometheus",
			want: &prometheusreceiver.Config{
				PrometheusConfig: &prometheusreceiver.PromConfig{
					GlobalConfig: config.GlobalConfig{
						ScrapeInterval:     model.Duration(1 * time.Minute),
						ScrapeTimeout:      model.Duration(30 * time.Second),
						ScrapeProtocols:    config.DefaultScrapeProtocols,
						EvaluationInterval: model.Duration(1 * time.Minute),
					},
					ScrapeConfigs: []*config.ScrapeConfig{
						{
							ScrapeInterval:    model.Duration(1 * time.Minute),
							ScrapeTimeout:     model.Duration(30 * time.Second),
							JobName:           "prometheus_test_job",
							HonorTimestamps:   true,
							ScrapeProtocols:   config.DefaultScrapeProtocols,
							MetricsPath:       "/metrics",
							Scheme:            "http",
							EnableCompression: true,
							ServiceDiscoveryConfigs: discovery.Configs{
								discovery.StaticConfig{
									&targetgroup.Group{
										Targets: []model.LabelSet{
											{
												model.AddressLabel: "localhost:8000",
											},
										},
										Labels: map[model.LabelName]model.LabelValue{
											"label1": "test1",
										},
										Source: "0",
									},
								},
							},
							HTTPClientConfig: promcommon.HTTPClientConfig{
								FollowRedirects: true,
								EnableHTTP2:     true,
							},
						},
					},
				},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			tt := NewTranslator(WithConfigKey(common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.PrometheusKey)))
			assert.EqualValues(t, testCase.wantID, tt.ID().String())
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*prometheusreceiver.Config)
				require.True(t, ok)

				assert.NoError(t, err)
				assert.Equal(t, testCase.want.PrometheusConfig.ScrapeConfigs, gotCfg.PrometheusConfig.ScrapeConfigs)
				assert.Equal(t, testCase.want.PrometheusConfig.GlobalConfig, gotCfg.PrometheusConfig.GlobalConfig)
				assert.Equal(t, testCase.want.TargetAllocator, gotCfg.TargetAllocator)
			}
		})
	}
}

func TestAddDefaultECSRelabelConfigs_Success(t *testing.T) {
	ecsutil.GetECSUtilSingleton().Region = "us-test-2"

	// ecs_service_discovery is configured
	conf := confmap.NewFromStringMap(map[string]any{
		"logs": map[string]any{
			"metrics_collected": map[string]any{
				"prometheus": map[string]any{
					"prometheus_config_path": "env:PROMETHEUS_CONFIG_CONTENT",
					"ecs_service_discovery": map[string]any{
						"sd_frequency":   "50s",
						"sd_result_file": defaultECSSDfileName,
					},
				},
			},
		},
	})

	configKey := "logs.metrics_collected.prometheus"

	result, err := addDefaultECSRelabelConfigs([]byte(prometheusYamlFile), conf, configKey)
	assert.NoError(t, err)

	t.Logf("Generated YAML:\n%s", string(result))

	// Parse result to verify relabel configs were added
	var config map[string]interface{}
	err = yaml.Unmarshal(result, &config)
	assert.NoError(t, err)

	scrapeConfigs := config["scrape_configs"].([]interface{})
	scrapeConfig := scrapeConfigs[0].(map[string]interface{})
	relabelConfigs := scrapeConfig["relabel_configs"].([]interface{})

	assert.Len(t, relabelConfigs, 14, "Should add 14 relabel configs")
}

func TestAddDefaultRelabelConfigs_notECS(t *testing.T) {
	ecsutil.GetECSUtilSingleton().Region = ""

	// ecs_service_discovery is configured
	conf := confmap.NewFromStringMap(map[string]any{
		"logs": map[string]any{
			"metrics_collected": map[string]any{
				"prometheus": map[string]any{
					"prometheus_config_path": "env:PROMETHEUS_CONFIG_CONTENT",
					"ecs_service_discovery": map[string]any{
						"sd_frequency": "50s",
					},
				},
			},
		},
	})

	configKey := "logs.metrics_collected.prometheus"

	result, err := addDefaultECSRelabelConfigs([]byte(prometheusYamlFile), conf, configKey)
	assert.NoError(t, err)

	// Parse result to verify relabel configs were added
	var config map[any]any
	err = yaml.Unmarshal(result, &config)
	assert.NoError(t, err)

	scrapeConfigs := config["scrape_configs"].([]interface{})
	scrapeConfig := scrapeConfigs[0].(map[string]interface{})
	relabelConfigs := scrapeConfig["relabel_configs"]

	assert.Nil(t, relabelConfigs, "ScrapeConfig should have no relabel configs when not in ECS")
}

func TestAddDefaultRelabelConfigs_noEcsSdConfig(t *testing.T) {
	ecsutil.GetECSUtilSingleton().Region = "us-east-1"

	// ecs_service_discovery not configured
	conf := confmap.NewFromStringMap(map[string]any{
		"logs": map[string]any{
			"metrics_collected": map[string]any{
				"prometheus": map[string]any{
					"prometheus_config_path": "env:PROMETHEUS_CONFIG_CONTENT",
				},
			},
		},
	})

	configKey := "logs.metrics_collected.prometheus"

	result, err := addDefaultECSRelabelConfigs([]byte(prometheusYamlFile), conf, configKey)
	assert.NoError(t, err)

	// Parse result to verify relabel configs were added
	var config map[any]any
	err = yaml.Unmarshal(result, &config)
	assert.NoError(t, err)

	scrapeConfigs := config["scrape_configs"].([]interface{})
	scrapeConfig := scrapeConfigs[0].(map[string]interface{})
	relabelConfigs := scrapeConfig["relabel_configs"]

	assert.Nil(t, relabelConfigs, "ScrapeConfig should have no relabel configs when ecs_service_discovery is not configured")
}

func TestAddDefaultRelabelConfigs_mismatchEcsSdResultFile(t *testing.T) {
	ecsutil.GetECSUtilSingleton().Region = "us-east-1"

	// Create config with ecs_service_discovery configured
	conf := confmap.NewFromStringMap(map[string]any{
		"logs": map[string]any{
			"metrics_collected": map[string]any{
				"prometheus": map[string]any{
					"prometheus_config_path": "env:PROMETHEUS_CONFIG_CONTENT",
					"ecs_service_discovery": map[string]any{
						"sd_frequency":   "50s",
						"sd_result_file": "/tmp/random-sd-file.yaml",
					},
				},
			},
		},
	})

	configKey := "logs.metrics_collected.prometheus"

	result, err := addDefaultECSRelabelConfigs([]byte(prometheusYamlFile), conf, configKey)
	assert.NoError(t, err)

	// Parse result to verify relabel configs were added
	var config map[any]any
	err = yaml.Unmarshal(result, &config)
	assert.NoError(t, err)

	scrapeConfigs := config["scrape_configs"].([]interface{})
	scrapeConfig := scrapeConfigs[0].(map[string]interface{})
	relabelConfigs := scrapeConfig["relabel_configs"]

	assert.Nil(t, relabelConfigs, "ScrapeConfig should have no relabel configs when sd_result_file doesn't match")
}

func TestAddDefaultRelabelConfigs_emptyScrapeConfigs(t *testing.T) {
	ecsutil.GetECSUtilSingleton().Region = "us-east-1"

	prometheusYamlFile := `scrape_configs:`

	// ecs_service_discovery is configured
	conf := confmap.NewFromStringMap(map[string]any{
		"logs": map[string]any{
			"metrics_collected": map[string]any{
				"prometheus": map[string]any{
					"prometheus_config_path": "env:PROMETHEUS_CONFIG_CONTENT",
					"ecs_service_discovery": map[string]any{
						"sd_frequency": "50s",
					},
				},
			},
		},
	})

	configKey := "logs.metrics_collected.prometheus"

	result, err := addDefaultECSRelabelConfigs([]byte(prometheusYamlFile), conf, configKey)
	assert.NoError(t, err)

	// Parse result to verify relabel configs were added
	var config map[any]any
	err = yaml.Unmarshal(result, &config)
	assert.NoError(t, err)

	// Should not panic with empty scrape configs
	scrapeConfigs := config["scrape_configs"]
	assert.Nil(t, scrapeConfigs, "ScrapeConfig should remain empty")
}

func TestEscapeStrings(t *testing.T) {
	inputYAML := `
scrape_configs:
  - job_name: test
    metric_relabel_configs:
      - action: replace
        replacement: $1  
        regex: (.*)
`

	var config map[any]any
	require.NoError(t, yaml.Unmarshal([]byte(inputYAML), &config))

	escapeStrings(config)

	outputBytes, err := yaml.Marshal(config)
	require.NoError(t, err)
	output := string(outputBytes)

	assert.Contains(t, output, "replacement: $$$$1")
}

func normalizeYAML(s string) string {
	lines := strings.Split(s, "\n")
	var buf strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			buf.WriteString(trimmed + "\n")
		}
	}
	return buf.String()
}
