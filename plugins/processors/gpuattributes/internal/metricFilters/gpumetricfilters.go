package metricFilters

import "github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"

var ContainerLabelFilter = map[string]map[string]interface{}{
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
		containerinsightscommon.HostKey: nil,
		"labels":                        nil,
		"pod_id":                        nil,
		"pod_name":                      nil,
		"pod_owners":                    nil,
		"namespace":                     nil,
		"container_name":                nil,
		"containerd":                    nil,
	},
}
var PodLabelFilter = map[string]map[string]interface{}{
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
		containerinsightscommon.HostKey: nil,
		"labels":                        nil,
		"pod_id":                        nil,
		"pod_name":                      nil,
		"pod_owners":                    nil,
		"namespace":                     nil,
	},
}
var NodeLabelFilter = map[string]map[string]interface{}{
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
