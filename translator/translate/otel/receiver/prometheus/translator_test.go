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
	"github.com/prometheus/prometheus/discovery/file"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/model/relabel"
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

	scrapeConfigs := []*config.ScrapeConfig{
		{
			JobName: "ecs-job",
			ServiceDiscoveryConfigs: discovery.Configs{
				&file.SDConfig{
					Files: []string{defaultECSSDfileName},
				},
			},
		},
		{
			JobName: "other-job",
			ServiceDiscoveryConfigs: discovery.Configs{
				&file.SDConfig{
					Files: []string{"/tmp/other_file.yml"},
				},
			},
		},
		{
			JobName: "ecs-job2",
			ServiceDiscoveryConfigs: discovery.Configs{
				&file.SDConfig{
					Files: []string{defaultECSSDfileName},
				},
			},
		},
	}

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
	addDefaultECSRelabelConfigs(scrapeConfigs, conf, configKey)

	// Should add configs because ecs_service_discovery is explicitly configured
	assert.Len(t, scrapeConfigs[0].RelabelConfigs, 13, "Should add relabel configs when ecs_service_discovery is explicitly configured")
	validateRelabelFields(t, scrapeConfigs[0])
	assert.Empty(t, scrapeConfigs[1].RelabelConfigs, "Other job should not have relabel configs")
	assert.Len(t, scrapeConfigs[2].RelabelConfigs, 13, "Should add relabel configs when ecs_service_discovery is explicitly configured")
	validateRelabelFields(t, scrapeConfigs[2])

}

func TestAddDefaultRelabelConfigs_notECS(t *testing.T) {
	ecsutil.GetECSUtilSingleton().Region = ""

	scrapeConfigWithFileSD := &config.ScrapeConfig{
		JobName: "test-scrape-configs-job",
		ServiceDiscoveryConfigs: discovery.Configs{
			&file.SDConfig{
				Files: []string{defaultECSSDfileName},
			},
		},
		RelabelConfigs: []*relabel.Config{},
	}

	scrapeConfigs := []*config.ScrapeConfig{scrapeConfigWithFileSD}

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

	addDefaultECSRelabelConfigs(scrapeConfigs, conf, configKey)

	assert.Len(t, scrapeConfigWithFileSD.RelabelConfigs, 0, "ScrapeConfig should have no relabel configs when not in ECS")
}

func TestAddDefaultRelabelConfigs_noEcsSdConfig(t *testing.T) {
	ecsutil.GetECSUtilSingleton().Region = "us-east-1"

	scrapeConfigWithFileSD := &config.ScrapeConfig{
		JobName: "test-scrape-configs-job",
		ServiceDiscoveryConfigs: discovery.Configs{
			&file.SDConfig{
				Files: []string{defaultECSSDfileName},
			},
		},
		RelabelConfigs: []*relabel.Config{},
	}

	scrapeConfigs := []*config.ScrapeConfig{scrapeConfigWithFileSD}

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

	addDefaultECSRelabelConfigs(scrapeConfigs, conf, configKey)

	assert.Len(t, scrapeConfigWithFileSD.RelabelConfigs, 0, "ScrapeConfig should have no relabel configs when ecs_service_discovery is not configured")
}

func TestAddDefaultRelabelConfigs_mismatchEcsSdResultFile(t *testing.T) {
	ecsutil.GetECSUtilSingleton().Region = "us-east-1"

	scrapeConfigWithFileSD := &config.ScrapeConfig{
		JobName: "test-scrape-configs-job",
		ServiceDiscoveryConfigs: discovery.Configs{
			&file.SDConfig{
				Files: []string{defaultECSSDfileName},
			},
		},
		RelabelConfigs: []*relabel.Config{},
	}

	scrapeConfigs := []*config.ScrapeConfig{scrapeConfigWithFileSD}

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

	addDefaultECSRelabelConfigs(scrapeConfigs, conf, configKey)

	assert.Len(t, scrapeConfigWithFileSD.RelabelConfigs, 0, "ScrapeConfig should have no relabel configs when sd_result_file doesn't match")
}

func TestAddDefaultRelabelConfigs_emptyScrapeConfigs(t *testing.T) {
	ecsutil.GetECSUtilSingleton().Region = "us-east-1"

	scrapeConfigs := []*config.ScrapeConfig{}

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

	addDefaultECSRelabelConfigs(scrapeConfigs, conf, configKey)

	// Should not panic with empty scrape configs
	assert.Len(t, scrapeConfigs, 0, "Should remain empty")
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

func validateRelabelFields(t *testing.T, scrapeConfigWithFileSD *config.ScrapeConfig) {
	assert.Equal(t, "ClusterName", scrapeConfigWithFileSD.RelabelConfigs[0].TargetLabel)
	assert.Equal(t, "TaskClusterName", scrapeConfigWithFileSD.RelabelConfigs[1].TargetLabel)
	assert.Equal(t, "container_name", scrapeConfigWithFileSD.RelabelConfigs[2].TargetLabel)
	assert.Equal(t, "LaunchType", scrapeConfigWithFileSD.RelabelConfigs[3].TargetLabel)
	assert.Equal(t, "StartedBy", scrapeConfigWithFileSD.RelabelConfigs[4].TargetLabel)
	assert.Equal(t, "TaskGroup", scrapeConfigWithFileSD.RelabelConfigs[5].TargetLabel)
	assert.Equal(t, "TaskDefinitionFamily", scrapeConfigWithFileSD.RelabelConfigs[6].TargetLabel)
	assert.Equal(t, "TaskRevision", scrapeConfigWithFileSD.RelabelConfigs[7].TargetLabel)
	assert.Equal(t, "InstanceType", scrapeConfigWithFileSD.RelabelConfigs[8].TargetLabel)
	assert.Equal(t, "SubnetId", scrapeConfigWithFileSD.RelabelConfigs[9].TargetLabel)
	assert.Equal(t, "VpcId", scrapeConfigWithFileSD.RelabelConfigs[10].TargetLabel)
	assert.Equal(t, "TaskId", scrapeConfigWithFileSD.RelabelConfigs[11].TargetLabel)
	assert.Equal(t, "app_x", scrapeConfigWithFileSD.RelabelConfigs[12].TargetLabel)
}
