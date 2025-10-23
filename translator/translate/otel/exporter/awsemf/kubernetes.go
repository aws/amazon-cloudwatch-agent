// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsemf

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/awscontainerinsight"
)

func setKubernetesMetricDeclaration(conf *confmap.Conf, cfg *awsemfexporter.Config) error {
	var kubernetesMetricDeclarations []*awsemfexporter.MetricDeclaration
	// For all the supported metrics in K8s container insights, please see the following:
	// * https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Container-Insights-metrics-EKS.html
	// * https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/awscontainerinsightreceiver

	// Setup container metrics
	kubernetesMetricDeclarations = append(kubernetesMetricDeclarations, getContainerMetricDeclarations(conf)...)

	// Setup pod metrics
	kubernetesMetricDeclarations = append(kubernetesMetricDeclarations, getPodMetricDeclarations(conf)...)

	// Setup node metrics
	kubernetesMetricDeclarations = append(kubernetesMetricDeclarations, getNodeMetricDeclarations(conf)...)

	// Setup node filesystem metrics
	kubernetesMetricDeclarations = append(kubernetesMetricDeclarations, getNodeFilesystemMetricDeclarations(conf)...)

	// Setup service metrics
	kubernetesMetricDeclarations = append(kubernetesMetricDeclarations, getServiceMetricDeclarations()...)

	// Setup deployment metrics
	kubernetesMetricDeclarations = append(kubernetesMetricDeclarations, getDeploymentMetricDeclarations(conf)...)

	// Setup daemon set metrics
	kubernetesMetricDeclarations = append(kubernetesMetricDeclarations, getDaemonSetMetricDeclarations(conf)...)

	// Setup namespace metrics
	kubernetesMetricDeclarations = append(kubernetesMetricDeclarations, getNamespaceMetricDeclarations(conf)...)

	// Setup cluster metrics
	kubernetesMetricDeclarations = append(kubernetesMetricDeclarations, getClusterMetricDeclarations(conf)...)

	// Setup control plane metrics
	kubernetesMetricDeclarations = append(kubernetesMetricDeclarations, getControlPlaneMetricDeclarations(conf)...)

	// Setup GPU metrics
	kubernetesMetricDeclarations = append(kubernetesMetricDeclarations, getGPUMetricDeclarations(conf)...)

	// Setup Aws Neuron metrics
	kubernetesMetricDeclarations = append(kubernetesMetricDeclarations, getAwsNeuronMetricDeclarations(conf)...)

	kubernetesMetricDeclarations = append(kubernetesMetricDeclarations, getEFAMetricDeclarations(conf)...)

	kubernetesMetricDeclarations = append(kubernetesMetricDeclarations, getEBSMetricDeclarations(conf)...)

	kubernetesMetricDeclarations = append(kubernetesMetricDeclarations, getVolumesMetricDeclarations(conf)...)

	cfg.MetricDeclarations = kubernetesMetricDeclarations
	cfg.MetricDescriptors = getControlPlaneMetricDescriptors(conf)
	cfg.GaugeMetricsToCompact = getGaugeMetricsToCompact(conf)
	return nil
}

func getContainerMetricDeclarations(conf *confmap.Conf) []*awsemfexporter.MetricDeclaration {
	var containerMetricDeclarations []*awsemfexporter.MetricDeclaration
	enhancedContainerInsightsEnabled := awscontainerinsight.EnhancedContainerInsightsEnabled(conf)
	if enhancedContainerInsightsEnabled {

		metricDeclaration := awsemfexporter.MetricDeclaration{
			Dimensions: [][]string{{"ClusterName"}, {"ContainerName", "FullPodName", "PodName", "Namespace", "ClusterName"}, {"ContainerName", "PodName", "Namespace", "ClusterName"}},
			MetricNameSelectors: []string{
				"container_cpu_utilization", "container_cpu_utilization_over_container_limit", "container_cpu_limit", "container_cpu_request",
				"container_memory_utilization", "container_memory_utilization_over_container_limit", "container_memory_failures_total", "container_memory_limit", "container_memory_request",
				"container_filesystem_usage", "container_filesystem_available", "container_filesystem_utilization",
			},
		}

		containerMetricDeclarations = append(containerMetricDeclarations, &metricDeclaration)
	}
	return containerMetricDeclarations
}

