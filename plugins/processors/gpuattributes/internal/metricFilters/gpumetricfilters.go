// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metricFilters

import (
	"github.com/aws/amazon-cloudwatch-agent/internal/constants"
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
	constants.ClusterNameKey:   nil,
	constants.InstanceIDKey:    nil,
	constants.GpuDeviceKey:     nil,
	constants.MetricType:       nil,
	constants.NodeNameKey:      nil,
	constants.K8sNamespace:     nil,
	constants.FullPodNameKey:   nil,
	constants.PodNameKey:       nil,
	constants.TypeService:      nil,
	constants.GpuUniqueID:      nil,
	constants.ContainerNamekey: nil,
	constants.InstanceTypeKey:  nil,
	constants.VersionKey:       nil,
	constants.SourcesKey:       nil,
	constants.Timestamp:        nil,
	constants.K8sKey: {
		constants.HostKey:      nil,
		constants.K8sLabelsKey: nil,
		pod_id:                 nil,
		pod_name:               nil,
		pod_owners:             nil,
		namespace:              nil,
		container_name:         nil,
		containerd:             nil,
	},
}
var PodGpuLabelFilter = map[string]map[string]interface{}{
	constants.ClusterNameKey:  nil,
	constants.InstanceIDKey:   nil,
	constants.GpuDeviceKey:    nil,
	constants.MetricType:      nil,
	constants.NodeNameKey:     nil,
	constants.K8sNamespace:    nil,
	constants.FullPodNameKey:  nil,
	constants.PodNameKey:      nil,
	constants.TypeService:     nil,
	constants.GpuUniqueID:     nil,
	constants.InstanceTypeKey: nil,
	constants.VersionKey:      nil,
	constants.SourcesKey:      nil,
	constants.Timestamp:       nil,
	constants.K8sKey: {
		constants.HostKey:      nil,
		constants.K8sLabelsKey: nil,
		pod_id:                 nil,
		pod_name:               nil,
		pod_owners:             nil,
		namespace:              nil,
	},
}
var NodeGpuLabelFilter = map[string]map[string]interface{}{
	constants.ClusterNameKey:  nil,
	constants.InstanceIDKey:   nil,
	constants.GpuDeviceKey:    nil,
	constants.MetricType:      nil,
	constants.NodeNameKey:     nil,
	constants.InstanceTypeKey: nil,
	constants.VersionKey:      nil,
	constants.SourcesKey:      nil,
	constants.Timestamp:       nil,
	constants.K8sKey: {
		constants.HostKey: nil,
	},
}

var PodNeuronLabelFilter = map[string]map[string]interface{}{
	constants.ClusterNameKey:  nil,
	constants.FullPodNameKey:  nil,
	constants.InstanceIDKey:   nil,
	constants.InstanceTypeKey: nil,
	constants.K8sPodNameKey:   nil,
	constants.K8sNamespace:    nil,
	internal.NeuronDevice:     nil,
	constants.NodeNameKey:     nil,
	constants.PodNameKey:      nil,
	constants.TypeService:     nil,
	internal.AvailabilityZone: nil,
	constants.K8sKey: {
		constants.HostKey:      nil,
		pod_id:                 nil,
		pod_owners:             nil,
		constants.K8sLabelsKey: nil,
	},
	internal.Region:      nil,
	internal.SubnetId:    nil,
	internal.NeuronCore:  nil,
	constants.MetricType: nil,
}

var ContainerNeuronLabelFilter = map[string]map[string]interface{}{
	constants.ClusterNameKey:   nil,
	constants.ContainerNamekey: nil,
	constants.FullPodNameKey:   nil,
	constants.InstanceIDKey:    nil,
	constants.InstanceTypeKey:  nil,
	constants.K8sPodNameKey:    nil,
	constants.K8sNamespace:     nil,
	internal.NeuronDevice:      nil,
	constants.NodeNameKey:      nil,
	constants.PodNameKey:       nil,
	constants.TypeService:      nil,
	internal.AvailabilityZone:  nil,
	constants.Kubernetes: {
		constants.HostKey:      nil,
		"containerd":           nil,
		pod_id:                 nil,
		pod_owners:             nil,
		constants.K8sLabelsKey: nil,
	},
	internal.Region:      nil,
	internal.SubnetId:    nil,
	internal.NeuronCore:  nil,
	constants.MetricType: nil,
}

var NodeNeuronLabelFilter = map[string]map[string]interface{}{
	constants.ClusterNameKey:  nil,
	constants.InstanceIDKey:   nil,
	constants.InstanceTypeKey: nil,
	constants.K8sNamespace:    nil,
	internal.NeuronDevice:     nil,
	constants.NodeNameKey:     nil,
	constants.TypeService:     nil,
	internal.AvailabilityZone: nil,
	constants.Kubernetes: {
		constants.HostKey:      nil,
		constants.K8sLabelsKey: nil,
	},
	internal.Region:      nil,
	internal.SubnetId:    nil,
	internal.NeuronCore:  nil,
	constants.MetricType: nil,
}
