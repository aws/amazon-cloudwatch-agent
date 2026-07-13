// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package containerinsightscommon

import (
	"testing"
)

func TestMetricName(t *testing.T) {
	testCases := []struct {
		name         string
		metricType   string
		metricName   string
		expectedName string
	}{
		// Node types should get "node_" prefix
		{"TypeNode", TypeNode, "cpu_utilization", "node_cpu_utilization"},
		{"TypeNodeFS", TypeNodeFS, "filesystem_usage", "node_filesystem_usage"},
		{"TypeNodeDiskIO", TypeNodeDiskIO, "diskio_read_ops", "node_diskio_read_ops"},
		{"TypeGpuNode", TypeGpuNode, "gpu_utilization", "node_gpu_utilization"},

		// Node network should get "node_interface_" prefix
		{"TypeNodeNet", TypeNodeNet, "network_rx_bytes", "node_interface_network_rx_bytes"},

		// Instance types should get "instance_" prefix
		{"TypeInstance", TypeInstance, "cpu_utilization", "instance_cpu_utilization"},
		{"TypeInstanceFS", TypeInstanceFS, "filesystem_usage", "instance_filesystem_usage"},
		{"TypeInstanceDiskIO", TypeInstanceDiskIO, "diskio_read_ops", "instance_diskio_read_ops"},

		// Instance network should get "instance_interface_" prefix
		{"TypeInstanceNet", TypeInstanceNet, "network_rx_bytes", "instance_interface_network_rx_bytes"},

		// Pod types should get "pod_" prefix
		{"TypePod", TypePod, "cpu_utilization", "pod_cpu_utilization"},
		{"TypeGpuPod", TypeGpuPod, "gpu_utilization", "pod_gpu_utilization"},

		// Pod network should get "pod_interface_" prefix
		{"TypePodNet", TypePodNet, "network_rx_bytes", "pod_interface_network_rx_bytes"},

		// Container types should get "container_" prefix
		{"TypeContainer", TypeContainer, "cpu_utilization", "container_cpu_utilization"},
		{"TypeContainerDiskIO", TypeContainerDiskIO, "diskio_read_ops", "container_diskio_read_ops"},
		{"TypeContainerFS", TypeContainerFS, "filesystem_usage", "container_filesystem_usage"},
		{"TypeGpuContainer", TypeGpuContainer, "gpu_utilization", "container_gpu_utilization"},

		// Service should get "service_" prefix
		{"TypeService", TypeService, "number_of_running_pods", "service_number_of_running_pods"},

		// Cluster types should get "cluster_" prefix
		{"TypeCluster", TypeCluster, "node_count", "cluster_node_count"},
		{"TypeGpuCluster", TypeGpuCluster, "gpu_count", "cluster_gpu_count"},
		{"TypeClusterQueue", TypeClusterQueue, "pending_workloads", "cluster_pending_workloads"},

		// Namespace should get "namespace_" prefix
		{"K8sNamespace", K8sNamespace, "number_of_running_pods", "namespace_number_of_running_pods"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := MetricName(tc.metricType, tc.metricName)
			if result != tc.expectedName {
				t.Errorf("MetricName(%q, %q) = %q, want %q", tc.metricType, tc.metricName, result, tc.expectedName)
			}
		})
	}
}

func TestMetricNameWithInvalidTypes(t *testing.T) {
	// These types should NOT be used with MetricName - they are for labels only
	invalidTypes := []struct {
		name       string
		metricType string
		reason     string
	}{
		{"TypeNodeEBS", TypeNodeEBS, "used for labels only, not metric naming"},
		{"TypeNodeInstanceStore", TypeNodeInstanceStore, "used for labels only, not metric naming"},
		{"UnknownType", "unknown_type", "not a valid metric type"},
	}

	for _, tc := range invalidTypes {
		t.Run(tc.name, func(t *testing.T) {
			// These should return the metric name unchanged (no prefix added)
			result := MetricName(tc.metricType, "some_metric")
			expected := "some_metric"
			if result != expected {
				t.Errorf("MetricName(%q, %q) = %q, want %q (reason: %s)",
					tc.metricType, "some_metric", result, expected, tc.reason)
			}
		})
	}
}

func TestMetricNameWithRealConstants(t *testing.T) {
	// Test with actual constants used in the codebase
	testCases := []struct {
		name         string
		metricType   string
		constant     string
		expectedName string
	}{
		// EBS metrics
		{"EBS Read Ops", TypeNode, NvmeReadOpsTotal, "node_" + NvmeReadOpsTotal},
		{"EBS Write Ops", TypeNode, NvmeWriteOpsTotal, "node_" + NvmeWriteOpsTotal},
		{"EBS Queue Length", TypeNode, NvmeVolumeQueueLength, "node_" + NvmeVolumeQueueLength},

		// Instance Store metrics
		{"Instance Store Read Ops", TypeNode, NvmeInstanceStoreReadOpsTotal, "node_" + NvmeInstanceStoreReadOpsTotal},
		{"Instance Store Write Ops", TypeNode, NvmeInstanceStoreWriteOpsTotal, "node_" + NvmeInstanceStoreWriteOpsTotal},
		{"Instance Store Queue Length", TypeNode, NvmeInstanceStoreVolumeQueueLength, "node_" + NvmeInstanceStoreVolumeQueueLength},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := MetricName(tc.metricType, tc.constant)
			if result != tc.expectedName {
				t.Errorf("MetricName(%q, %q) = %q, want %q", tc.metricType, tc.constant, result, tc.expectedName)
			}
		})
	}
}
