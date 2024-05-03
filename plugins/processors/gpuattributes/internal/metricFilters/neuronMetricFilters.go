// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metricFilters

import (
	"github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/gpuattributes/internal"
)

var CommonNeuronMetricFilter = map[string]map[string]interface{}{
	internal.ClusterName:      nil,
	internal.ContainerName:    nil,
	internal.FullPodName:      nil,
	internal.InstanceId:       nil,
	internal.InstanceType:     nil,
	internal.K8sPodName:       nil,
	internal.Namespace:        nil,
	internal.NeuronDevice:     nil,
	internal.NodeName:         nil,
	internal.PodName:          nil,
	internal.Service:          nil,
	internal.AvailabilityZone: nil,
	internal.Kubernetes: {
		containerinsightscommon.HostKey: nil,
		"containerd":                    nil,
		"pod_id":                        nil,
		"pod_owners":                    nil,
		"labels":                        nil,
	},
	internal.Region:                    nil,
	internal.RuntimeTag:                nil,
	internal.SubnetId:                  nil,
	internal.NeuronCore:                nil,
	containerinsightscommon.MetricType: nil,
}
var NodeAWSNeuronDeviceMetricFilter = map[string]map[string]interface{}{
	internal.ClusterName:      nil,
	internal.ContainerName:    nil,
	internal.FullPodName:      nil,
	internal.InstanceId:       nil,
	internal.InstanceType:     nil,
	internal.K8sPodName:       nil,
	internal.Namespace:        nil,
	internal.NeuronDevice:     nil,
	internal.NodeName:         nil,
	internal.PodName:          nil,
	internal.Service:          nil,
	internal.AvailabilityZone: nil,
	internal.Kubernetes: {
		containerinsightscommon.HostKey: nil,
		"containerd":                    nil,
		"pod_id":                        nil,
		"pod_owners":                    nil,
	},
	internal.Region:                    nil,
	internal.RuntimeTag:                nil,
	internal.SubnetId:                  nil,
	internal.NeuronCore:                nil,
	containerinsightscommon.MetricType: nil,
}
