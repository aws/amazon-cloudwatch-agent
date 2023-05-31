// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsemf

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
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
	kubernetesMetricDeclarations = append(kubernetesMetricDeclarations, getClusterMetricDeclarations()...)

	cfg.MetricDeclarations = kubernetesMetricDeclarations
	return nil
}

func getContainerMetricDeclarations(conf *confmap.Conf) []*awsemfexporter.MetricDeclaration {
	var containerMetricDeclarations []*awsemfexporter.MetricDeclaration
	enableContainerMetricsKey := common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.KubernetesKey, common.EnableContainerMetricsKey)
	if common.GetOrDefaultBool(conf, enableContainerMetricsKey, false) {
		containerMetricDeclarations = append(containerMetricDeclarations, []*awsemfexporter.MetricDeclaration{
			{
				Dimensions: [][]string{{"ContainerName", "FullPodName", "Namespace", "ClusterName"}, {"ContainerName", "Namespace", "ClusterName"}},
				MetricNameSelectors: []string{
					"container_cpu_utilization", "container_memory_utilization", "container_filesystem_usage",
				},
			},
		}...)

		// additional container-pod metrics
		containerMetricDeclarations = append(containerMetricDeclarations, []*awsemfexporter.MetricDeclaration{
			{
				Dimensions: [][]string{{"ContainerName", "FullPodName", "PodName", "Namespace", "ClusterName"}, {"ContainerName", "PodName", "Namespace", "ClusterName"}},
				MetricNameSelectors: []string{
					"container_status_running", "container_status_terminated", "container_status_waiting", "container_status_waiting_reason_crashed",
				},
			},
		}...)
	}
	return containerMetricDeclarations
}

func getPodMetricDeclarations(conf *confmap.Conf) []*awsemfexporter.MetricDeclaration {
	enableFullPodMetricsKey := common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.KubernetesKey, common.EnableFullPodMetricsKey)
	isEnableFullPodMetrics := common.GetOrDefaultBool(conf, enableFullPodMetricsKey, false)

	podMetricDeclarations := []*awsemfexporter.MetricDeclaration{
		{
			Dimensions: [][]string{{"PodName", "Namespace", "ClusterName"}, {"Service", "Namespace", "ClusterName"}, {"Namespace", "ClusterName"}, {"ClusterName"}},
			MetricNameSelectors: []string{
				"pod_cpu_utilization", "pod_memory_utilization", "pod_network_rx_bytes", "pod_network_tx_bytes",
				"pod_cpu_utilization_over_pod_limit", "pod_memory_utilization_over_pod_limit",
			},
		},
	}
	metricDeclaration := awsemfexporter.MetricDeclaration{
		Dimensions:          [][]string{{"PodName", "Namespace", "ClusterName"}, {"ClusterName"}},
		MetricNameSelectors: []string{"pod_cpu_reserved_capacity", "pod_memory_reserved_capacity"},
	}

	if !isEnableFullPodMetrics {
		podMetricDeclarations = append(
			podMetricDeclarations,
			&metricDeclaration,
			&awsemfexporter.MetricDeclaration{
				Dimensions:          [][]string{{"PodName", "Namespace", "ClusterName"}},
				MetricNameSelectors: []string{"pod_number_of_container_restarts"},
			},
		)
		return podMetricDeclarations
	}

	metricDeclaration.Dimensions = append(metricDeclaration.Dimensions, []string{"Service", "Namespace", "ClusterName"})
	metricDeclaration.MetricNameSelectors = append(
		metricDeclaration.MetricNameSelectors, "pod_number_of_container_restarts", "pod_number_of_containers", "pod_number_of_running_containers")
	podMetricDeclarations = append(
		podMetricDeclarations,
		&metricDeclaration,
	)

	for _, declaration := range podMetricDeclarations {
		declaration.Dimensions = append([][]string{{"FullPodName", "PodName", "Namespace", "ClusterName"}}, declaration.Dimensions...)
	}

	return podMetricDeclarations
}

func getNodeMetricDeclarations(conf *confmap.Conf) []*awsemfexporter.MetricDeclaration {
	enableNodeDetailedMetricsKey := common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.KubernetesKey, common.EnableNodeDetailedMetricsKey)
	if common.GetOrDefaultBool(conf, enableNodeDetailedMetricsKey, false) {
		return []*awsemfexporter.MetricDeclaration{
			{
				Dimensions: [][]string{{"NodeName", "InstanceId", "ClusterName"}, {"ClusterName"}},
				MetricNameSelectors: []string{
					"node_cpu_utilization", "node_memory_utilization", "node_network_total_bytes", "node_cpu_reserved_capacity",
					"node_memory_reserved_capacity", "node_number_of_running_pods", "node_number_of_running_containers",
					"node_cpu_usage_total", "node_cpu_limit", "node_memory_working_set", "node_memory_limit",
					"node_status_condition_ready", "node_status_condition_disk_pressure", "node_status_condition_memory_pressure",
					"node_status_condition_pid_pressure", "node_status_condition_network_unavailable",
					"node_status_capacity_pods", "node_status_allocatable_pods",
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
	enableNodeDetailedMetricsKey := common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.KubernetesKey, common.EnableNodeDetailedMetricsKey)
	if common.GetOrDefaultBool(conf, enableNodeDetailedMetricsKey, false) {
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
	// As a placeholder, using the enable_container_metrics flag for now. This will be replaced soon.
	enableContainerMetricsKey := common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.KubernetesKey, common.EnableContainerMetricsKey)
	if common.GetOrDefaultBool(conf, enableContainerMetricsKey, false) {
		deploymentMetricDeclarations = append(deploymentMetricDeclarations, []*awsemfexporter.MetricDeclaration{
			{
				Dimensions: [][]string{{"PodName", "Namespace", "ClusterName"}, {"ClusterName"}},
				MetricNameSelectors: []string{
					"deployment_spec_replicas", "deployment_status_replicas", "deployment_status_replicas_available", "deployment_status_replicas_unavailable",
				},
			},
		}...)
	}
	return deploymentMetricDeclarations
}

func getDaemonSetMetricDeclarations(conf *confmap.Conf) []*awsemfexporter.MetricDeclaration {
	var daemonSetMetricDeclarations []*awsemfexporter.MetricDeclaration
	// As a placeholder, using the enable_container_metrics flag for now. This will be replaced soon.
	enableContainerMetricsKey := common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.KubernetesKey, common.EnableContainerMetricsKey)
	if common.GetOrDefaultBool(conf, enableContainerMetricsKey, false) {
		daemonSetMetricDeclarations = append(daemonSetMetricDeclarations, []*awsemfexporter.MetricDeclaration{
			{
				Dimensions: [][]string{{"PodName", "Namespace", "ClusterName"}, {"ClusterName"}},
				MetricNameSelectors: []string{
					"daemonset_status_number_available", "daemonset_status_number_unavailable",
					"daemonset_status_desired_number_scheduled", "daemonset_status_current_number_scheduled",
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

func getClusterMetricDeclarations() []*awsemfexporter.MetricDeclaration {
	return []*awsemfexporter.MetricDeclaration{
		{
			Dimensions: [][]string{{"ClusterName"}},
			MetricNameSelectors: []string{
				"cluster_node_count", "cluster_failed_node_count",
			},
		},
	}
}