func getPodMetricDeclarations(conf *confmap.Conf) []*awsemfexporter.MetricDeclaration {
	selectors := []string{"pod_cpu_reserved_capacity", "pod_memory_reserved_capacity"}
	dimensions := [][]string{{"PodName", "Namespace", "ClusterName"}, {"ClusterName"}}

	podMetricDeclarations := []*awsemfexporter.MetricDeclaration{
		{
			Dimensions: append(dimensions, []string{"Service", "Namespace", "ClusterName"}, []string{"ClusterName", "Namespace"}),
			MetricNameSelectors: []string{
				"pod_cpu_utilization", "pod_memory_utilization", "pod_network_rx_bytes", "pod_network_tx_bytes",
				"pod_cpu_utilization_over_pod_limit", "pod_memory_utilization_over_pod_limit",
			},
		},
	}

	enhancedContainerInsightsEnabled := awscontainerinsight.EnhancedContainerInsightsEnabled(conf)
	if enhancedContainerInsightsEnabled {
		dimensions = append(dimensions, []string{"FullPodName", "PodName", "Namespace", "ClusterName"}, []string{"Service", "Namespace", "ClusterName"})
		podMetricDeclarations[0].Dimensions = append(podMetricDeclarations[0].Dimensions, []string{"FullPodName", "PodName", "Namespace", "ClusterName"})
		selectors = append(selectors, []string{"pod_number_of_container_restarts", "pod_number_of_containers", "pod_number_of_running_containers",
			"pod_status_ready", "pod_status_scheduled", "pod_status_running", "pod_status_pending", "pod_status_failed", "pod_status_unknown",
			"pod_status_succeeded", "pod_memory_request", "pod_memory_limit", "pod_cpu_limit", "pod_cpu_request", "pod_cpu_usage_total", "pod_memory_working_set",
			"pod_container_status_running", "pod_container_status_terminated", "pod_container_status_waiting", "pod_container_status_waiting_reason_crash_loop_back_off",
			"pod_container_status_waiting_reason_image_pull_error", "pod_container_status_waiting_reason_start_error", "pod_container_status_waiting_reason_create_container_error",
			"pod_container_status_waiting_reason_create_container_config_error", "pod_container_status_terminated_reason_oom_killed",
		}...)
		if awscontainerinsight.AcceleratedComputeMetricsEnabled(conf) {
			selectors = append(selectors, "pod_gpu_request", "pod_gpu_limit", "pod_gpu_usage_total", "pod_gpu_reserved_capacity")
			selectors = append(selectors, "pod_neuroncore_request", "pod_neuroncore_limit", "pod_neuroncore_usage_total", "pod_neuroncore_reserved_capacity")
			selectors = append(selectors, "pod_efa_request", "pod_efa_limit", "pod_efa_usage_total", "pod_efa_reserved_capacity")
		}
	}

	metricDeclaration := awsemfexporter.MetricDeclaration{
		Dimensions:          dimensions,
		MetricNameSelectors: selectors,
	}

	if enhancedContainerInsightsEnabled {
		podMetricDeclarations = append(
			podMetricDeclarations,
			&awsemfexporter.MetricDeclaration{
				Dimensions: [][]string{
					{"FullPodName", "PodName", "Namespace", "ClusterName"},
					{"PodName", "Namespace", "ClusterName"},
					{"Namespace", "ClusterName"},
					{"ClusterName"},
				},
				MetricNameSelectors: []string{"pod_interface_network_rx_dropped", "pod_interface_network_tx_dropped"},
			},
		)
	} else {
		podMetricDeclarations = append(podMetricDeclarations, &awsemfexporter.MetricDeclaration{
			Dimensions:          [][]string{{"PodName", "Namespace", "ClusterName"}},
			MetricNameSelectors: []string{"pod_number_of_container_restarts"},
		})
	}

	podMetricDeclarations = append(
		podMetricDeclarations,
		&metricDeclaration)

	return podMetricDeclarations
}

