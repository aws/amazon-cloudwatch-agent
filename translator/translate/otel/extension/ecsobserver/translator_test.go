// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsobserver

import (
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	eoTranslator := NewTranslator()
	require.EqualValues(t, "ecs_observer", eoTranslator.Type())
	testCases := map[string]struct {
		input   map[string]interface{}
		want    *ecsobserver.Config
		wantErr error
	}{
		"GenerateEcsObserverExtensionConfig": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"net": map[string]interface{}{},
					},
				},
			},
			wantErr: &common.MissingKeyError{Type: eoTranslator.Type(), JsonKey: ecsSdBaseKey},
		},
		"GenerateMetricsTransformProcessorConfigPrometheus": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"cluster_name": "TestCluster",
							"ecs_service_discovery": map[string]interface{}{
								"sd_target_cluster": "TestTargetCluster",
								"sd_cluster_region": "TestRegion",
								"sd_result_file":    "/result/file/path.yaml",
								"sd_frequency":      "30s",
								"docker_label": map[string]interface{}{
									"sd_job_name_label":     "ECS_PROMETHEUS_JOB_NAME_1",
									"sd_metrics_path_label": "ECS_PROMETHEUS_METRICS_PATH",
									"sd_port_label":         "ECS_PROMETHEUS_EXPORTER_PORT_SUBSET",
								},
								"task_definition_list": []interface{}{
									map[string]interface{}{
										"sd_job_name":                    "task_def_1",
										"sd_metrics_path":                "/stats/metrics",
										"sd_metrics_ports":               "9901",
										"sd_task_definition_arn_pattern": ".*task_def_1:[0-9]+",
									},
									map[string]interface{}{
										"sd_container_name_pattern":      "^envoy$",
										"sd_metrics_ports":               "9902",
										"sd_task_definition_arn_pattern": "task_def_2",
									},
								},
								"service_name_list_for_tasks": []interface{}{
									map[string]interface{}{
										"sd_job_name":               "service_name_1",
										"sd_metrics_path":           "/metrics",
										"sd_metrics_ports":          "9113",
										"sd_service_name_pattern":   ".*-application-stack",
										"sd_container_name_pattern": "nginx-prometheus-exporter",
									},
									map[string]interface{}{
										"sd_metrics_path":         "/stats/metrics",
										"sd_metrics_ports":        "9114",
										"sd_service_name_pattern": "run-application-stack",
									},
								},
							},
						},
					},
				},
			},
			want: &ecsobserver.Config{
				ClusterName:     "TestTargetCluster",
				ClusterRegion:   "TestRegion",
				ResultFile:      "/result/file/path.yaml",
				RefreshInterval: time.Duration(30000000000),
				TaskDefinitions: []ecsobserver.TaskDefinitionConfig{
					{
						ContainerNamePattern: "",
						ArnPattern:           ".*task_def_1:[0-9]+",
						CommonExporterConfig: ecsobserver.CommonExporterConfig{
							JobName:      "task_def_1",
							MetricsPorts: []int{9901},
							MetricsPath:  "/stats/metrics",
						},
					},
					{
						ContainerNamePattern: "^envoy$",
						ArnPattern:           "task_def_2",
						CommonExporterConfig: ecsobserver.CommonExporterConfig{
							MetricsPorts: []int{9902},
						},
					},
				},
				Services: []ecsobserver.ServiceConfig{
					{
						CommonExporterConfig: ecsobserver.CommonExporterConfig{
							JobName:      "service_name_1",
							MetricsPorts: []int{9113},
							MetricsPath:  "/metrics",
						},
						NamePattern:          ".*-application-stack",
						ContainerNamePattern: "nginx-prometheus-exporter",
					},
					{
						CommonExporterConfig: ecsobserver.CommonExporterConfig{
							MetricsPorts: []int{9114},
							MetricsPath:  "/stats/metrics",
						},
						NamePattern: "run-application-stack",
					},
				},
				DockerLabels: []ecsobserver.DockerLabelConfig{
					{
						PortLabel:        "ECS_PROMETHEUS_EXPORTER_PORT_SUBSET",
						JobNameLabel:     "ECS_PROMETHEUS_JOB_NAME_1",
						MetricsPathLabel: "ECS_PROMETHEUS_METRICS_PATH",
					},
				},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := eoTranslator.Translate(conf, common.TranslatorOptions{})
			require.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*ecsobserver.Config)
				require.True(t, ok)
				require.Equal(t, testCase.want.ClusterName, gotCfg.ClusterName)
				require.Equal(t, testCase.want.ClusterRegion, gotCfg.ClusterRegion)
				require.Equal(t, testCase.want.ResultFile, gotCfg.ResultFile)
				require.Equal(t, testCase.want.RefreshInterval, gotCfg.RefreshInterval)
				require.Equal(t, testCase.want.TaskDefinitions, gotCfg.TaskDefinitions)
				require.Equal(t, testCase.want.Services, gotCfg.Services)
				require.Equal(t, testCase.want.DockerLabels, gotCfg.DockerLabels)
			}
		})
	}
}
