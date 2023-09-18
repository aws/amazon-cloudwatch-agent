// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsemf

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	legacytranslator "github.com/aws/amazon-cloudwatch-agent/translator"
)

var nilSlice []string
var nilMetricDescriptorsSlice []awsemfexporter.MetricDescriptor

func TestTranslator(t *testing.T) {
	tt := NewTranslator()
	require.EqualValues(t, "awsemf", tt.ID().String())
	testCases := map[string]struct {
		env     map[string]string
		input   map[string]interface{}
		want    map[string]interface{} // Can't construct & use awsemfexporter.Config as it uses internal only types
		wantErr error
	}{
		"GenerateAwsEmfExporterConfigEcs": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"ecs": map[string]interface{}{},
					},
				},
			},
			want: map[string]interface{}{
				"namespace":                              "ECS/ContainerInsights",
				"log_group_name":                         "/aws/ecs/containerinsights/{ClusterName}/performance",
				"log_stream_name":                        "NodeTelemetry-{ContainerInstanceId}",
				"dimension_rollup_option":                "NoDimensionRollup",
				"disable_metric_extraction":              false,
				"enhanced_container_insights":            false,
				"parse_json_encoded_attr_values":         []string{"Sources"},
				"output_destination":                     "cloudwatch",
				"eks_fargate_container_insights_enabled": false,
				"resource_to_telemetry_conversion": resourcetotelemetry.Settings{
					Enabled: true,
				},
				"metric_declarations": []*awsemfexporter.MetricDeclaration{
					{
						Dimensions: [][]string{{"ContainerInstanceId", "InstanceId", "ClusterName"}},
						MetricNameSelectors: []string{"instance_cpu_reserved_capacity", "instance_cpu_utilization",
							"instance_filesystem_utilization", "instance_memory_reserved_capacity",
							"instance_memory_utilization", "instance_network_total_bytes", "instance_number_of_running_tasks"},
					},
					{
						Dimensions: [][]string{{"ClusterName"}},
						MetricNameSelectors: []string{"instance_cpu_limit", "instance_cpu_reserved_capacity",
							"instance_cpu_usage_total", "instance_cpu_utilization", "instance_filesystem_utilization",
							"instance_memory_limit", "instance_memory_reserved_capacity", "instance_memory_utilization",
							"instance_memory_working_set", "instance_network_total_bytes", "instance_number_of_running_tasks"},
					},
				},
				"metric_descriptors": nilMetricDescriptorsSlice,
			},
		},
		"GenerateAwsEmfExporterConfigEcsDisableMetricExtraction": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"ecs": map[string]interface{}{
							"disable_metric_extraction": true,
						},
					},
				},
			},
			want: map[string]interface{}{
				"namespace":                              "ECS/ContainerInsights",
				"log_group_name":                         "/aws/ecs/containerinsights/{ClusterName}/performance",
				"log_stream_name":                        "NodeTelemetry-{ContainerInstanceId}",
				"dimension_rollup_option":                "NoDimensionRollup",
				"disable_metric_extraction":              true,
				"enhanced_container_insights":            false,
				"parse_json_encoded_attr_values":         []string{"Sources"},
				"output_destination":                     "cloudwatch",
				"eks_fargate_container_insights_enabled": false,
				"resource_to_telemetry_conversion": resourcetotelemetry.Settings{
					Enabled: true,
				},
				"metric_declarations": []*awsemfexporter.MetricDeclaration{
					{
						Dimensions: [][]string{{"ContainerInstanceId", "InstanceId", "ClusterName"}},
						MetricNameSelectors: []string{"instance_cpu_reserved_capacity", "instance_cpu_utilization",
							"instance_filesystem_utilization", "instance_memory_reserved_capacity",
							"instance_memory_utilization", "instance_network_total_bytes", "instance_number_of_running_tasks"},
					},
					{
						Dimensions: [][]string{{"ClusterName"}},
						MetricNameSelectors: []string{"instance_cpu_limit", "instance_cpu_reserved_capacity",
							"instance_cpu_usage_total", "instance_cpu_utilization", "instance_filesystem_utilization",
							"instance_memory_limit", "instance_memory_reserved_capacity", "instance_memory_utilization",
							"instance_memory_working_set", "instance_network_total_bytes", "instance_number_of_running_tasks"},
					},
				},
				"metric_descriptors": nilMetricDescriptorsSlice,
			},
		},
		"GenerateAwsEmfExporterConfigKubernetes": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"kubernetes": map[string]interface{}{},
					},
				},
			},
			want: map[string]interface{}{
				"namespace":                              "ContainerInsights",
				"log_group_name":                         "/aws/containerinsights/{ClusterName}/performance",
				"log_stream_name":                        "{NodeName}",
				"dimension_rollup_option":                "NoDimensionRollup",
				"disable_metric_extraction":              false,
				"enhanced_container_insights":            false,
				"parse_json_encoded_attr_values":         []string{"Sources", "kubernetes"},
				"output_destination":                     "cloudwatch",
				"eks_fargate_container_insights_enabled": false,
				"resource_to_telemetry_conversion": resourcetotelemetry.Settings{
					Enabled: true,
				},
				"metric_declarations": []*awsemfexporter.MetricDeclaration{
					{
						Dimensions: [][]string{{"PodName", "Namespace", "ClusterName"}, {"ClusterName"}, {"Service", "Namespace", "ClusterName"}, {"ClusterName", "Namespace"}},
						MetricNameSelectors: []string{"pod_cpu_utilization", "pod_memory_utilization",
							"pod_network_rx_bytes", "pod_network_tx_bytes", "pod_cpu_utilization_over_pod_limit",
							"pod_memory_utilization_over_pod_limit"},
					},
					{
						Dimensions:          [][]string{{"PodName", "Namespace", "ClusterName"}},
						MetricNameSelectors: []string{"pod_number_of_container_restarts"},
					},
					{
						Dimensions:          [][]string{{"PodName", "Namespace", "ClusterName"}, {"ClusterName"}},
						MetricNameSelectors: []string{"pod_cpu_reserved_capacity", "pod_memory_reserved_capacity"},
					},
					{
						Dimensions: [][]string{{"NodeName", "InstanceId", "ClusterName"}, {"ClusterName"}},
						MetricNameSelectors: []string{"node_cpu_utilization", "node_memory_utilization",
							"node_network_total_bytes", "node_cpu_reserved_capacity",
							"node_memory_reserved_capacity", "node_number_of_running_pods", "node_number_of_running_containers"},
					},
					{
						Dimensions:          [][]string{{"ClusterName"}},
						MetricNameSelectors: []string{"node_cpu_usage_total", "node_cpu_limit", "node_memory_working_set", "node_memory_limit"},
					},
					{
						Dimensions:          [][]string{{"NodeName", "InstanceId", "ClusterName"}, {"ClusterName"}},
						MetricNameSelectors: []string{"node_filesystem_utilization"},
					},
					{
						Dimensions:          [][]string{{"Service", "Namespace", "ClusterName"}, {"ClusterName"}},
						MetricNameSelectors: []string{"service_number_of_running_pods"},
					},
					{
						Dimensions:          [][]string{{"Namespace", "ClusterName"}, {"ClusterName"}},
						MetricNameSelectors: []string{"namespace_number_of_running_pods"},
					},
					{
						Dimensions:          [][]string{{"ClusterName"}},
						MetricNameSelectors: []string{"cluster_node_count", "cluster_failed_node_count"},
					},
				},
				"metric_descriptors": nilMetricDescriptorsSlice,
			},
		},
		"GenerateAwsEmfExporterConfigKubernetesDisableMetricExtraction": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"kubernetes": map[string]interface{}{
							"disable_metric_extraction": true,
						},
					},
				},
			},
			want: map[string]interface{}{
				"namespace":                              "ContainerInsights",
				"log_group_name":                         "/aws/containerinsights/{ClusterName}/performance",
				"log_stream_name":                        "{NodeName}",
				"dimension_rollup_option":                "NoDimensionRollup",
				"disable_metric_extraction":              true,
				"enhanced_container_insights":            false,
				"parse_json_encoded_attr_values":         []string{"Sources", "kubernetes"},
				"output_destination":                     "cloudwatch",
				"eks_fargate_container_insights_enabled": false,
				"resource_to_telemetry_conversion": resourcetotelemetry.Settings{
					Enabled: true,
				},
				"metric_declarations": []*awsemfexporter.MetricDeclaration{
					{
						Dimensions: [][]string{{"PodName", "Namespace", "ClusterName"}, {"ClusterName"}, {"Service", "Namespace", "ClusterName"}, {"ClusterName", "Namespace"}},
						MetricNameSelectors: []string{"pod_cpu_utilization", "pod_memory_utilization",
							"pod_network_rx_bytes", "pod_network_tx_bytes", "pod_cpu_utilization_over_pod_limit",
							"pod_memory_utilization_over_pod_limit"},
					},
					{
						Dimensions:          [][]string{{"PodName", "Namespace", "ClusterName"}},
						MetricNameSelectors: []string{"pod_number_of_container_restarts"},
					},
					{
						Dimensions:          [][]string{{"PodName", "Namespace", "ClusterName"}, {"ClusterName"}},
						MetricNameSelectors: []string{"pod_cpu_reserved_capacity", "pod_memory_reserved_capacity"},
					},
					{
						Dimensions: [][]string{{"NodeName", "InstanceId", "ClusterName"}, {"ClusterName"}},
						MetricNameSelectors: []string{"node_cpu_utilization", "node_memory_utilization",
							"node_network_total_bytes", "node_cpu_reserved_capacity",
							"node_memory_reserved_capacity", "node_number_of_running_pods", "node_number_of_running_containers"},
					},
					{
						Dimensions:          [][]string{{"ClusterName"}},
						MetricNameSelectors: []string{"node_cpu_usage_total", "node_cpu_limit", "node_memory_working_set", "node_memory_limit"},
					},
					{
						Dimensions:          [][]string{{"NodeName", "InstanceId", "ClusterName"}, {"ClusterName"}},
						MetricNameSelectors: []string{"node_filesystem_utilization"},
					},
					{
						Dimensions:          [][]string{{"Service", "Namespace", "ClusterName"}, {"ClusterName"}},
						MetricNameSelectors: []string{"service_number_of_running_pods"},
					},
					{
						Dimensions:          [][]string{{"Namespace", "ClusterName"}, {"ClusterName"}},
						MetricNameSelectors: []string{"namespace_number_of_running_pods"},
					},
					{
						Dimensions:          [][]string{{"ClusterName"}},
						MetricNameSelectors: []string{"cluster_node_count", "cluster_failed_node_count"},
					},
				},
				"metric_descriptors": nilMetricDescriptorsSlice,
			},
		},
		"GenerateAwsEmfExporterConfigKubernetesWithEnableFullPodAndContainerMetrics": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"kubernetes": map[string]interface{}{
							"enhanced_container_insights": true,
						},
					},
				},
			},
			want: map[string]interface{}{
				"namespace":                              "ContainerInsights",
				"log_group_name":                         "/aws/containerinsights/{ClusterName}/performance",
				"log_stream_name":                        "{NodeName}",
				"dimension_rollup_option":                "NoDimensionRollup",
				"disable_metric_extraction":              false,
				"enhanced_container_insights":            true,
				"parse_json_encoded_attr_values":         []string{"Sources", "kubernetes"},
				"output_destination":                     "cloudwatch",
				"eks_fargate_container_insights_enabled": false,
				"resource_to_telemetry_conversion": resourcetotelemetry.Settings{
					Enabled: true,
				},
				"metric_declarations": []*awsemfexporter.MetricDeclaration{
					{
						Dimensions: [][]string{{"ClusterName"}, {"ContainerName", "FullPodName", "PodName", "Namespace", "ClusterName"}, {"ContainerName", "PodName", "Namespace", "ClusterName"}},
						MetricNameSelectors: []string{
							"container_cpu_utilization", "container_cpu_utilization_over_container_limit",
							"container_memory_utilization", "container_memory_utilization_over_container_limit", "container_memory_failures_total",
							"container_filesystem_usage", "container_status_running", "container_status_terminated", "container_status_waiting", "container_status_waiting_reason_crashed"},
					},
					{
						Dimensions: [][]string{{"PodName", "Namespace", "ClusterName"}, {"ClusterName"}, {"Service", "Namespace", "ClusterName"}, {"ClusterName", "Namespace"}, {"FullPodName", "PodName", "Namespace", "ClusterName"}},
						MetricNameSelectors: []string{"pod_cpu_utilization", "pod_memory_utilization",
							"pod_network_rx_bytes", "pod_network_tx_bytes", "pod_cpu_utilization_over_pod_limit",
							"pod_memory_utilization_over_pod_limit"},
					},
					{
						Dimensions: [][]string{{"PodName", "Namespace", "ClusterName"}, {"ClusterName"}, {"FullPodName", "PodName", "Namespace", "ClusterName"}, {"Service", "Namespace", "ClusterName"}},
						MetricNameSelectors: []string{"pod_cpu_reserved_capacity", "pod_memory_reserved_capacity", "pod_number_of_container_restarts",
							"pod_number_of_containers", "pod_number_of_running_containers",
							"pod_status_ready", "pod_status_scheduled",
							"pod_status_running", "pod_status_pending",
							"pod_status_failed", "pod_status_unknown",
							"pod_status_succeeded"},
					},
					{
						Dimensions: [][]string{
							{"FullPodName", "PodName", "Namespace", "ClusterName"},
							{"PodName", "Namespace", "ClusterName"},
							{"Service", "Namespace", "ClusterName"},
							{"ClusterName"},
						},
						MetricNameSelectors: []string{"pod_interface_network_rx_dropped", "pod_interface_network_rx_errors", "pod_interface_network_tx_dropped", "pod_interface_network_tx_errors"},
					},
					{
						Dimensions: [][]string{{"NodeName", "InstanceId", "ClusterName"}, {"ClusterName"}},
						MetricNameSelectors: []string{"node_cpu_utilization", "node_memory_utilization", "node_network_total_bytes", "node_cpu_reserved_capacity",
							"node_memory_reserved_capacity", "node_number_of_running_pods", "node_number_of_running_containers",
							"node_cpu_usage_total", "node_cpu_limit", "node_memory_working_set", "node_memory_limit",
							"node_status_condition_ready", "node_status_condition_disk_pressure", "node_status_condition_memory_pressure",
							"node_status_condition_pid_pressure", "node_status_condition_network_unavailable", "node_status_condition_unknown",
							"node_status_capacity_pods", "node_status_allocatable_pods"},
					},
					{
						Dimensions: [][]string{
							{"NodeName", "InstanceId", "ClusterName"},
							{"ClusterName"},
						},
						MetricNameSelectors: []string{
							"node_interface_network_rx_dropped", "node_interface_network_rx_errors",
							"node_interface_network_tx_dropped", "node_interface_network_tx_errors",
							"node_diskio_io_service_bytes_total", "node_diskio_io_serviced_total"},
					},
					{
						Dimensions:          [][]string{{"NodeName", "InstanceId", "ClusterName"}, {"ClusterName"}},
						MetricNameSelectors: []string{"node_filesystem_utilization", "node_filesystem_inodes", "node_filesystem_inodes_free"},
					},
					{
						Dimensions:          [][]string{{"Service", "Namespace", "ClusterName"}, {"ClusterName"}},
						MetricNameSelectors: []string{"service_number_of_running_pods"},
					},
					{
						Dimensions:          [][]string{{"PodName", "Namespace", "ClusterName"}, {"ClusterName"}},
						MetricNameSelectors: []string{"replicas_desired", "replicas_ready", "status_replicas_available", "status_replicas_unavailable"},
					},
					{
						Dimensions:          [][]string{{"PodName", "Namespace", "ClusterName"}, {"ClusterName"}},
						MetricNameSelectors: []string{"daemonset_status_number_available", "daemonset_status_number_unavailable"},
					},
					{
						Dimensions:          [][]string{{"Namespace", "ClusterName"}, {"ClusterName"}},
						MetricNameSelectors: []string{"namespace_number_of_running_pods"},
					},
					{
						Dimensions:          [][]string{{"ClusterName"}},
						MetricNameSelectors: []string{"cluster_node_count", "cluster_failed_node_count", "cluster_number_of_running_pods"},
					},
					{
						Dimensions:          [][]string{{"ClusterName", "endpoint"}, {"ClusterName"}},
						MetricNameSelectors: []string{"etcd_db_total_size_in_bytes"},
					},
					{
						Dimensions:          [][]string{{"ClusterName", "resource"}, {"ClusterName"}},
						MetricNameSelectors: []string{"apiserver_storage_list_duration_seconds"},
					},
					{
						Dimensions:          [][]string{{"ClusterName", "priority_level"}, {"ClusterName"}},
						MetricNameSelectors: []string{"apiserver_flowcontrol_request_concurrency_limit"},
					},
					{
						Dimensions: [][]string{{"ClusterName"}},
						MetricNameSelectors: []string{
							"apiserver_admission_controller_admission_duration_seconds",
							"apiserver_flowcontrol_rejected_requests_total",
							"apiserver_request_duration_seconds",
							"apiserver_request_total",
							"apiserver_request_total_5xx",
							"apiserver_storage_objects",
							"etcd_request_duration_seconds",
							"rest_client_request_duration_seconds",
							"rest_client_requests_total",
						},
					},
				},
				"metric_descriptors": []awsemfexporter.MetricDescriptor{
					{
						MetricName: "apiserver_admission_controller_admission_duration_seconds",
						Unit:       "Seconds",
						Overwrite:  true,
					},
					{
						MetricName: "apiserver_flowcontrol_request_concurrency_limit",
						Unit:       "Count",
						Overwrite:  true,
					},
					{
						MetricName: "apiserver_flowcontrol_rejected_requests_total",
						Unit:       "Count",
						Overwrite:  true,
					},
					{
						MetricName: "apiserver_request_duration_seconds",
						Unit:       "Seconds",
						Overwrite:  true,
					},
					{
						MetricName: "apiserver_request_total",
						Unit:       "Count",
						Overwrite:  true,
					},
					{
						MetricName: "apiserver_request_total_5xx",
						Unit:       "Count",
						Overwrite:  true,
					},
					{
						MetricName: "apiserver_storage_objects",
						Unit:       "Count",
						Overwrite:  true,
					},
					{
						MetricName: "etcd_request_duration_seconds",
						Unit:       "Seconds",
						Overwrite:  true,
					},
					{
						MetricName: "rest_client_request_duration_seconds",
						Unit:       "Seconds",
						Overwrite:  true,
					},
					{
						MetricName: "rest_client_requests_total",
						Unit:       "Count",
						Overwrite:  true,
					},
				},
			},
		},
		"GenerateAwsEmfExporterConfigPrometheus": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"log_group_name":  "/test/log/group",
							"log_stream_name": "{LogStreamName}",
							"emf_processor": map[string]interface{}{
								"metric_declaration": []interface{}{
									map[string]interface{}{
										"source_labels":    []string{"Service", "Namespace"},
										"label_matcher":    "(.*node-exporter.*|.*kube-dns.*);kube-system$",
										"dimensions":       [][]string{{"Service", "Namespace"}},
										"metric_selectors": []string{"^coredns_dns_request_type_count_total$"},
									},
								},
								"metric_unit": map[string]interface{}{
									"jvm_gc_collection_seconds_sum": "Milliseconds",
								},
							},
						},
					},
				},
			},
			want: map[string]interface{}{
				"namespace":                              "CWAgent/Prometheus",
				"log_group_name":                         "/test/log/group",
				"log_stream_name":                        "{JobName}",
				"dimension_rollup_option":                "NoDimensionRollup",
				"disable_metric_extraction":              false,
				"enhanced_container_insights":            false,
				"parse_json_encoded_attr_values":         nilSlice,
				"output_destination":                     "cloudwatch",
				"eks_fargate_container_insights_enabled": false,
				"resource_to_telemetry_conversion": resourcetotelemetry.Settings{
					Enabled: true,
				},
				"metric_declarations": []*awsemfexporter.MetricDeclaration{
					{
						Dimensions:          [][]string{{"Service", "Namespace"}},
						MetricNameSelectors: []string{"^coredns_dns_request_type_count_total$"},
						LabelMatchers: []*awsemfexporter.LabelMatcher{
							{
								LabelNames: []string{"Service", "Namespace"},
								Regex:      "(.*node-exporter.*|.*kube-dns.*);kube-system$",
							},
						},
					},
				},
				"metric_descriptors": []awsemfexporter.MetricDescriptor{
					{
						MetricName: "jvm_gc_collection_seconds_sum",
						Unit:       "Milliseconds",
					},
				},
			},
		},
		"GenerateAwsEmfExporterConfigPrometheusDisableMetricExtraction": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"disable_metric_extraction": true,
							"log_group_name":            "/test/log/group",
							"log_stream_name":           "{JobName}",
						},
					},
				},
			},
			want: map[string]interface{}{
				"namespace":                              "",
				"log_group_name":                         "/test/log/group",
				"log_stream_name":                        "{JobName}",
				"dimension_rollup_option":                "NoDimensionRollup",
				"disable_metric_extraction":              true,
				"enhanced_container_insights":            false,
				"parse_json_encoded_attr_values":         nilSlice,
				"output_destination":                     "cloudwatch",
				"eks_fargate_container_insights_enabled": false,
				"resource_to_telemetry_conversion": resourcetotelemetry.Settings{
					Enabled: true,
				},
				"metric_declarations": []*awsemfexporter.MetricDeclaration{
					{
						MetricNameSelectors: []string{"$^"},
					},
				},
				"metric_descriptors": nilMetricDescriptorsSlice,
			},
		},
		"GenerateAwsEmfExporterConfigPrometheusNoDeclarations": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"log_group_name":  "/test/log/group",
							"log_stream_name": "{JobName}",
							"emf_processor": map[string]interface{}{
								"metric_unit": map[string]interface{}{
									"jvm_gc_collection_seconds_sum": "Milliseconds",
								},
							},
						},
					},
				},
			},
			want: map[string]interface{}{
				"namespace":                              "CWAgent/Prometheus",
				"log_group_name":                         "/test/log/group",
				"log_stream_name":                        "{JobName}",
				"dimension_rollup_option":                "NoDimensionRollup",
				"disable_metric_extraction":              false,
				"enhanced_container_insights":            false,
				"parse_json_encoded_attr_values":         nilSlice,
				"output_destination":                     "cloudwatch",
				"eks_fargate_container_insights_enabled": false,
				"resource_to_telemetry_conversion": resourcetotelemetry.Settings{
					Enabled: true,
				},
				"metric_declarations": []*awsemfexporter.MetricDeclaration{
					{
						MetricNameSelectors: []string{"$^"},
					},
				},
				"metric_descriptors": []awsemfexporter.MetricDescriptor{
					{
						MetricName: "jvm_gc_collection_seconds_sum",
						Unit:       "Milliseconds",
					},
				},
			},
		},
		"GenerateAwsEmfExporterConfigPrometheusNoEmfProcessor": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"log_group_name":  "/test/log/group",
							"log_stream_name": "{JobName}",
						},
					},
				},
			},
			want: map[string]interface{}{
				"namespace":                              "",
				"log_group_name":                         "/test/log/group",
				"log_stream_name":                        "{JobName}",
				"dimension_rollup_option":                "NoDimensionRollup",
				"disable_metric_extraction":              false,
				"enhanced_container_insights":            false,
				"parse_json_encoded_attr_values":         nilSlice,
				"output_destination":                     "cloudwatch",
				"eks_fargate_container_insights_enabled": false,
				"resource_to_telemetry_conversion": resourcetotelemetry.Settings{
					Enabled: true,
				},
				"metric_declarations": []*awsemfexporter.MetricDeclaration{
					{
						MetricNameSelectors: []string{"$^"},
					},
				},
				"metric_descriptors": nilMetricDescriptorsSlice,
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			require.Equal(t, testCase.wantErr, err)
			require.Truef(t, legacytranslator.IsTranslateSuccess(), "Error in legacy translation rules: %v", legacytranslator.ErrorMessages)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*awsemfexporter.Config)
				require.True(t, ok)
				require.Equal(t, testCase.want["namespace"], gotCfg.Namespace)
				require.Equal(t, testCase.want["log_group_name"], gotCfg.LogGroupName)
				require.Equal(t, testCase.want["log_stream_name"], gotCfg.LogStreamName)
				require.Equal(t, testCase.want["dimension_rollup_option"], gotCfg.DimensionRollupOption)
				require.Equal(t, testCase.want["disable_metric_extraction"], gotCfg.DisableMetricExtraction)
				require.Equal(t, testCase.want["enhanced_container_insights"], gotCfg.EnhancedContainerInsights)
				require.Equal(t, testCase.want["parse_json_encoded_attr_values"], gotCfg.ParseJSONEncodedAttributeValues)
				require.Equal(t, testCase.want["output_destination"], gotCfg.OutputDestination)
				require.Equal(t, testCase.want["eks_fargate_container_insights_enabled"], gotCfg.EKSFargateContainerInsightsEnabled)
				require.Equal(t, testCase.want["resource_to_telemetry_conversion"], gotCfg.ResourceToTelemetrySettings)
				require.ElementsMatch(t, testCase.want["metric_declarations"], gotCfg.MetricDeclarations)
				require.ElementsMatch(t, testCase.want["metric_descriptors"], gotCfg.MetricDescriptors)
			}
		})
	}
}
