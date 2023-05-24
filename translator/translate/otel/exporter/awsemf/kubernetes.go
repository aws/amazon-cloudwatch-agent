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
	}
	return containerMetricDeclarations
}

func getPodMetricDeclarations(conf *confmap.Conf) []*awsemfexporter.MetricDeclaration {
	podMetricDeclarations := []*awsemfexporter.MetricDeclaration{
		{
			Dimensions: [][]string{{"PodName", "Namespace", "ClusterName"}, {"Service", "Namespace", "ClusterName"}, {"Namespace", "ClusterName"}, {"ClusterName"}},
			MetricNameSelectors: []string{
				"pod_cpu_utilization", "pod_memory_utilization", "pod_network_rx_bytes", "pod_network_tx_bytes",
				"pod_cpu_utilization_over_pod_limit", "pod_memory_utilization_over_pod_limit",
			},
		},
		{
			Dimensions: [][]string{{"PodName", "Namespace", "ClusterName"}, {"ClusterName"}},
			MetricNameSelectors: []string{
				"pod_cpu_reserved_capacity", "pod_memory_reserved_capacity",
			},
		},
		{
			Dimensions: [][]string{{"PodName", "Namespace", "ClusterName"}},
			MetricNameSelectors: []string{
				"pod_number_of_container_restarts",
			},
		},
	}

	enableFullPodMetricsKey := common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.KubernetesKey, common.EnableFullPodMetricsKey)
	if common.GetOrDefaultBool(conf, enableFullPodMetricsKey, false) {
		for _, declaration := range podMetricDeclarations {
			declaration.Dimensions = append([][]string{{"FullPodName", "PodName", "Namespace", "ClusterName"}}, declaration.Dimensions...)
		}
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
