// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cadvisor

import (
	"fmt"
	. "github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	"github.com/aws/amazon-cloudwatch-agent/plugins/inputs/cadvisor/extractors"
)

func mergeMetrics(metrics []*extractors.CAdvisorMetric) []*extractors.CAdvisorMetric {
	var result []*extractors.CAdvisorMetric
	metricMap := make(map[string]*extractors.CAdvisorMetric)
	for _, metric := range metrics {
		if metricKey := getMetricKey(metric); metricKey != "" {
			if mergedMetric, ok := metricMap[metricKey]; ok {
				mergedMetric.Merge(metric)
			} else {
				metricMap[metricKey] = metric
			}
		} else {
			// this metric cannot be merged
			result = append(result, metric)
		}
	}
	for _, metric := range metricMap {
		result = append(result, metric)
	}
	return result
}

// return MetricKey for merge-able metrics
func getMetricKey(metric *extractors.CAdvisorMetric) string {
	metricType := metric.GetMetricType()
	metricKey := ""
	switch metricType {
	case TypeInstance:
		// merge cpu, memory, net metric for type Instance
		metricKey = fmt.Sprintf("metricType:%s", TypeInstance)
	case TypeNode:
		// merge cpu, memory, net metric for type Node
		metricKey = fmt.Sprintf("metricType:%s", TypeNode)
	case TypePod:
		// merge cpu, memory, net metric for type Pod
		metricKey = fmt.Sprintf("metricType:%s,podId:%s", TypePod, metric.GetTags()[PodIdKey])
	case TypeContainer:
		// merge cpu, memory metric for type Container
		metricKey = fmt.Sprintf("metricType:%s,podId:%s,containerName:%s", TypeContainer, metric.GetTags()[PodIdKey], metric.GetTags()[ContainerNamekey])
	case TypeInstanceDiskIO:
		// merge io_serviced, io_service_bytes for type InstanceDiskIO
		metricKey = fmt.Sprintf("metricType:%s,device:%s", TypeInstanceDiskIO, metric.GetTags()[DiskDev])
	case TypeNodeDiskIO:
		// merge io_serviced, io_service_bytes for type NodeDiskIO
		metricKey = fmt.Sprintf("metricType:%s,device:%s", TypeNodeDiskIO, metric.GetTags()[DiskDev])
	default:
		metricKey = ""
	}
	return metricKey
}