func getNodeMetricDeclarations(conf *confmap.Conf) []*awsemfexporter.MetricDeclaration {
	enhancedContainerInsightsEnabled := awscontainerinsight.EnhancedContainerInsightsEnabled(conf)
	nodeMetrics := []string{
		"node_cpu_utilization", "node_memory_utilization", "node_network_total_bytes", "node_cpu_reserved_capacity",
		"node_memory_reserved_capacity", "node_number_of_running_pods", "node_number_of_running_containers",
		"node_cpu_usage_total", "node_cpu_limit", "node_memory_working_set", "node_memory_limit",
		"node_status_condition_ready", "node_status_condition_disk_pressure", "node_status_condition_memory_pressure",
		"node_status_condition_pid_pressure", "node_status_condition_network_unavailable", "node_status_condition_unknown",
		"node_status_capacity_pods", "node_status_allocatable_pods",
	}
	if awscontainerinsight.AcceleratedComputeMetricsEnabled(conf) {
		nodeMetrics = append(nodeMetrics, "node_gpu_limit", "node_gpu_usage_total", "node_gpu_reserved_capacity", "node_gpu_unreserved_capacity", "node_gpu_available_capacity")
		nodeMetrics = append(nodeMetrics, "node_neuroncore_limit", "node_neuroncore_usage_total", "node_neuroncore_reserved_capacity", "node_neuroncore_unreserved_capacity", "node_neuroncore_available_capacity")
		nodeMetrics = append(nodeMetrics, "node_efa_limit", "node_efa_usage_total", "node_efa_reserved_capacity", "node_efa_unreserved_capacity", "node_efa_available_capacity")
	}
	if enhancedContainerInsightsEnabled {
		return []*awsemfexporter.MetricDeclaration{
			{
				Dimensions:          [][]string{{"NodeName", "InstanceId", "ClusterName"}, {"ClusterName"}},
				MetricNameSelectors: nodeMetrics,
			},
			{
				Dimensions: [][]string{
					{"NodeName", "InstanceId", "ClusterName"},
					{"ClusterName"},
				},
				MetricNameSelectors: []string{
					"node_interface_network_rx_dropped", "node_interface_network_tx_dropped",
					"node_diskio_io_service_bytes_total", "node_diskio_io_serviced_total",
					"hyperpod_node_health_status_schedulable", "hyperpod_node_health_status_unschedulable_pending_replacement",
					"hyperpod_node_health_status_unschedulable_pending_reboot", "hyperpod_node_health_status_unschedulable",
				},
			},
		}
	} else {
		return []*awsemfexporter.MetricDeclaration{
			{
				Dimensions: [][]string{{"NodeName", "InstanceId", "ClusterName"}, {"ClusterName"}},
				MetricNameSelectors: []string{
					"node_cpu_utilization", "node_memory_utilization", "node_network_total_bytes", "node_cpu_reserved_capacity",
					"node_memory_reserved_capacity", "node_number_of_running_pods", "node_number_of_running_containers",
				},
			},
			{
				Dimensions: [][]string{{"ClusterName"}},
				MetricNameSelectors: []string{
					"node_cpu_usage_total", "node_cpu_limit", "node_memory_working_set", "node_memory_limit",
				},
			},
		}
	}
}

func getNodeFilesystemMetricDeclarations(conf *confmap.Conf) []*awsemfexporter.MetricDeclaration {
	metrics := []string{"node_filesystem_utilization"}
	enhancedContainerInsightsEnabled := awscontainerinsight.EnhancedContainerInsightsEnabled(conf)
	if enhancedContainerInsightsEnabled {
		metrics = append(metrics, "node_filesystem_inodes", "node_filesystem_inodes_free")
	}

	nodeFilesystemMetricDeclarations := []*awsemfexporter.MetricDeclaration{
		{
			Dimensions:          [][]string{{"NodeName", "InstanceId", "ClusterName"}, {"ClusterName"}},
			MetricNameSelectors: metrics,
		},
	}

	return nodeFilesystemMetricDeclarations
}

func getServiceMetricDeclarations() []*awsemfexporter.MetricDeclaration {
	return []*awsemfexporter.MetricDeclaration{
		{
			Dimensions: [][]string{{"Service", "Namespace", "ClusterName"}, {"ClusterName"}},
			MetricNameSelectors: []string{
				"service_number_of_running_pods",
			},
		},
	}
}

