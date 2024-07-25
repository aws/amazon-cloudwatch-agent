package metricFilters

import (
	"github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/gpuattributes/internal"
)

const label = "labels"

var PodNeuronMetricFilter = map[string]map[string]interface{}{
	internal.ClusterName:      nil,
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
		"pod_id":                        nil,
		"pod_owners":                    nil,
		label:                           nil,
	},
	internal.Region:                    nil,
	internal.RuntimeTag:                nil,
	internal.SubnetId:                  nil,
	internal.NeuronCore:                nil,
	containerinsightscommon.MetricType: nil,
}

var ContainerNeuronMetricFilter = map[string]map[string]interface{}{
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
		label:                           nil,
	},
	internal.Region:                    nil,
	internal.RuntimeTag:                nil,
	internal.SubnetId:                  nil,
	internal.NeuronCore:                nil,
	containerinsightscommon.MetricType: nil,
}

var NodeNeuronMetricFilter = map[string]map[string]interface{}{
	internal.ClusterName:      nil,
	internal.InstanceId:       nil,
	internal.InstanceType:     nil,
	internal.Namespace:        nil,
	internal.NeuronDevice:     nil,
	internal.NodeName:         nil,
	internal.Service:          nil,
	internal.AvailabilityZone: nil,
	internal.Kubernetes: {
		containerinsightscommon.HostKey: nil,
		label:                           nil,
	},
	internal.Region:                    nil,
	internal.RuntimeTag:                nil,
	internal.SubnetId:                  nil,
	internal.NeuronCore:                nil,
	containerinsightscommon.MetricType: nil,
}
