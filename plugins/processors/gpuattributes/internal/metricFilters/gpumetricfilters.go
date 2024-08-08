// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metricFilters

import (
	"github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/gpuattributes/internal"
)

// This class contains the attribute filters which are applied to the metric datapoints of GPU and Neuron metrics.
// If the datapoint contains metrics apart from the ones mentioned in the filter, then they'll be dropped.

const (
	containerd     = "containerd"
	pod_id         = "pod_id"
	pod_name       = "pod_name"
	pod_owners     = "pod_owners"
	namespace      = "namespace"
	container_name = "container_name"
)

var ContainerGpuLabelFilter = map[string]map[string]interface{}{
	containerinsightscommon.ClusterNameKey:   nil,
	containerinsightscommon.InstanceIdKey:    nil,
	containerinsightscommon.GpuDeviceKey:     nil,
	containerinsightscommon.MetricType:       nil,
	containerinsightscommon.NodeNameKey:      nil,
	containerinsightscommon.K8sNamespace:     nil,
	containerinsightscommon.FullPodNameKey:   nil,
	containerinsightscommon.PodNameKey:       nil,
	containerinsightscommon.TypeService:      nil,
	containerinsightscommon.GpuUniqueId:      nil,
	containerinsightscommon.ContainerNamekey: nil,
	containerinsightscommon.InstanceTypeKey:  nil,
	containerinsightscommon.VersionKey:       nil,
	containerinsightscommon.SourcesKey:       nil,
	containerinsightscommon.Timestamp:        nil,
	containerinsightscommon.K8sKey: {
		containerinsightscommon.HostKey:      nil,
		containerinsightscommon.K8sLabelsKey: nil,
		pod_id:                               nil,
		pod_name:                             nil,
		pod_owners:                           nil,
		namespace:                            nil,
		container_name:                       nil,
		containerd:                           nil,
	},
}
var PodGpuLabelFilter = map[string]map[string]interface{}{
	containerinsightscommon.ClusterNameKey:  nil,
	containerinsightscommon.InstanceIdKey:   nil,
	containerinsightscommon.GpuDeviceKey:    nil,
	containerinsightscommon.MetricType:      nil,
	containerinsightscommon.NodeNameKey:     nil,
	containerinsightscommon.K8sNamespace:    nil,
	containerinsightscommon.FullPodNameKey:  nil,
	containerinsightscommon.PodNameKey:      nil,
	containerinsightscommon.TypeService:     nil,
	containerinsightscommon.GpuUniqueId:     nil,
	containerinsightscommon.InstanceTypeKey: nil,
	containerinsightscommon.VersionKey:      nil,
	containerinsightscommon.SourcesKey:      nil,
	containerinsightscommon.Timestamp:       nil,
	containerinsightscommon.K8sKey: {
		containerinsightscommon.HostKey:      nil,
		containerinsightscommon.K8sLabelsKey: nil,
		pod_id:                               nil,
		pod_name:                             nil,
		pod_owners:                           nil,
		namespace:                            nil,
	},
}
var NodeGpuLabelFilter = map[string]map[string]interface{}{
	containerinsightscommon.ClusterNameKey:  nil,
	containerinsightscommon.InstanceIdKey:   nil,
	containerinsightscommon.GpuDeviceKey:    nil,
	containerinsightscommon.MetricType:      nil,
	containerinsightscommon.NodeNameKey:     nil,
	containerinsightscommon.InstanceTypeKey: nil,
	containerinsightscommon.VersionKey:      nil,
	containerinsightscommon.SourcesKey:      nil,
	containerinsightscommon.Timestamp:       nil,
	containerinsightscommon.K8sKey: {
		containerinsightscommon.HostKey: nil,
	},
}

var PodNeuronLabelFilter = map[string]map[string]interface{}{
	containerinsightscommon.ClusterNameKey:  nil,
	containerinsightscommon.FullPodNameKey:  nil,
	containerinsightscommon.InstanceIdKey:   nil,
	containerinsightscommon.InstanceTypeKey: nil,
	containerinsightscommon.K8sPodNameKey:   nil,
	containerinsightscommon.K8sNamespace:    nil,
	internal.NeuronDevice:                   nil,
	containerinsightscommon.NodeNameKey:     nil,
	containerinsightscommon.PodNameKey:      nil,
	containerinsightscommon.TypeService:     nil,
	internal.AvailabilityZone:               nil,
	containerinsightscommon.K8sKey: {
		containerinsightscommon.HostKey:      nil,
		pod_id:                               nil,
		pod_owners:                           nil,
		containerinsightscommon.K8sLabelsKey: nil,
	},
	internal.Region:                    nil,
	internal.SubnetId:                  nil,
	internal.NeuronCore:                nil,
	containerinsightscommon.MetricType: nil,
}

var ContainerNeuronLabelFilter = map[string]map[string]interface{}{
	containerinsightscommon.ClusterNameKey:   nil,
	containerinsightscommon.ContainerNamekey: nil,
	containerinsightscommon.FullPodNameKey:   nil,
	containerinsightscommon.InstanceIdKey:    nil,
	containerinsightscommon.InstanceTypeKey:  nil,
	containerinsightscommon.K8sPodNameKey:    nil,
	containerinsightscommon.K8sNamespace:     nil,
	internal.NeuronDevice:                    nil,
	containerinsightscommon.NodeNameKey:      nil,
	containerinsightscommon.PodNameKey:       nil,
	containerinsightscommon.TypeService:      nil,
	internal.AvailabilityZone:                nil,
	containerinsightscommon.Kubernetes: {
		containerinsightscommon.HostKey:      nil,
		"containerd":                         nil,
		pod_id:                               nil,
		pod_owners:                           nil,
		containerinsightscommon.K8sLabelsKey: nil,
	},
	internal.Region:                    nil,
	internal.SubnetId:                  nil,
	internal.NeuronCore:                nil,
	containerinsightscommon.MetricType: nil,
}

var NodeNeuronLabelFilter = map[string]map[string]interface{}{
	containerinsightscommon.ClusterNameKey:  nil,
	containerinsightscommon.InstanceIdKey:   nil,
	containerinsightscommon.InstanceTypeKey: nil,
	containerinsightscommon.K8sNamespace:    nil,
	internal.NeuronDevice:                   nil,
	containerinsightscommon.NodeNameKey:     nil,
	containerinsightscommon.TypeService:     nil,
	internal.AvailabilityZone:               nil,
	containerinsightscommon.Kubernetes: {
		containerinsightscommon.HostKey:      nil,
		containerinsightscommon.K8sLabelsKey: nil,
	},
	internal.Region:                    nil,
	internal.SubnetId:                  nil,
	internal.NeuronCore:                nil,
	containerinsightscommon.MetricType: nil,
}
