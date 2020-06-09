package containerinsightscommon

import (
	"log"
)

func IsNode(mType string) bool {
	return mType == TypeNode || mType == TypeNodeNet || mType == TypeNodeFS || mType == TypeNodeDiskIO
}
func IsInstance(mType string) bool {
	return mType == TypeInstance || mType == TypeInstanceNet || mType == TypeInstanceFS || mType == TypeInstanceDiskIO
}
func IsContainer(mType string) bool {
	return mType == TypeContainer || mType == TypeContainerDiskIO || mType == TypeContainerFS
}
func IsPod(mType string) bool {
	return mType == TypePod || mType == TypePodNet
}

func MetricName(mType string, name string) string {
	prefix := ""
	instancePrefix := "instance_"
	nodePrefix := "node_"
	instanceNetPrefix := "instance_interface_"
	nodeNetPrefix := "node_interface_"
	podPrefix := "pod_"
	podNetPrefix := "pod_interface_"
	containerPrefix := "container_"
	service := "service_"
	cluster := "cluster_"
	namespace := "namespace_"

	switch mType {
	case TypeInstance:
		prefix = instancePrefix
	case TypeInstanceFS:
		prefix = instancePrefix
	case TypeInstanceDiskIO:
		prefix = instancePrefix
	case TypeInstanceNet:
		prefix = instanceNetPrefix
	case TypeNode:
		prefix = nodePrefix
	case TypeNodeFS:
		prefix = nodePrefix
	case TypeNodeDiskIO:
		prefix = nodePrefix
	case TypeNodeNet:
		prefix = nodeNetPrefix
	case TypePod:
		prefix = podPrefix
	case TypePodNet:
		prefix = podNetPrefix
	case TypeContainer:
		prefix = containerPrefix
	case TypeContainerDiskIO:
		prefix = containerPrefix
	case TypeContainerFS:
		prefix = containerPrefix
	case TypeService:
		prefix = service
	case TypeCluster:
		prefix = cluster
	case K8sNamespace:
		prefix = namespace
	default:
		log.Printf("E! Unexpected MetricType: %s", mType)
	}
	return prefix + name
}
