// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package structuredlogsadapter

import (
	"github.com/influxdata/telegraf"

	. "github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	"github.com/aws/amazon-cloudwatch-agent/internal/structuredlogscommon"
)

const (
	cloudwatchNamespace = "ContainerInsights"
	Bytes               = "Bytes"
	BytesPerSec         = "Bytes/Second"
	Count               = "Count"
	Percent             = "Percent"
)

var nodeMetricRules = []structuredlogscommon.MetricRule{
	{
		Metrics: []structuredlogscommon.MetricAttr{
			{Unit: Percent, Name: MetricName(TypeNode, CpuUtilization)},
			{Unit: Percent, Name: MetricName(TypeNode, MemUtilization)},
			{Unit: BytesPerSec, Name: MetricName(TypeNode, NetTotalBytes)},
			{Unit: Percent, Name: MetricName(TypeNode, CpuReservedCapacity)},
			{Unit: Percent, Name: MetricName(TypeNode, MemReservedCapacity)},
			{Unit: Count, Name: MetricName(TypeNode, RunningPodCount)},
			{Unit: Count, Name: MetricName(TypeNode, RunningContainerCount)}},
		DimensionSets: [][]string{{NodeNameKey, InstanceIdKey, ClusterNameKey}},
		Namespace:     cloudwatchNamespace,
	},
	{
		Metrics: []structuredlogscommon.MetricAttr{
			{Unit: Percent, Name: MetricName(TypeNode, CpuUtilization)},
			{Unit: Percent, Name: MetricName(TypeNode, MemUtilization)},
			{Unit: BytesPerSec, Name: MetricName(TypeNode, NetTotalBytes)},
			{Unit: Percent, Name: MetricName(TypeNode, CpuReservedCapacity)},
			{Unit: Percent, Name: MetricName(TypeNode, MemReservedCapacity)},
			{Unit: Count, Name: MetricName(TypeNode, RunningPodCount)},
			{Unit: Count, Name: MetricName(TypeNode, RunningContainerCount)},
			{Name: MetricName(TypeNode, CpuTotal)},
			{Name: MetricName(TypeNode, CpuLimit)},
			{Unit: Bytes, Name: MetricName(TypeNode, MemWorkingset)},
			{Unit: Bytes, Name: MetricName(TypeNode, MemLimit)}},
		DimensionSets: [][]string{{ClusterNameKey}},
		Namespace:     cloudwatchNamespace,
	},
}

var podMetricRules = []structuredlogscommon.MetricRule{
	{
		Metrics: []structuredlogscommon.MetricAttr{
			{Unit: Percent, Name: MetricName(TypePod, CpuUtilization)},
			{Unit: Percent, Name: MetricName(TypePod, MemUtilization)},
			{Unit: BytesPerSec, Name: MetricName(TypePod, NetRxBytes)},
			{Unit: BytesPerSec, Name: MetricName(TypePod, NetTxBytes)},
			{Unit: Percent, Name: MetricName(TypePod, CpuUtilizationOverPodLimit)},
			{Unit: Percent, Name: MetricName(TypePod, MemUtilizationOverPodLimit)}},
		DimensionSets: [][]string{{PodNameKey, K8sNamespace, ClusterNameKey}, {TypeService, K8sNamespace, ClusterNameKey}, {K8sNamespace, ClusterNameKey}, {ClusterNameKey}},
		Namespace:     cloudwatchNamespace,
	},
	{
		Metrics: []structuredlogscommon.MetricAttr{
			{Unit: Percent, Name: MetricName(TypePod, CpuReservedCapacity)},
			{Unit: Percent, Name: MetricName(TypePod, MemReservedCapacity)}},
		DimensionSets: [][]string{{PodNameKey, K8sNamespace, ClusterNameKey}, {ClusterNameKey}},
		Namespace:     cloudwatchNamespace,
	},
	{
		Metrics: []structuredlogscommon.MetricAttr{
			{Unit: Count, Name: MetricName(TypePod, ContainerRestartCount)}},
		DimensionSets: [][]string{{PodNameKey, K8sNamespace, ClusterNameKey}},
		Namespace:     cloudwatchNamespace,
	},
}

var nodeFSMetricRules = []structuredlogscommon.MetricRule{
	{
		Metrics: []structuredlogscommon.MetricAttr{
			{Unit: Percent, Name: MetricName(TypeNodeFS, FSUtilization)}},
		DimensionSets: [][]string{{NodeNameKey, InstanceIdKey, ClusterNameKey}, {ClusterNameKey}},
		Namespace:     cloudwatchNamespace,
	},
}

var clusterMetricRules = []structuredlogscommon.MetricRule{
	{
		Metrics: []structuredlogscommon.MetricAttr{
			{Unit: Count, Name: MetricName(TypeCluster, NodeCount)},
			{Unit: Count, Name: MetricName(TypeCluster, FailedNodeCount)}},
		DimensionSets: [][]string{{ClusterNameKey}},
		Namespace:     cloudwatchNamespace,
	},
}

var serviceMetricRules = []structuredlogscommon.MetricRule{
	{
		Metrics: []structuredlogscommon.MetricAttr{
			{Unit: Count, Name: MetricName(TypeService, RunningPodCount)}},
		DimensionSets: [][]string{{TypeService, K8sNamespace, ClusterNameKey}, {ClusterNameKey}},
		Namespace:     cloudwatchNamespace,
	},
}

var namespaceMetricRules = []structuredlogscommon.MetricRule{
	{
		Metrics: []structuredlogscommon.MetricAttr{
			{Unit: Count, Name: MetricName(K8sNamespace, RunningPodCount)}},
		DimensionSets: [][]string{{K8sNamespace, ClusterNameKey}, {ClusterNameKey}},
		Namespace:     cloudwatchNamespace,
	},
}

var staticMetricRule = map[string][]structuredlogscommon.MetricRule{
	TypeCluster:          clusterMetricRules,
	TypeClusterService:   serviceMetricRules,
	TypeClusterNamespace: namespaceMetricRules,
	TypeNode:             nodeMetricRules,
	TypePod:              podMetricRules,
	TypeNodeFS:           nodeFSMetricRules,
}

func TagMetricRule(metric telegraf.Metric) {
	rules, ok := staticMetricRule[metric.Tags()[MetricType]]
	if !ok {
		return
	}
	structuredlogscommon.AttachMetricRule(metric, rules)
}
