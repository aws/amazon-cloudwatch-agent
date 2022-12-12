// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheusreceiver

import (
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
	cfg "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/file"
	"github.com/prometheus/prometheus/discovery/kubernetes"
	"github.com/prometheus/prometheus/model/relabel"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	pt := NewTranslator()
	require.EqualValues(t, "prometheus", pt.Type())
	temp := t.TempDir()
	prometheusConfigFileName := filepath.Join(temp, "prometheus.yaml")
	ecsSdFileName := filepath.Join(temp, "ecs_sd_results.yaml")
	testCases := map[string]struct {
		input            map[string]interface{}
		prometheusConfig string
		want             *prometheusreceiver.Config
		wantErr          error
	}{
		"WithoutPrometheusKey": {
			input: map[string]interface{}{},
			wantErr: &common.MissingKeyError{
				Type:    "prometheus",
				JsonKey: "logs::metrics_collected::prometheus",
			},
		},
		"WithPrometheusEKS": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"prometheus_config_path": prometheusConfigFileName,
						},
					},
				},
			},
			prometheusConfig: `
global:
  scrape_interval: 5m
  scrape_timeout: 5s
scrape_configs:
  - job_name: kubernetes-service-endpoints
    sample_limit: 10000
    kubernetes_sd_configs:
      - role: endpoints
    relabel_configs:
      - action: keep
        regex: true
        source_labels:
          - __meta_kubernetes_service_annotation_prometheus_io_scrape`,
			want: &prometheusreceiver.Config{
				PrometheusConfig: &config.Config{
					GlobalConfig: config.GlobalConfig{
						ScrapeInterval:     300000000000, // 5m
						ScrapeTimeout:      5000000000,   // 5s
						EvaluationInterval: 60000000000,  // 1m - default
					},
					ScrapeConfigs: []*config.ScrapeConfig{
						{
							JobName:     "kubernetes-service-endpoints",
							SampleLimit: 10000,
							ServiceDiscoveryConfigs: []discovery.Config{
								&kubernetes.SDConfig{
									Role: kubernetes.RoleEndpoint,
									HTTPClientConfig: cfg.HTTPClientConfig{
										FollowRedirects: true, // default
										EnableHTTP2:     true, // default
									},
								},
							},
							RelabelConfigs: []*relabel.Config{
								{
									SourceLabels: model.LabelNames{"__meta_kubernetes_service_annotation_prometheus_io_scrape"},
									Regex:        relabel.MustNewRegexp("true"),
									Action:       relabel.Keep,
									Separator:    ";",  // default
									Replacement:  "$1", // default
								},
							},
							HonorTimestamps: true,       // default
							MetricsPath:     "/metrics", // default
							Scheme:          "http",     // default
							HTTPClientConfig: cfg.HTTPClientConfig{
								FollowRedirects: true, // default
								EnableHTTP2:     true, // default
							},
							ScrapeInterval: 300000000000, // 5m
							ScrapeTimeout:  5000000000,   // 5s
						},
					},
				},
			},
		},
		"WithPrometheusEcs": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"prometheus_config_path": prometheusConfigFileName,
							"ecs_service_discovery": map[string]interface{}{
								"sd_target_cluster": "TestTargetCluster",
								"sd_cluster_region": "TestRegion",
								"sd_result_file":    ecsSdFileName,
								"sd_frequency":      "30s",
								"docker_label": map[string]interface{}{
									"sd_job_name_label":     "ECS_PROMETHEUS_JOB_NAME_1",
									"sd_metrics_path_label": "ECS_PROMETHEUS_METRICS_PATH",
									"sd_port_label":         "ECS_PROMETHEUS_EXPORTER_PORT_SUBSET",
								},
							},
						},
					},
				},
			},
			prometheusConfig: fmt.Sprintf(`
global:
  scrape_interval: 5m
  scrape_timeout: 5s
scrape_configs:
  - job_name: cwagent-ecs-file-sd-config
    sample_limit: 10000
    file_sd_configs:
      - files: [ "%s" ]`, strings.ReplaceAll(ecsSdFileName, "\\", "\\\\")),
			want: &prometheusreceiver.Config{
				PrometheusConfig: &config.Config{
					GlobalConfig: config.GlobalConfig{
						ScrapeInterval:     300000000000, // 5m
						ScrapeTimeout:      5000000000,   // 5s
						EvaluationInterval: 60000000000,  // 1m - default
					},
					ScrapeConfigs: []*config.ScrapeConfig{
						{
							JobName:     "cwagent-ecs-file-sd-config",
							SampleLimit: 10000,
							ServiceDiscoveryConfigs: []discovery.Config{
								&file.SDConfig{
									Files:           []string{ecsSdFileName},
									RefreshInterval: model.Duration(300000000000), // default
								},
							},
							RelabelConfigs:       EcsRelabelConfigs,
							MetricRelabelConfigs: EcsMetricRelabelConfigs,
							HonorTimestamps:      true,       // default
							MetricsPath:          "/metrics", // default
							Scheme:               "http",     // default
							HTTPClientConfig: cfg.HTTPClientConfig{
								FollowRedirects: true, // default
								EnableHTTP2:     true, // default
							},
							ScrapeInterval: 300000000000, // 5m
							ScrapeTimeout:  5000000000,   // 5s
						},
					},
				},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			if testCase.prometheusConfig != "" {
				err := os.WriteFile(prometheusConfigFileName, []byte(testCase.prometheusConfig), os.ModePerm)
				require.NoError(t, err)
			}
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := pt.Translate(conf)
			require.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*prometheusreceiver.Config)
				require.True(t, ok)
				require.Equal(t, testCase.want.PrometheusConfig, gotCfg.PrometheusConfig)
			}
			os.Remove(prometheusConfigFileName)
			os.Remove(ecsSdFileName)
		})
	}
}
