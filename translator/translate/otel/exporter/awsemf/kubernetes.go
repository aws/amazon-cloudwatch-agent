// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsemf

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"
	"go.opentelemetry.io/collector/confmap"

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
	kubernetesMetricDeclarations = append(kubernetesMetricDeclarations, getNamespaceMetricDeclarations()...)

	// Setup cluster metrics
	kubernetesMetricDeclarations = append(kubernetesMetricDeclarations, getClusterMetricDeclarations(conf)...)

	// Setup control plane metrics
	kubernetesMetricDeclarations = append(kubernetesMetricDeclarations, getControlPlaneMetricDeclarations(conf)...)

	cfg.MetricDeclarations = kubernetesMetricDeclarations
	cfg.MetricDescriptors = getControlPlaneMetricDescriptors(conf)

	return nil
}

func getContainerMetricDeclarations(conf *confmap.Conf) []*awsemfexporter.MetricDeclaration {
	var containerMetricDeclarations []*awsemfexporter.MetricDeclaration
	enhancedContainerInsightsEnabled := awscontainerinsight.EnhancedContainerInsightsEnabled(conf)
	if enhancedContainerInsightsEnabled {

		metricDeclaration := awsemfexporter.MetricDeclaration{
			Dimensions: [][]string{{"ClusterName"}, {"ContainerName", "FullPodName", "PodName", "Namespace", "ClusterName"}, {"ContainerName", "PodName", "Namespace", "ClusterName"}},
			MetricNameSelectors: []string{
				"container_cpu_utilization", "container_cpu_utilization_over_container_limit",
				"container_memory_utilization", "container_memory_utilization_over_container_limit", "container_memory_failures_total",
				"container_filesystem_usage", "container_status_running", "container_status_terminated", "container_status_waiting", "container_status_waiting_reason_crashed",
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
			"pod_status_succeeded"}...)

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
					{"Service", "Namespace", "ClusterName"},
					{"ClusterName"},
				},
				MetricNameSelectors: []string{"pod_interface_network_rx_dropped", "pod_interface_network_rx_errors", "pod_interface_network_tx_dropped", "pod_interface_network_tx_errors"},
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
	if enhancedContainerInsightsEnabled {
		return []*awsemfexporter.MetricDeclaration{
			{
				Dimensions: [][]string{{"NodeName", "InstanceId", "ClusterName"}, {"ClusterName"}},
				MetricNameSelectors: []string{
					"node_cpu_utilization", "node_memory_utilization", "node_network_total_bytes", "node_cpu_reserved_capacity",
					"node_memory_reserved_capacity", "node_number_of_running_pods", "node_number_of_running_containers",
					"node_cpu_usage_total", "node_cpu_limit", "node_memory_working_set", "node_memory_limit",
					"node_status_condition_ready", "node_status_condition_disk_pressure", "node_status_condition_memory_pressure",
					"node_status_condition_pid_pressure", "node_status_condition_network_unavailable", "node_status_condition_unknown",
					"node_status_capacity_pods", "node_status_allocatable_pods",
				},
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

func getNamespaceMetricDeclarations() []*awsemfexporter.MetricDeclaration {
	return []*awsemfexporter.MetricDeclaration{
		{
			Dimensions: [][]string{{"Namespace", "ClusterName"}, {"ClusterName"}},
			MetricNameSelectors: []string{
				"namespace_number_of_running_pods",
			},
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
					"etcd_db_total_size_in_bytes",
				},
			},
			{
				Dimensions: [][]string{{"ClusterName", "resource"}, {"ClusterName"}},
				MetricNameSelectors: []string{
					"apiserver_storage_list_duration_seconds",
				},
			},
			{
				Dimensions: [][]string{{"ClusterName"}},
				MetricNameSelectors: []string{
					"apiserver_storage_objects",
					"apiserver_request_total",
					"apiserver_request_total_5xx",
					"apiserver_request_duration_seconds",
					"apiserver_admission_controller_admission_duration_seconds",
					"rest_client_request_duration_seconds",
					"rest_client_requests_total",
					"etcd_request_duration_seconds",
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
				MetricName: "apiserver_storage_objects",
				Unit:       "Count",
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
				MetricName: "apiserver_request_duration_seconds",
				Unit:       "Seconds",
				Overwrite:  true,
			},
			{
				MetricName: "apiserver_admission_controller_admission_duration_seconds",
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
			{
				MetricName: "etcd_request_duration_seconds",
				Unit:       "Seconds",
				Overwrite:  true,
			},
		}
	}
	return []awsemfexporter.MetricDescriptor{}

}
