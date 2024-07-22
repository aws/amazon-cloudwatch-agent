// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsemf

import (
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/testutil"
	legacytranslator "github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

var nilSlice []string
var nilMetricDescriptorsSlice []awsemfexporter.MetricDescriptor

func TestTranslator(t *testing.T) {
	t.Setenv(envconfig.AWS_CA_BUNDLE, "/ca/bundle")
	agent.Global_Config.Region = "us-east-1"
	agent.Global_Config.Role_arn = "global_arn"
	tt := NewTranslator()
	require.EqualValues(t, "awsemf", tt.ID().String())
	testCases := map[string]struct {
		env     map[string]string
		input   map[string]any
		want    map[string]any // Can't construct & use awsemfexporter.Config as it uses internal only types
		wantErr error
	}{
		"GenerateAwsEmfExporterConfigEcs": {
			input: map[string]any{
				"logs": map[string]any{
					"metrics_collected": map[string]any{
						"ecs": map[string]any{},
					},
				},
			},
			want: map[string]any{
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
				"local_mode":         false,
			},
		},
		"GenerateAwsEmfExporterConfigEcsDisableMetricExtraction": {
			input: map[string]any{
				"logs": map[string]any{
					"metrics_collected": map[string]any{
						"ecs": map[string]any{
							"disable_metric_extraction": true,
						},
					},
				},
			},
			want: map[string]any{
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
				"local_mode":         false,
			},
		},
		"GenerateAwsEmfExporterConfigKubernetes": {
			input: map[string]any{
				"logs": map[string]any{
					"metrics_collected": map[string]any{
						"kubernetes": map[string]any{},
					},
				},
			},
			want: map[string]any{
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
				"local_mode":         false,
			},
		},
		"GenerateAwsEmfExporterConfigKubernetesDisableMetricExtraction": {
			input: map[string]any{
				"logs": map[string]any{
					"metrics_collected": map[string]any{
						"kubernetes": map[string]any{
							"disable_metric_extraction": true,
						},
					},
				},
			},
			want: map[string]any{
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
				"local_mode":         false,
			},
		},
		"GenerateAwsEmfExporterConfigKubernetesWithEnableFullPodAndContainerMetrics": {
			input: map[string]any{
				"logs": map[string]any{
					"metrics_collected": map[string]any{
						"kubernetes": map[string]any{
							"enhanced_container_insights": true,
						},
					},
				},
			},
			want: map[string]any{
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
							"container_cpu_utilization", "container_cpu_utilization_over_container_limit", "container_cpu_limit", "container_cpu_request",
							"container_memory_utilization", "container_memory_utilization_over_container_limit", "container_memory_failures_total", "container_memory_limit", "container_memory_request",
							"container_filesystem_usage", "container_filesystem_available", "container_filesystem_utilization",
						},
					},
					{
						Dimensions: [][]string{{"PodName", "Namespace", "ClusterName"}, {"ClusterName"}, {"Service", "Namespace", "ClusterName"}, {"ClusterName", "Namespace"}, {"FullPodName", "PodName", "Namespace", "ClusterName"}},
						MetricNameSelectors: []string{"pod_cpu_utilization", "pod_memory_utilization",
							"pod_network_rx_bytes", "pod_network_tx_bytes", "pod_cpu_utilization_over_pod_limit",
							"pod_memory_utilization_over_pod_limit"},
					},
					{
						Dimensions: [][]string{
							{"FullPodName", "PodName", "Namespace", "ClusterName"},
							{"PodName", "Namespace", "ClusterName"},
							{"Namespace", "ClusterName"},
							{"ClusterName"},
						},
						MetricNameSelectors: []string{"pod_interface_network_rx_dropped", "pod_interface_network_tx_dropped"},
					},
					{
						Dimensions: [][]string{{"PodName", "Namespace", "ClusterName"}, {"ClusterName"}, {"FullPodName", "PodName", "Namespace", "ClusterName"}, {"Service", "Namespace", "ClusterName"}},
						MetricNameSelectors: []string{"pod_cpu_reserved_capacity", "pod_memory_reserved_capacity", "pod_number_of_container_restarts", "pod_number_of_containers", "pod_number_of_running_containers",
							"pod_status_ready", "pod_status_scheduled", "pod_status_running", "pod_status_pending", "pod_status_failed", "pod_status_unknown",
							"pod_status_succeeded", "pod_memory_request", "pod_memory_limit", "pod_cpu_limit", "pod_cpu_request",
							"pod_container_status_running", "pod_container_status_terminated", "pod_container_status_waiting", "pod_container_status_waiting_reason_crash_loop_back_off",
							"pod_container_status_waiting_reason_image_pull_error", "pod_container_status_waiting_reason_start_error", "pod_container_status_waiting_reason_create_container_error",
							"pod_container_status_waiting_reason_create_container_config_error", "pod_container_status_terminated_reason_oom_killed",
						},
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
							"node_interface_network_rx_dropped", "node_interface_network_tx_dropped",
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
						MetricNameSelectors: []string{"apiserver_storage_size_bytes", "apiserver_storage_db_total_size_in_bytes", "etcd_db_total_size_in_bytes"},
					},
					{
						Dimensions:          [][]string{{"ClusterName", "resource"}, {"ClusterName"}},
						MetricNameSelectors: []string{"apiserver_storage_list_duration_seconds", "apiserver_longrunning_requests", "apiserver_storage_objects"},
					},
					{
						Dimensions:          [][]string{{"ClusterName", "verb"}, {"ClusterName"}},
						MetricNameSelectors: []string{"apiserver_request_duration_seconds", "rest_client_request_duration_seconds"},
					},
					{
						Dimensions:          [][]string{{"ClusterName", "code", "verb"}, {"ClusterName"}},
						MetricNameSelectors: []string{"apiserver_request_total", "apiserver_request_total_5xx"},
					},
					{
						Dimensions:          [][]string{{"ClusterName", "operation"}, {"ClusterName"}},
						MetricNameSelectors: []string{"apiserver_admission_controller_admission_duration_seconds", "apiserver_admission_step_admission_duration_seconds", "etcd_request_duration_seconds"},
					},
					{
						Dimensions:          [][]string{{"ClusterName", "code", "method"}, {"ClusterName"}},
						MetricNameSelectors: []string{"rest_client_requests_total"},
					},
					{
						Dimensions:          [][]string{{"ClusterName", "request_kind"}, {"ClusterName"}},
						MetricNameSelectors: []string{"apiserver_current_inflight_requests", "apiserver_current_inqueue_requests"},
					},
					{
						Dimensions:          [][]string{{"ClusterName", "name"}, {"ClusterName"}},
						MetricNameSelectors: []string{"apiserver_admission_webhook_admission_duration_seconds"},
					},
					{
						Dimensions:          [][]string{{"ClusterName", "group"}, {"ClusterName"}},
						MetricNameSelectors: []string{"apiserver_requested_deprecated_apis"},
					},
					{
						Dimensions:          [][]string{{"ClusterName", "reason"}, {"ClusterName"}},
						MetricNameSelectors: []string{"apiserver_flowcontrol_rejected_requests_total"},
					},
					{
						Dimensions:          [][]string{{"ClusterName", "priority_level"}, {"ClusterName"}},
						MetricNameSelectors: []string{"apiserver_flowcontrol_request_concurrency_limit"},
					},
					{
						Dimensions: [][]string{{"ClusterName"}, {"ClusterName", "Namespace", "PodName", "ContainerName"}, {"ClusterName", "Namespace", "PodName", "FullPodName", "ContainerName"}, {"ClusterName", "Namespace", "PodName", "FullPodName", "ContainerName", "GpuDevice"}},
						MetricNameSelectors: []string{
							"container_gpu_utilization", "container_gpu_memory_utilization", "container_gpu_memory_total", "container_gpu_memory_used", "container_gpu_power_draw", "container_gpu_temperature",
						},
					},
					{
						Dimensions: [][]string{{"ClusterName"}, {"ClusterName", "Namespace"}, {"ClusterName", "Namespace", "Service"}, {"ClusterName", "Namespace", "PodName"}, {"ClusterName", "Namespace", "PodName", "FullPodName"}, {"ClusterName", "Namespace", "PodName", "FullPodName", "GpuDevice"}},
						MetricNameSelectors: []string{
							"pod_gpu_utilization", "pod_gpu_memory_utilization", "pod_gpu_memory_total", "pod_gpu_memory_used", "pod_gpu_power_draw", "pod_gpu_temperature",
						},
					},
					{
						Dimensions: [][]string{{"ClusterName"}, {"ClusterName", "NodeName", "InstanceId"}, {"ClusterName", "NodeName", "InstanceId", "InstanceType", "GpuDevice"}},
						MetricNameSelectors: []string{
							"node_gpu_utilization", "node_gpu_memory_utilization", "node_gpu_memory_total", "node_gpu_memory_used", "node_gpu_power_draw", "node_gpu_temperature",
						},
					},
					{
						Dimensions: [][]string{{"ClusterName"}, {"ClusterName", "Namespace"}, {"ClusterName", "Namespace", "Service"}, {"ClusterName", "Namespace", "PodName"}, {"ClusterName", "Namespace", "PodName", "FullPodName"}},
						MetricNameSelectors: []string{
							"pod_gpu_total", "pod_gpu_request", "pod_gpu_limit",
						},
					},
					{
						Dimensions: [][]string{{"ClusterName"}, {"ClusterName", "NodeName", "InstanceId", "InstanceType"}},
						MetricNameSelectors: []string{
							"node_gpu_total", "node_gpu_request", "node_gpu_limit",
						},
					},
					{
						Dimensions: [][]string{{"ClusterName"}},
						MetricNameSelectors: []string{
							"cluster_gpu_total", "cluster_gpu_request",
						},
					},
					{
						Dimensions: [][]string{{"ClusterName"}, {"ClusterName", "Namespace", "PodName", "ContainerName"}, {"ClusterName", "Namespace", "PodName", "FullPodName", "ContainerName"}, {"ClusterName", "Namespace", "PodName", "FullPodName", "ContainerName", "NeuronDevice", "NeuronCore"}},
						MetricNameSelectors: []string{
							"container_neuroncore_utilization",
							"container_neuroncore_memory_usage_total",
							"container_neuroncore_memory_usage_constants",
							"container_neuroncore_memory_usage_model_code",
							"container_neuroncore_memory_usage_model_shared_scratchpad",
							"container_neuroncore_memory_usage_runtime_memory",
							"container_neuroncore_memory_usage_tensors",
						},
					},
					{
						Dimensions: [][]string{{"ClusterName"}, {"ClusterName", "Namespace", "PodName", "ContainerName"}, {"ClusterName", "Namespace", "PodName", "FullPodName", "ContainerName"}, {"ClusterName", "Namespace", "PodName", "FullPodName", "ContainerName", "NeuronDevice"}},
						MetricNameSelectors: []string{
							"container_neurondevice_hw_ecc_events_total",
						},
					},
					{
						Dimensions: [][]string{{"ClusterName"}, {"ClusterName", "Namespace"}, {"ClusterName", "Namespace", "Service"}, {"ClusterName", "Namespace", "PodName"}, {"ClusterName", "Namespace", "PodName", "FullPodName"}, {"ClusterName", "Namespace", "PodName", "FullPodName", "NeuronDevice", "NeuronCore"}},
						MetricNameSelectors: []string{
							"pod_neuroncore_utilization",
							"pod_neuroncore_memory_usage_total",
							"pod_neuroncore_memory_usage_constants",
							"pod_neuroncore_memory_usage_model_code",
							"pod_neuroncore_memory_usage_model_shared_scratchpad",
							"pod_neuroncore_memory_usage_runtime_memory",
							"pod_neuroncore_memory_usage_tensors",
						},
					},
					{
						Dimensions: [][]string{{"ClusterName"}, {"ClusterName", "Namespace"}, {"ClusterName", "Namespace", "Service"}, {"ClusterName", "Namespace", "PodName"}, {"ClusterName", "Namespace", "PodName", "FullPodName"}, {"ClusterName", "Namespace", "PodName", "FullPodName", "NeuronDevice"}},
						MetricNameSelectors: []string{
							"pod_neurondevice_hw_ecc_events_total",
						},
					},
					{
						Dimensions: [][]string{{"ClusterName"}, {"ClusterName", "InstanceId", "NodeName"}, {"ClusterName", "InstanceType", "InstanceId", "NodeName", "NeuronDevice", "NeuronCore"}},
						MetricNameSelectors: []string{
							"node_neuroncore_utilization",
							"node_neuroncore_memory_usage_total",
							"node_neuroncore_memory_usage_constants",
							"node_neuroncore_memory_usage_model_code",
							"node_neuroncore_memory_usage_model_shared_scratchpad",
							"node_neuroncore_memory_usage_runtime_memory",
							"node_neuroncore_memory_usage_tensors",
						},
					},
					{
						Dimensions: [][]string{{"ClusterName"}, {"ClusterName", "InstanceId", "NodeName"}},
						MetricNameSelectors: []string{
							"node_neuron_execution_errors_total",
							"node_neurondevice_runtime_memory_used_bytes",
							"node_neuron_execution_latency",
						},
					},
					{
						Dimensions: [][]string{{"ClusterName"}, {"ClusterName", "InstanceId", "NodeName"}, {"ClusterName", "InstanceId", "NodeName", "NeuronDevice"}},
						MetricNameSelectors: []string{
							"node_neurondevice_hw_ecc_events_total",
						},
					},
					{
						Dimensions: [][]string{{"ClusterName"}, {"ClusterName", "Namespace", "PodName", "ContainerName"}, {"ClusterName", "Namespace", "PodName", "FullPodName", "ContainerName"}},
						MetricNameSelectors: []string{
							"container_efa_rx_bytes", "container_efa_tx_bytes", "container_efa_rx_dropped", "container_efa_rdma_read_bytes", "container_efa_rdma_write_bytes", "container_efa_rdma_write_recv_bytes",
						},
					},
					{
						Dimensions: [][]string{{"ClusterName"}, {"ClusterName", "Namespace"}, {"ClusterName", "Namespace", "Service"}, {"ClusterName", "Namespace", "PodName"}, {"ClusterName", "Namespace", "PodName", "FullPodName"}},
						MetricNameSelectors: []string{
							"pod_efa_rx_bytes", "pod_efa_tx_bytes", "pod_efa_rx_dropped", "pod_efa_rdma_read_bytes", "pod_efa_rdma_write_bytes", "pod_efa_rdma_write_recv_bytes",
						},
					},
					{
						Dimensions: [][]string{{"ClusterName"}, {"ClusterName", "NodeName", "InstanceId"}},
						MetricNameSelectors: []string{
							"node_efa_rx_bytes", "node_efa_tx_bytes", "node_efa_rx_dropped", "node_efa_rdma_read_bytes", "node_efa_rdma_write_bytes", "node_efa_rdma_write_recv_bytes",
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
						MetricName: "apiserver_admission_step_admission_duration_seconds",
						Unit:       "Seconds",
						Overwrite:  true,
					},
					{
						MetricName: "apiserver_admission_webhook_admission_duration_seconds",
						Unit:       "Seconds",
						Overwrite:  true,
					},
					{
						MetricName: "apiserver_current_inflight_requests",
						Unit:       "Count",
						Overwrite:  true,
					},
					{
						MetricName: "apiserver_current_inqueue_requests",
						Unit:       "Count",
						Overwrite:  true,
					},
					{
						MetricName: "apiserver_flowcontrol_rejected_requests_total",
						Unit:       "Count",
						Overwrite:  true,
					},
					{
						MetricName: "apiserver_flowcontrol_request_concurrency_limit",
						Unit:       "Count",
						Overwrite:  true,
					},
					{
						MetricName: "apiserver_longrunning_requests",
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
						MetricName: "apiserver_requested_deprecated_apis",
						Unit:       "Count",
						Overwrite:  true,
					},
					{
						MetricName: "apiserver_storage_objects",
						Unit:       "Count",
						Overwrite:  true,
					},
					{
						MetricName: "apiserver_storage_list_duration_seconds",
						Unit:       "Seconds",
						Overwrite:  true,
					},
					{
						MetricName: "apiserver_storage_db_total_size_in_bytes",
						Unit:       "Bytes",
						Overwrite:  true,
					},
					{
						MetricName: "apiserver_storage_size_bytes",
						Unit:       "Bytes",
						Overwrite:  true,
					},
					{
						MetricName: "etcd_db_total_size_in_bytes",
						Unit:       "Bytes",
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
				"local_mode": false,
			},
		},
		"GenerateAwsEmfExporterConfigPrometheus": {
			input: map[string]any{
				"logs": map[string]any{
					"metrics_collected": map[string]any{
						"prometheus": map[string]any{
							"log_group_name":  "/test/log/group",
							"log_stream_name": "{LogStreamName}",
							"emf_processor": map[string]any{
								"metric_declaration": []any{
									map[string]any{
										"source_labels":    []string{"Service", "Namespace"},
										"label_matcher":    "(.*node-exporter.*|.*kube-dns.*);kube-system$",
										"dimensions":       [][]string{{"Service", "Namespace"}},
										"metric_selectors": []string{"^coredns_dns_request_type_count_total$"},
									},
								},
								"metric_unit": map[string]any{
									"jvm_gc_collection_seconds_sum": "Milliseconds",
								},
							},
						},
					},
				},
			},
			want: map[string]any{
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
				"local_mode": false,
			},
		},
		"GenerateAwsEmfExporterConfigPrometheusDisableMetricExtraction": {
			input: map[string]any{
				"logs": map[string]any{
					"metrics_collected": map[string]any{
						"prometheus": map[string]any{
							"disable_metric_extraction": true,
							"log_group_name":            "/test/log/group",
							"log_stream_name":           "{JobName}",
						},
					},
				},
			},
			want: map[string]any{
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
				"local_mode":         false,
			},
		},
		"GenerateAwsEmfExporterConfigPrometheusNoDeclarations": {
			input: map[string]any{
				"logs": map[string]any{
					"metrics_collected": map[string]any{
						"prometheus": map[string]any{
							"log_group_name":  "/test/log/group",
							"log_stream_name": "{JobName}",
							"emf_processor": map[string]any{
								"metric_unit": map[string]any{
									"jvm_gc_collection_seconds_sum": "Milliseconds",
								},
							},
						},
					},
				},
			},
			want: map[string]any{
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
				"local_mode": false,
			},
		},
		"GenerateAwsEmfExporterConfigPrometheusNoEmfProcessor": {
			input: map[string]any{
				"logs": map[string]any{
					"metrics_collected": map[string]any{
						"prometheus": map[string]any{
							"log_group_name":  "/test/log/group",
							"log_stream_name": "{JobName}",
						},
					},
				},
			},
			want: map[string]any{
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
				"local_mode":         false,
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
				assert.Equal(t, testCase.want["namespace"], gotCfg.Namespace)
				assert.Equal(t, testCase.want["log_group_name"], gotCfg.LogGroupName)
				assert.Equal(t, testCase.want["log_stream_name"], gotCfg.LogStreamName)
				assert.Equal(t, testCase.want["dimension_rollup_option"], gotCfg.DimensionRollupOption)
				assert.Equal(t, testCase.want["disable_metric_extraction"], gotCfg.DisableMetricExtraction)
				assert.Equal(t, testCase.want["enhanced_container_insights"], gotCfg.EnhancedContainerInsights)
				assert.Equal(t, testCase.want["parse_json_encoded_attr_values"], gotCfg.ParseJSONEncodedAttributeValues)
				assert.Equal(t, testCase.want["output_destination"], gotCfg.OutputDestination)
				assert.Equal(t, testCase.want["eks_fargate_container_insights_enabled"], gotCfg.EKSFargateContainerInsightsEnabled)
				assert.Equal(t, testCase.want["resource_to_telemetry_conversion"], gotCfg.ResourceToTelemetrySettings)
				assert.ElementsMatch(t, testCase.want["metric_declarations"], gotCfg.MetricDeclarations)
				assert.ElementsMatch(t, testCase.want["metric_descriptors"], gotCfg.MetricDescriptors)
				assert.Equal(t, testCase.want["local_mode"], gotCfg.LocalMode)
				assert.Equal(t, "/ca/bundle", gotCfg.CertificateFilePath)
				assert.Equal(t, "global_arn", gotCfg.RoleARN)
				assert.Equal(t, "us-east-1", gotCfg.Region)
				assert.NotNil(t, gotCfg.MiddlewareID)
				assert.Equal(t, "agenthealth/logs", gotCfg.MiddlewareID.String())
			}
		})
	}
}

