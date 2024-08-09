// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsdecorator

import (
	. "github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	"github.com/aws/amazon-cloudwatch-agent/internal/structuredlogscommon"
)

const (
	Bytes       = "Bytes"
	BytesPerSec = "Bytes/Second"
	Count       = "Count"
	Percent     = "Percent"
)

var nodeMetricRules = []structuredlogscommon.MetricRule{
	{
		Metrics: []structuredlogscommon.MetricAttr{
			{Unit: Percent, Name: MetricName(TypeInstance, CpuUtilization)},
			{Unit: Percent, Name: MetricName(TypeInstance, CpuReservedCapacity)},
			{Unit: Percent, Name: MetricName(TypeInstance, MemUtilization)},
			{Unit: Percent, Name: MetricName(TypeInstance, MemReservedCapacity)},
			{Unit: BytesPerSec, Name: MetricName(TypeInstance, NetTotalBytes)},
			{Unit: Count, Name: MetricName(TypeInstance, RunningTaskCount)}},
		DimensionSets: [][]string{{ContainerInstanceIdKey, InstanceIdKey, ClusterNameKey}},
		Namespace:     cloudwatchNamespace,
	},
	{
		Metrics: []structuredlogscommon.MetricAttr{
			{Unit: Percent, Name: MetricName(TypeInstance, CpuUtilization)},
			{Unit: Percent, Name: MetricName(TypeInstance, MemUtilization)},
			{Unit: BytesPerSec, Name: MetricName(TypeInstance, NetTotalBytes)},
			{Unit: Percent, Name: MetricName(TypeInstance, CpuReservedCapacity)},
			{Unit: Percent, Name: MetricName(TypeInstance, MemReservedCapacity)},
			{Unit: Count, Name: MetricName(TypeInstance, RunningTaskCount)},
			{Name: MetricName(TypeInstance, CpuTotal)},
			{Name: MetricName(TypeInstance, CpuLimit)},
			{Unit: Bytes, Name: MetricName(TypeInstance, MemWorkingset)},
			{Unit: Bytes, Name: MetricName(TypeInstance, MemLimit)}},
		DimensionSets: [][]string{{ClusterNameKey}},
		Namespace:     cloudwatchNamespace,
	},
}

var nodeFSMetricRules = []structuredlogscommon.MetricRule{
	{
		Metrics: []structuredlogscommon.MetricAttr{
			{Unit: Percent, Name: MetricName(TypeInstanceFS, FSUtilization)}},
		DimensionSets: [][]string{{ContainerInstanceIdKey, InstanceIdKey, ClusterNameKey}, {ClusterNameKey}},
		Namespace:     cloudwatchNamespace,
	},
}

var staticMetricRule = map[string][]structuredlogscommon.MetricRule{
	TypeInstance:   nodeMetricRules,
	TypeInstanceFS: nodeFSMetricRules,
}
