// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package containerinsightscommon

import (
	"log"
)

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
	case TypeInstance, TypeInstanceFS, TypeInstanceDiskIO:
		prefix = instancePrefix
	case TypeInstanceNet:
		prefix = instanceNetPrefix
	case TypeNode, TypeNodeFS, TypeNodeDiskIO, TypeGpuNode:
		prefix = nodePrefix
	case TypeNodeNet:
		prefix = nodeNetPrefix
	case TypePod, TypeGpuPod:
		prefix = podPrefix
	case TypePodNet:
		prefix = podNetPrefix
	case TypeContainer, TypeContainerDiskIO, TypeContainerFS, TypeGpuContainer:
		prefix = containerPrefix
	case TypeService:
		prefix = service
	case TypeCluster, TypeGpuCluster, TypeClusterQueue:
		prefix = cluster
	case K8sNamespace:
		prefix = namespace
	default:
		log.Printf("E! Unexpected MetricType: %s", mType)
	}
	return prefix + name
}