func TestTranslateAppSignals(t *testing.T) {
	t.Setenv(envconfig.AWS_CA_BUNDLE, "/ca/bundle")
	agent.Global_Config.Region = "us-east-1"
	agent.Global_Config.Role_arn = "global_arn"
	t.Setenv(envconfig.IMDS_NUMBER_RETRY, "0")
	tt := NewTranslatorWithName(common.AppSignals)
	testCases := map[string]struct {
		input          map[string]any
		want           *confmap.Conf
		wantErr        error
		kubernetesMode string
		mode           string
	}{
		"WithAppSignalsEnabledEKS": {
			input: map[string]any{
				"logs": map[string]any{
					"metrics_collected": map[string]any{
						"application_signals": map[string]any{},
					},
				}},
			want: testutil.GetConfWithOverrides(t, filepath.Join("appsignals_config_eks.yaml"), map[string]any{
				"local_mode":            "false",
				"region":                "us-east-1",
				"role_arn":              "global_arn",
				"certificate_file_path": "/ca/bundle",
			}),
			kubernetesMode: config.ModeEKS,
			mode:           config.ModeEC2,
		},
		"WithAppSignalsEnabledK8s": {
			input: map[string]any{
				"logs": map[string]any{
					"metrics_collected": map[string]any{
						"application_signals": map[string]any{},
					},
				}},
			want: testutil.GetConfWithOverrides(t, filepath.Join("appsignals_config_k8s.yaml"), map[string]any{
				"local_mode":            "true",
				"region":                "us-east-1",
				"role_arn":              "global_arn",
				"certificate_file_path": "/ca/bundle",
			}),
			kubernetesMode: config.ModeK8sOnPrem,
			mode:           config.ModeOnPrem,
		},
		"WithAppSignalsEnabledGeneric": {
			input: map[string]any{
				"logs": map[string]any{
					"metrics_collected": map[string]any{
						"application_signals": map[string]any{},
					},
				}},
			want: testutil.GetConfWithOverrides(t, filepath.Join("appsignals_config_generic.yaml"), map[string]any{
				"local_mode":            "true",
				"region":                "us-east-1",
				"role_arn":              "global_arn",
				"certificate_file_path": "/ca/bundle",
			}),
			kubernetesMode: "",
			mode:           config.ModeOnPrem,
		},
		"WithAppSignalsEnabledEC2": {
			input: map[string]any{
				"logs": map[string]any{
					"metrics_collected": map[string]any{
						"application_signals": map[string]any{},
					},
				}},
			want: testutil.GetConfWithOverrides(t, filepath.Join("appsignals_config_generic.yaml"), map[string]any{
				"local_mode":            "false",
				"region":                "us-east-1",
				"role_arn":              "global_arn",
				"certificate_file_path": "/ca/bundle",
			}),
			kubernetesMode: "",
			mode:           config.ModeEC2,
		},
		"WithAppSignalsFallbackEnabledEKS": {
			input: map[string]any{
				"logs": map[string]any{
					"metrics_collected": map[string]any{
						"app_signals": map[string]any{},
					},
				}},
			want: testutil.GetConfWithOverrides(t, filepath.Join("appsignals_config_eks.yaml"), map[string]any{
				"local_mode":            "false",
				"region":                "us-east-1",
				"role_arn":              "global_arn",
				"certificate_file_path": "/ca/bundle",
			}),
			kubernetesMode: config.ModeEKS,
			mode:           config.ModeEC2,
		},
		"WithAppSignalsFallbackEnabledK8s": {
			input: map[string]any{
				"logs": map[string]any{
					"metrics_collected": map[string]any{
						"app_signals": map[string]any{},
					},
				}},
			want: testutil.GetConfWithOverrides(t, filepath.Join("appsignals_config_k8s.yaml"), map[string]any{
				"local_mode":            "true",
				"region":                "us-east-1",
				"role_arn":              "global_arn",
				"certificate_file_path": "/ca/bundle",
			}),
			kubernetesMode: config.ModeK8sOnPrem,
			mode:           config.ModeOnPrem,
		},
		"WithAppSignalsFallbackEnabledGeneric": {
			input: map[string]any{
				"logs": map[string]any{
					"metrics_collected": map[string]any{
						"app_signals": map[string]any{},
					},
				}},
			want: testutil.GetConfWithOverrides(t, filepath.Join("appsignals_config_generic.yaml"), map[string]any{
				"local_mode":            "true",
				"region":                "us-east-1",
				"role_arn":              "global_arn",
				"certificate_file_path": "/ca/bundle",
			}),
			kubernetesMode: "",
			mode:           config.ModeOnPrem,
		},
		"WithAppSignalsFallbackEnabledEC2": {
			input: map[string]any{
				"logs": map[string]any{
					"metrics_collected": map[string]any{
						"app_signals": map[string]any{},
					},
				}},
			want: testutil.GetConfWithOverrides(t, filepath.Join("appsignals_config_generic.yaml"), map[string]any{
				"local_mode":            "false",
				"region":                "us-east-1",
				"role_arn":              "global_arn",
				"certificate_file_path": "/ca/bundle",
			}),
			kubernetesMode: "",
			mode:           config.ModeEC2,
		},
	}
	factory := awsemfexporter.NewFactory()
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			context.CurrentContext().SetKubernetesMode(testCase.kubernetesMode)
			context.CurrentContext().SetMode(testCase.mode)
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*awsemfexporter.Config)
				require.True(t, ok)
				wantCfg := factory.CreateDefaultConfig()
				require.NoError(t, component.UnmarshalConfig(testCase.want, wantCfg))
				assert.Equal(t, wantCfg, gotCfg)
			}
		})
	}
}