func getDeploymentMetricDeclarations(conf *confmap.Conf) []*awsemfexporter.MetricDeclaration {
	var deploymentMetricDeclarations []*awsemfexporter.MetricDeclaration
	enhancedContainerInsightsEnabled := awscontainerinsight.EnhancedContainerInsightsEnabled(conf)
	if enhancedContainerInsightsEnabled {
		deploymentMetricDeclarations = append(deploymentMetricDeclarations, []*awsemfexporter.MetricDeclaration{
			{
				Dimensions: [][]string{{"PodName", "Namespace", "ClusterName"}, {"ClusterName"}},
				MetricNameSelectors: []string{
					"replicas_desired", "replicas_ready", "status_replicas_available", "status_replicas_unavailable",
				},
			},
		}...)
	}
	return deploymentMetricDeclarations
}

func getDaemonSetMetricDeclarations(conf *confmap.Conf) []*awsemfexporter.MetricDeclaration {
	var daemonSetMetricDeclarations []*awsemfexporter.MetricDeclaration
	enhancedContainerInsightsEnabled := awscontainerinsight.EnhancedContainerInsightsEnabled(conf)
	if enhancedContainerInsightsEnabled {
		daemonSetMetricDeclarations = append(daemonSetMetricDeclarations, []*awsemfexporter.MetricDeclaration{
			{
				Dimensions: [][]string{{"PodName", "Namespace", "ClusterName"}, {"ClusterName"}},
				MetricNameSelectors: []string{
					"daemonset_status_number_available", "daemonset_status_number_unavailable",
				},
			},
		}...)
	}
	return daemonSetMetricDeclarations
}

func getNamespaceMetricDeclarations(conf *confmap.Conf) []*awsemfexporter.MetricDeclaration {
	metricNameSelectors := []string{"namespace_number_of_running_pods"}

	enhancedContainerInsightsEnabled := awscontainerinsight.EnhancedContainerInsightsEnabled(conf)
	if enhancedContainerInsightsEnabled {
		metricNameSelectors = append(metricNameSelectors, "namespace_ingress_count")
	}

	return []*awsemfexporter.MetricDeclaration{
		{
			Dimensions:          [][]string{{"Namespace", "ClusterName"}, {"ClusterName"}},
			MetricNameSelectors: metricNameSelectors,
		},
	}
}

func getClusterMetricDeclarations(conf *confmap.Conf) []*awsemfexporter.MetricDeclaration {
	metricNameSelectors := []string{"cluster_node_count", "cluster_failed_node_count"}

	enhancedContainerInsightsEnabled := awscontainerinsight.EnhancedContainerInsightsEnabled(conf)
	if enhancedContainerInsightsEnabled {
		metricNameSelectors = append(metricNameSelectors, "cluster_number_of_running_pods")
	}

	return []*awsemfexporter.MetricDeclaration{
		{
			Dimensions:          [][]string{{"ClusterName"}},
			MetricNameSelectors: metricNameSelectors,
		},
	}
}

