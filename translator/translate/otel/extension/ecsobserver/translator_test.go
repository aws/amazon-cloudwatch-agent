// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsobserver

import (
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator_Translate(t *testing.T) {
	tests := []struct {
		name     string
		config   map[string]interface{}
		expected *ecsobserver.Config
		wantErr  bool
	}{
		{
			name: "full agent config",
			config: map[string]interface{}{
				"agent": map[string]interface{}{
					"region": "us-west-2",
				},
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"prometheus_config_path": "{prometheusFileName}",
							"ecs_service_discovery": map[string]interface{}{
								"sd_frequency":      "1m",
								"sd_target_cluster": "my-ecs-cluster",
								"sd_cluster_region": "us-west-2",
								"sd_result_file":    "{ecsSdFileName}",
								"docker_label": map[string]interface{}{
									"sd_port_label":         "ECS_PROMETHEUS_EXPORTER_PORT",
									"sd_metrics_path_label": "ECS_PROMETHEUS_METRICS_PATH",
									"sd_job_name_label":     "ECS_PROMETHEUS_JOB_NAME",
								},
								"task_definition_list": []interface{}{
									map[string]interface{}{
										"sd_job_name":                    "java-prometheus",
										"sd_metrics_path":                "/metrics",
										"sd_metrics_ports":               "9404;9406",
										"sd_task_definition_arn_pattern": ".*:task-definition/.*javajmx.*:[0-9]+",
									},
									map[string]interface{}{
										"sd_job_name":                    "envoy-prometheus",
										"sd_metrics_path":                "/stats/prometheus",
										"sd_container_name_pattern":      "^envoy$",
										"sd_metrics_ports":               "9901",
										"sd_task_definition_arn_pattern": ".*:task-definition/.*appmesh.*:23",
									},
								},
								"service_name_list_for_tasks": []interface{}{
									map[string]interface{}{
										"sd_job_name":             "nginx-prometheus",
										"sd_metrics_path":         "/metrics",
										"sd_metrics_ports":        "9113",
										"sd_service_name_pattern": "^nginx-.*",
									},
									map[string]interface{}{
										"sd_job_name":               "haproxy-prometheus",
										"sd_metrics_path":           "/stats/metrics",
										"sd_container_name_pattern": "^haproxy$",
										"sd_metrics_ports":          "8404",
										"sd_service_name_pattern":   ".*haproxy-service.*",
									},
								},
							},
						},
					},
					"force_flush_interval": "1m",
					"credentials": map[string]interface{}{
						"role_arn": "arn:aws:iam::123456789012:role/my-role",
					},
					"endpoint_override": "https://monitoring.us-west-2.amazonaws.com",
				},
			},
			expected: &ecsobserver.Config{
				RefreshInterval: time.Minute,
				ClusterName:     "my-ecs-cluster",
				ClusterRegion:   "us-west-2",
				ResultFile:      "{ecsSdFileName}",
				DockerLabels: []ecsobserver.DockerLabelConfig{
					{
						PortLabel:        "ECS_PROMETHEUS_EXPORTER_PORT",
						JobNameLabel:     "ECS_PROMETHEUS_JOB_NAME",
						MetricsPathLabel: "ECS_PROMETHEUS_METRICS_PATH",
					},
				},
				TaskDefinitions: []ecsobserver.TaskDefinitionConfig{
					{
						CommonExporterConfig: ecsobserver.CommonExporterConfig{
							JobName:      "java-prometheus",
							MetricsPath:  "/metrics",
							MetricsPorts: []int{9404, 9406},
						},
						ArnPattern:           ".*:task-definition/.*javajmx.*:[0-9]+",
						ContainerNamePattern: "",
					},
					{
						CommonExporterConfig: ecsobserver.CommonExporterConfig{
							JobName:      "envoy-prometheus",
							MetricsPath:  "/stats/prometheus",
							MetricsPorts: []int{9901},
						},
						ArnPattern:           ".*:task-definition/.*appmesh.*:23",
						ContainerNamePattern: "^envoy$",
					},
				},
				Services: []ecsobserver.ServiceConfig{
					{
						CommonExporterConfig: ecsobserver.CommonExporterConfig{
							JobName:      "nginx-prometheus",
							MetricsPath:  "/metrics",
							MetricsPorts: []int{9113},
						},
						NamePattern:          "^nginx-.*",
						ContainerNamePattern: "",
					},
					{
						CommonExporterConfig: ecsobserver.CommonExporterConfig{
							JobName:      "haproxy-prometheus",
							MetricsPath:  "/stats/metrics",
							MetricsPorts: []int{8404},
						},
						NamePattern:          ".*haproxy-service.*",
						ContainerNamePattern: "^haproxy$",
					},
				},
			},
		},
		{
			name: "basic docker label config",
			config: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"ecs_service_discovery": map[string]interface{}{
								"sd_frequency":      "1m",
								"sd_target_cluster": "my-ecs-cluster",
								"sd_cluster_region": "us-west-2",
								"sd_result_file":    "/tmp/cwagent_ecs_auto_sd.yaml",
								"docker_label":      map[string]interface{}{},
							},
						},
					},
				},
			},
			expected: &ecsobserver.Config{
				RefreshInterval: time.Minute,
				ClusterName:     "my-ecs-cluster",
				ClusterRegion:   "us-west-2",
				ResultFile:      "/tmp/cwagent_ecs_auto_sd.yaml",
				DockerLabels: []ecsobserver.DockerLabelConfig{
					{
						JobNameLabel:     defaultJobNameLabel,
						MetricsPathLabel: defaultMetricsPath,
						PortLabel:        defaultPortLabel,
					},
				},
			},
		},
		{
			name: "custom docker label config",
			config: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"ecs_service_discovery": map[string]interface{}{
								"sd_frequency":      "15s",
								"sd_target_cluster": "my-ecs-cluster",
								"sd_cluster_region": "us-west-2",
								"sd_result_file":    "/tmp/cwagent_ecs_auto_sd.yaml",
								"docker_label": map[string]interface{}{
									"sd_port_label":         "ECS_PROMETHEUS_EXPORTER_PORT_SUBSET_A",
									"sd_metrics_path_label": "MY_METRICS_PATH",
									"sd_job_name_label":     "ECS_PROMETHEUS_JOB_NAME",
								},
							},
						},
					},
				},
			},
			expected: &ecsobserver.Config{
				RefreshInterval: 15 * time.Second,
				ClusterName:     "my-ecs-cluster",
				ClusterRegion:   "us-west-2",
				ResultFile:      "/tmp/cwagent_ecs_auto_sd.yaml",
				DockerLabels: []ecsobserver.DockerLabelConfig{
					{
						JobNameLabel:     "ECS_PROMETHEUS_JOB_NAME",
						MetricsPathLabel: "MY_METRICS_PATH",
						PortLabel:        "ECS_PROMETHEUS_EXPORTER_PORT_SUBSET_A",
					},
				},
			},
		},
		{
			name: "task definition config",
			config: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"ecs_service_discovery": map[string]interface{}{
								"sd_frequency":      "5m",
								"sd_target_cluster": "my-ecs-cluster",
								"sd_cluster_region": "us-west-2",
								"sd_result_file":    "/tmp/cwagent_ecs_auto_sd.yaml",
								"task_definition_list": []interface{}{
									map[string]interface{}{
										"sd_job_name":                    "java-prometheus",
										"sd_metrics_path":                "/metrics",
										"sd_metrics_ports":               "9404;9406",
										"sd_task_definition_arn_pattern": ".*:task-definition/.*javajmx.*:[0-9]+",
									},
									map[string]interface{}{
										"sd_job_name":                    "envoy-prometheus",
										"sd_metrics_path":                "/stats/prometheus",
										"sd_container_name_pattern":      "^envoy$",
										"sd_metrics_ports":               "9901",
										"sd_task_definition_arn_pattern": ".*:task-definition/.*appmesh.*:23",
									},
								},
							},
						},
					},
				},
			},
			expected: &ecsobserver.Config{
				RefreshInterval: 5 * time.Minute,
				ClusterName:     "my-ecs-cluster",
				ClusterRegion:   "us-west-2",
				ResultFile:      "/tmp/cwagent_ecs_auto_sd.yaml",
				TaskDefinitions: []ecsobserver.TaskDefinitionConfig{
					{
						CommonExporterConfig: ecsobserver.CommonExporterConfig{
							JobName:      "java-prometheus",
							MetricsPath:  "/metrics",
							MetricsPorts: []int{9404, 9406},
						},
						ArnPattern:           ".*:task-definition/.*javajmx.*:[0-9]+",
						ContainerNamePattern: "",
					},
					{
						CommonExporterConfig: ecsobserver.CommonExporterConfig{
							JobName:      "envoy-prometheus",
							MetricsPath:  "/stats/prometheus",
							MetricsPorts: []int{9901},
						},
						ArnPattern:           ".*:task-definition/.*appmesh.*:23",
						ContainerNamePattern: "^envoy$",
					},
				},
			},
		},
		{
			name: "service name config",
			config: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"ecs_service_discovery": map[string]interface{}{
								"sd_frequency":      "5m",
								"sd_target_cluster": "my-ecs-cluster",
								"sd_cluster_region": "us-west-2",
								"sd_result_file":    "/tmp/cwagent_ecs_auto_sd.yaml",
								"service_name_list_for_tasks": []interface{}{
									map[string]interface{}{
										"sd_job_name":             "nginx-prometheus",
										"sd_metrics_path":         "/metrics",
										"sd_metrics_ports":        "9113",
										"sd_service_name_pattern": "^nginx-.*",
									},
									map[string]interface{}{
										"sd_job_name":               "haproxy-prometheus",
										"sd_metrics_path":           "/stats/metrics",
										"sd_container_name_pattern": "^haproxy$",
										"sd_metrics_ports":          "8404",
										"sd_service_name_pattern":   ".*haproxy-service.*",
									},
								},
							},
						},
					},
				},
			},
			expected: &ecsobserver.Config{
				RefreshInterval: 5 * time.Minute,
				ClusterName:     "my-ecs-cluster",
				ClusterRegion:   "us-west-2",
				ResultFile:      "/tmp/cwagent_ecs_auto_sd.yaml",
				Services: []ecsobserver.ServiceConfig{
					{
						CommonExporterConfig: ecsobserver.CommonExporterConfig{
							JobName:      "nginx-prometheus",
							MetricsPath:  "/metrics",
							MetricsPorts: []int{9113},
						},
						NamePattern:          "^nginx-.*",
						ContainerNamePattern: "",
					},
					{
						CommonExporterConfig: ecsobserver.CommonExporterConfig{
							JobName:      "haproxy-prometheus",
							MetricsPath:  "/stats/metrics",
							MetricsPorts: []int{8404},
						},
						NamePattern:          ".*haproxy-service.*",
						ContainerNamePattern: "^haproxy$",
					},
				},
			},
		},
		{
			name: "combined config",
			config: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"ecs_service_discovery": map[string]interface{}{
								"sd_frequency":      "1m30s",
								"sd_target_cluster": "my-ecs-cluster",
								"sd_cluster_region": "us-west-2",
								"sd_result_file":    "/tmp/cwagent_ecs_auto_sd.yaml",
								"docker_label": map[string]interface{}{
									"sd_port_label":         "MY_PROMETHEUS_EXPORTER_PORT_LABEL",
									"sd_metrics_path_label": "MY_PROMETHEUS_METRICS_PATH_LABEL",
									"sd_job_name_label":     "MY_PROMETHEUS_METRICS_NAME_LABEL",
								},
								"task_definition_list": []interface{}{
									map[string]interface{}{
										"sd_metrics_ports":               "9150",
										"sd_task_definition_arn_pattern": "*memcached.*",
									},
								},
							},
						},
					},
				},
			},
			expected: &ecsobserver.Config{
				RefreshInterval: 90 * time.Second,
				ClusterName:     "my-ecs-cluster",
				ClusterRegion:   "us-west-2",
				ResultFile:      "/tmp/cwagent_ecs_auto_sd.yaml",
				DockerLabels: []ecsobserver.DockerLabelConfig{
					{
						JobNameLabel:     "MY_PROMETHEUS_METRICS_NAME_LABEL",
						MetricsPathLabel: "MY_PROMETHEUS_METRICS_PATH_LABEL",
						PortLabel:        "MY_PROMETHEUS_EXPORTER_PORT_LABEL",
					},
				},
				TaskDefinitions: []ecsobserver.TaskDefinitionConfig{
					{
						CommonExporterConfig: ecsobserver.CommonExporterConfig{
							MetricsPath:  defaultMetricsPath,
							MetricsPorts: []int{9150},
						},
						ArnPattern:           "*memcached.*",
						ContainerNamePattern: "",
					},
				},
			},
		},
		{
			name: "missing sd_target_cluster uses ECS util",
			config: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"ecs_service_discovery": map[string]interface{}{
								"sd_frequency":      "1m",
								"sd_cluster_region": "us-west-2",
								"sd_result_file":    "/tmp/test.yaml",
							},
						},
					},
				},
			},
			wantErr: true, // ECS util returns empty in test environment
		},
		{
			name: "missing required fields",
			config: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"ecs_service_discovery": map[string]interface{}{
								"sd_frequency": "1m",
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(tt.config)
			tr := NewTranslator()
			tr.(common.NameSetter).SetName("test")

			got, err := tr.Translate(conf)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestTranslator_ID(t *testing.T) {
	tr := &translator{
		factory: ecsobserver.NewFactory(),
		name:    "test",
	}

	expected := component.NewIDWithName(tr.factory.Type(), tr.name)
	assert.Equal(t, expected, tr.ID())
}

func TestParseMetricsPorts(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single port",
			input:    "9090",
			expected: []string{"9090"},
		},
		{
			name:     "multiple ports",
			input:    "9090;9091;9092",
			expected: []string{"9090", "9091", "9092"},
		},
		{
			name:     "ports with spaces",
			input:    "9090; 9091 ;9092",
			expected: []string{"9090", "9091", "9092"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "only spaces and semicolons",
			input:    " ; ; ",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseMetricsPorts(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}
