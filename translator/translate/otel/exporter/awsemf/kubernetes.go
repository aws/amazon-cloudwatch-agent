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

	// Setup node metrics
	kubernetesMetricDeclarations = append(kubernetesMetricDeclarations, []*awsemfexporter.MetricDeclaration{
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
	}...)

	// Setup pod metrics
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

	kubernetesMetricDeclarations = append(kubernetesMetricDeclarations, podMetricDeclarations...)

	//Setup container metrics
	enableContainerMetricsKey := common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.KubernetesKey, common.EnableContainerMetricsKey)
	if common.GetOrDefaultBool(conf, enableContainerMetricsKey, false) {
		kubernetesMetricDeclarations = append(kubernetesMetricDeclarations, []*awsemfexporter.MetricDeclaration{
			{
				Dimensions: [][]string{{"ContainerName", "FullPodName", "Namespace", "ClusterName"}, {"ContainerName", "Namespace", "ClusterName"}},
				MetricNameSelectors: []string{
					"container_cpu_utilization", "container_memory_utilization", "container_filesystem_usage",
				},
			},
		}...)
	}

	// Setup cluster metrics
	kubernetesMetricDeclarations = append(kubernetesMetricDeclarations, []*awsemfexporter.MetricDeclaration{
		{
			Dimensions: [][]string{{"ClusterName"}},
			MetricNameSelectors: []string{
				"cluster_node_count", "cluster_failed_node_count",
			},
		},
	}...)

	// Setup service metrics
	kubernetesMetricDeclarations = append(kubernetesMetricDeclarations, []*awsemfexporter.MetricDeclaration{
		{
			Dimensions: [][]string{{"Service", "Namespace", "ClusterName"}, {"ClusterName"}},
			MetricNameSelectors: []string{
				"service_number_of_running_pods",
			},
		},
	}...)

	// Setup node filesystem metrics
	kubernetesMetricDeclarations = append(kubernetesMetricDeclarations, []*awsemfexporter.MetricDeclaration{
		{
			Dimensions: [][]string{{"NodeName", "InstanceId", "ClusterName"}, {"ClusterName"}},
			MetricNameSelectors: []string{
				"node_filesystem_utilization",
			},
		},
	}...)

	// Setup namespace metrics
	kubernetesMetricDeclarations = append(kubernetesMetricDeclarations, []*awsemfexporter.MetricDeclaration{
		{
			Dimensions: [][]string{{"Namespace", "ClusterName"}, {"ClusterName"}},
			MetricNameSelectors: []string{
				"namespace_number_of_running_pods",
			},
		},
	}...)

	cfg.MetricDeclarations = kubernetesMetricDeclarations
	return nil
}