func getControlPlaneMetricDeclarations(conf *confmap.Conf) []*awsemfexporter.MetricDeclaration {
	var metricDeclarations []*awsemfexporter.MetricDeclaration
	enhancedContainerInsightsEnabled := awscontainerinsight.EnhancedContainerInsightsEnabled(conf)
	if enhancedContainerInsightsEnabled {
		metricDeclarations = append(metricDeclarations, []*awsemfexporter.MetricDeclaration{
			{
				Dimensions: [][]string{{"ClusterName", "endpoint"}, {"ClusterName"}},
				MetricNameSelectors: []string{
					"apiserver_storage_size_bytes",
					"apiserver_storage_db_total_size_in_bytes",
					"etcd_db_total_size_in_bytes",
				},
			},
			{
				Dimensions: [][]string{{"ClusterName", "resource"}, {"ClusterName"}},
				MetricNameSelectors: []string{
					"apiserver_storage_list_duration_seconds",
					"apiserver_longrunning_requests",
					"apiserver_storage_objects",
				},
			},
			{
				Dimensions: [][]string{{"ClusterName", "verb"}, {"ClusterName"}},
				MetricNameSelectors: []string{
					"apiserver_request_duration_seconds",
					"rest_client_request_duration_seconds",
				},
			},
			{
				Dimensions: [][]string{{"ClusterName", "code", "verb"}, {"ClusterName"}},
				MetricNameSelectors: []string{
					"apiserver_request_total",
					"apiserver_request_total_5xx",
				},
			},
			{
				Dimensions: [][]string{{"ClusterName", "operation"}, {"ClusterName"}},
				MetricNameSelectors: []string{
					"apiserver_admission_controller_admission_duration_seconds",
					"apiserver_admission_step_admission_duration_seconds",
					"etcd_request_duration_seconds",
				},
			},
			{
				Dimensions: [][]string{{"ClusterName", "code", "method"}, {"ClusterName"}},
				MetricNameSelectors: []string{
					"rest_client_requests_total",
				},
			},
			{
				Dimensions: [][]string{{"ClusterName", "request_kind"}, {"ClusterName"}},
				MetricNameSelectors: []string{
					"apiserver_current_inflight_requests",
					"apiserver_current_inqueue_requests",
				},
			},
			{
				Dimensions: [][]string{{"ClusterName", "name"}, {"ClusterName"}},
				MetricNameSelectors: []string{
					"apiserver_admission_webhook_admission_duration_seconds",
				},
			},
			{
				Dimensions: [][]string{{"ClusterName", "group"}, {"ClusterName"}},
				MetricNameSelectors: []string{
					"apiserver_requested_deprecated_apis",
				},
			},
			{
				Dimensions: [][]string{{"ClusterName", "reason"}, {"ClusterName"}},
				MetricNameSelectors: []string{
					"apiserver_flowcontrol_rejected_requests_total",
				},
			},
			{
				Dimensions: [][]string{{"ClusterName", "priority_level"}, {"ClusterName"}},
				MetricNameSelectors: []string{
					"apiserver_flowcontrol_request_concurrency_limit",
				},
			},
		}...)
	}
	return metricDeclarations
}

func getControlPlaneMetricDescriptors(conf *confmap.Conf) []awsemfexporter.MetricDescriptor {
	enhancedContainerInsightsEnabled := awscontainerinsight.EnhancedContainerInsightsEnabled(conf)
	if enhancedContainerInsightsEnabled {
		// the control plane metrics do not have units so we need to add them manually
		return []awsemfexporter.MetricDescriptor{
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
				MetricName: "etcd_request_duration_seconds",
				Unit:       "Seconds",
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
				MetricName: "rest_client_request_duration_seconds",
				Unit:       "Seconds",
				Overwrite:  true,
			},
			{
				MetricName: "rest_client_requests_total",
				Unit:       "Count",
				Overwrite:  true,
			},
		}
	}
	return []awsemfexporter.MetricDescriptor{}

}

func getGPUMetricDeclarations(conf *confmap.Conf) []*awsemfexporter.MetricDeclaration {
	var metricDeclarations []*awsemfexporter.MetricDeclaration
	enhancedContainerInsightsEnabled := awscontainerinsight.EnhancedContainerInsightsEnabled(conf)
	if awscontainerinsight.AcceleratedComputeMetricsEnabled(conf) && enhancedContainerInsightsEnabled {
		metricDeclarations = append(metricDeclarations, []*awsemfexporter.MetricDeclaration{
			{
				Dimensions: [][]string{{"ClusterName"}, {"ClusterName", "Namespace", "PodName", "ContainerName"}, {"ClusterName", "Namespace", "PodName", "FullPodName", "ContainerName"}, {"ClusterName", "Namespace", "PodName", "FullPodName", "ContainerName", "GpuDevice"}},
				MetricNameSelectors: []string{
					"container_gpu_utilization",
					"container_gpu_memory_utilization",
					"container_gpu_memory_total",
					"container_gpu_memory_used",
					"container_gpu_power_draw",
					"container_gpu_temperature",
					"container_gpu_tensor_core_utilization",
				},
			},
			{
				Dimensions: [][]string{{"ClusterName"}, {"ClusterName", "Namespace"}, {"ClusterName", "Namespace", "Service"}, {"ClusterName", "Namespace", "PodName"}, {"ClusterName", "Namespace", "PodName", "FullPodName"}, {"ClusterName", "Namespace", "PodName", "FullPodName", "GpuDevice"}},
				MetricNameSelectors: []string{
					"pod_gpu_utilization",
					"pod_gpu_memory_utilization",
					"pod_gpu_memory_total",
					"pod_gpu_memory_used",
					"pod_gpu_power_draw",
					"pod_gpu_temperature",
					"pod_gpu_tensor_core_utilization",
				},
			},
			{
				Dimensions: [][]string{{"ClusterName"}, {"ClusterName", "NodeName", "InstanceId"}, {"ClusterName", "NodeName", "InstanceId", "InstanceType", "GpuDevice"}},
				MetricNameSelectors: []string{
					"node_gpu_utilization",
					"node_gpu_memory_utilization",
					"node_gpu_memory_total",
					"node_gpu_memory_used",
					"node_gpu_power_draw",
					"node_gpu_temperature",
					"node_gpu_tensor_core_utilization",
				},
			},
		}...)
	}
	return metricDeclarations
}

func getAwsNeuronMetricDeclarations(conf *confmap.Conf) []*awsemfexporter.MetricDeclaration {
	var metricDeclarations []*awsemfexporter.MetricDeclaration
	enhancedContainerInsightsEnabled := awscontainerinsight.EnhancedContainerInsightsEnabled(conf)
	if awscontainerinsight.AcceleratedComputeMetricsEnabled(conf) && enhancedContainerInsightsEnabled {
		metricDeclarations = append(metricDeclarations, []*awsemfexporter.MetricDeclaration{
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
				Dimensions: [][]string{{"ClusterName"}, {"ClusterName", "UltraServer"}, {"ClusterName", "InstanceId", "NodeName"}, {"ClusterName", "InstanceType", "InstanceId", "NodeName", "NeuronDevice", "NeuronCore"}},
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
				Dimensions: [][]string{{"ClusterName"}, {"ClusterName", "UltraServer"}, {"ClusterName", "InstanceId", "NodeName"}},
				MetricNameSelectors: []string{
					"node_neuron_execution_errors_total",
					"node_neurondevice_runtime_memory_used_bytes",
					"node_neuron_execution_latency",
				},
			},
			{
				Dimensions: [][]string{{"ClusterName"}, {"ClusterName", "UltraServer"}, {"ClusterName", "InstanceId", "NodeName"}, {"ClusterName", "InstanceId", "NodeName", "NeuronDevice"}},
				MetricNameSelectors: []string{
					"node_neurondevice_hw_ecc_events_total",
				},
			},
		}...)
	}
	return metricDeclarations
}

func getEFAMetricDeclarations(conf *confmap.Conf) []*awsemfexporter.MetricDeclaration {
	var metricDeclarations []*awsemfexporter.MetricDeclaration
	if awscontainerinsight.EnhancedContainerInsightsEnabled(conf) && awscontainerinsight.AcceleratedComputeMetricsEnabled(conf) {
		metricDeclarations = []*awsemfexporter.MetricDeclaration{
			{
				Dimensions: [][]string{
					{"ClusterName"},
					{"ClusterName", "Namespace", "PodName", "ContainerName"},
					{"ClusterName", "Namespace", "PodName", "FullPodName", "ContainerName"},
					{"ClusterName", "Namespace", "PodName", "FullPodName", "ContainerName", "NetworkInterfaceId"},
				},
				MetricNameSelectors: []string{
					"container_efa_rx_bytes",
					"container_efa_tx_bytes",
					"container_efa_rx_dropped",
					"container_efa_rdma_read_bytes",
					"container_efa_rdma_write_bytes",
					"container_efa_rdma_write_recv_bytes",
				},
			},
			{
				Dimensions: [][]string{
					{"ClusterName"},
					{"ClusterName", "Namespace"},
					{"ClusterName", "Namespace", "Service"},
					{"ClusterName", "Namespace", "PodName"},
					{"ClusterName", "Namespace", "PodName", "FullPodName"},
					{"ClusterName", "Namespace", "PodName", "FullPodName", "NetworkInterfaceId"},
				},
				MetricNameSelectors: []string{
					"pod_efa_rx_bytes",
					"pod_efa_tx_bytes",
					"pod_efa_rx_dropped",
					"pod_efa_rdma_read_bytes",
					"pod_efa_rdma_write_bytes",
					"pod_efa_rdma_write_recv_bytes",
				},
			},
			{
				Dimensions: [][]string{
					{"ClusterName"},
					{"ClusterName", "NodeName", "InstanceId"},
					{"ClusterName", "NodeName", "InstanceId", "InstanceType", "NetworkInterfaceId"},
				},
				MetricNameSelectors: []string{
					"node_efa_rx_bytes",
					"node_efa_tx_bytes",
					"node_efa_rx_dropped",
					"node_efa_rdma_read_bytes",
					"node_efa_rdma_write_bytes",
					"node_efa_rdma_write_recv_bytes",
				},
			},
		}
	}
	return metricDeclarations
}

func getEBSMetricDeclarations(conf *confmap.Conf) []*awsemfexporter.MetricDeclaration {
	var metricDeclarations []*awsemfexporter.MetricDeclaration
	if awscontainerinsight.EnhancedContainerInsightsEnabled(conf) {
		metricDeclarations = []*awsemfexporter.MetricDeclaration{
			{
				Dimensions: [][]string{
					{"ClusterName"},
					{"ClusterName", "NodeName", "InstanceId"},
					{"ClusterName", "NodeName", "InstanceId", "VolumeId"},
				},
				MetricNameSelectors: []string{
					"node_diskio_ebs_total_read_ops",
					"node_diskio_ebs_total_write_ops",
					"node_diskio_ebs_total_read_bytes",
					"node_diskio_ebs_total_write_bytes",
					"node_diskio_ebs_total_read_time",
					"node_diskio_ebs_total_write_time",
					"node_diskio_ebs_volume_performance_exceeded_iops",
					"node_diskio_ebs_volume_performance_exceeded_tp",
					"node_diskio_ebs_ec2_instance_performance_exceeded_iops",
					"node_diskio_ebs_ec2_instance_performance_exceeded_tp",
					"node_diskio_ebs_volume_queue_length",
				},
			},
		}
	}
	return metricDeclarations
}

func getVolumesMetricDeclarations(conf *confmap.Conf) []*awsemfexporter.MetricDeclaration {
	var metricDeclarations []*awsemfexporter.MetricDeclaration
	if awscontainerinsight.EnhancedContainerInsightsEnabled(conf) {
		metricDeclarations = []*awsemfexporter.MetricDeclaration{
			{
				Dimensions: [][]string{
					{"ClusterName"},
					{"ClusterName", "Namespace"},
					{"ClusterName", "Namespace", "PersistentVolumeClaimName"},
				},
				MetricNameSelectors: []string{
					"persistent_volume_claim_status_bound",
					"persistent_volume_claim_status_lost",
					"persistent_volume_claim_status_pending",
					"persistent_volume_claim_count",
				},
			},
			{
				Dimensions: [][]string{
					{"ClusterName"},
				},
				MetricNameSelectors: []string{
					"persistent_volume_count",
				},
			},
		}
	}
	return metricDeclarations
}

func getGaugeMetricsToCompact(conf *confmap.Conf) []string {
	var metricsToHistogram []string
	if awscontainerinsight.IsHighFrequencyGPUMetricsEnabled(conf) {
		var gpuMetricTypes = []string{
			containerinsightscommon.TypeGpuContainer,
			containerinsightscommon.TypeGpuPod,
			containerinsightscommon.TypeGpuNode,
		}

		// GPU metrics to be compacted to values and counts
		gpuMetrics := []string{
			containerinsightscommon.GpuUtilization,
			containerinsightscommon.GpuMemUtilization,
			containerinsightscommon.GpuMemTotal,
			containerinsightscommon.GpuMemUsed,
			containerinsightscommon.GpuPowerDraw,
			containerinsightscommon.GpuTemperature,
			containerinsightscommon.GpuTensorCoreUtilization,
		}

		// Generate metric names by looping through types and metrics
		for _, t := range gpuMetricTypes {
			for _, m := range gpuMetrics {
				metricsToHistogram = append(metricsToHistogram, containerinsightscommon.MetricName(t, m))
			}
		}
	}
	return metricsToHistogram
}
