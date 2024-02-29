// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsdecorator

import (
	"testing"
	"time"

	"github.com/influxdata/telegraf/metric"
	"github.com/stretchr/testify/assert"

	. "github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	"github.com/aws/amazon-cloudwatch-agent/internal/logscommon"
)

func TestTagMetricSourceForTypeInstance(t *testing.T) {
	tags := map[string]string{MetricType: TypeInstance, InstanceIdKey: "TestEC2InstanceId", ContainerInstanceIdKey: "TestContainerInstanceId", ClusterNameKey: "TestClusterName"}
	fields := map[string]interface{}{MetricName(TypeInstance, CpuUtilization): 0, MetricName(TypeInstance, MemUtilization): 0,
		MetricName(TypeInstance, NetTotalBytes): 0, MetricName(TypeInstance, CpuReservedCapacity): 0, MetricName(TypeInstance, MemReservedCapacity): 0,
		MetricName(TypeInstance, RunningTaskCount): 0, MetricName(TypeInstance, CpuTotal): 0,
		MetricName(TypeInstance, CpuLimit): 0, MetricName(TypeInstance, MemWorkingset): 0, MetricName(TypeInstance, MemLimit): 0}

	m := metric.New("test", tags, fields, time.Now())
	new(ECSDecorator).tagMetricSource(m, tags)

	assert.Equal(t, "Sources", m.Tags()["attributesInFields"], "Expected to be equal")
	assert.Equal(t, []string{"cadvisor", "/proc", "ecsagent", "calculated"}, m.Fields()[SourcesKey], "Expected to be equal")
}

func TestTagMetricSourceForTypeInstanceFS(t *testing.T) {
	tags := map[string]string{MetricType: TypeInstanceFS, InstanceIdKey: "TestEC2InstanceId", ContainerInstanceIdKey: "TestContainerInstanceId", ClusterNameKey: "TestClusterName"}
	fields := map[string]interface{}{MetricName(TypeInstance, CpuUtilization): 0, MetricName(TypeInstance, MemUtilization): 0,
		MetricName(TypeInstance, NetTotalBytes): 0, MetricName(TypeInstance, CpuReservedCapacity): 0, MetricName(TypeInstance, MemReservedCapacity): 0,
		MetricName(TypeInstance, RunningTaskCount): 0, MetricName(TypeInstance, CpuTotal): 0,
		MetricName(TypeInstance, CpuLimit): 0, MetricName(TypeInstance, MemWorkingset): 0, MetricName(TypeInstance, MemLimit): 0}

	m := metric.New("test", tags, fields, time.Now())
	new(ECSDecorator).tagMetricSource(m, tags)

	assert.Equal(t, "Sources", m.Tags()["attributesInFields"], "Expected to be equal")
	assert.Equal(t, []string{"cadvisor", "calculated"}, m.Fields()[SourcesKey], "Expected to be equal")
}

func TestTagMetricSourceForTypeInstanceNet(t *testing.T) {
	tags := map[string]string{MetricType: TypeInstanceNet, InstanceIdKey: "TestEC2InstanceId", ContainerInstanceIdKey: "TestContainerInstanceId", ClusterNameKey: "TestClusterName"}
	fields := map[string]interface{}{MetricName(TypeInstance, CpuUtilization): 0, MetricName(TypeInstance, MemUtilization): 0,
		MetricName(TypeInstance, NetTotalBytes): 0, MetricName(TypeInstance, CpuReservedCapacity): 0, MetricName(TypeInstance, MemReservedCapacity): 0,
		MetricName(TypeInstance, RunningTaskCount): 0, MetricName(TypeInstance, CpuTotal): 0,
		MetricName(TypeInstance, CpuLimit): 0, MetricName(TypeInstance, MemWorkingset): 0, MetricName(TypeInstance, MemLimit): 0}

	m := metric.New("test", tags, fields, time.Now())
	new(ECSDecorator).tagMetricSource(m, tags)

	assert.Equal(t, "Sources", m.Tags()["attributesInFields"], "Expected to be equal")
	assert.Equal(t, []string{"cadvisor", "calculated"}, m.Fields()[SourcesKey], "Expected to be equal")
}

func TestTagMetricSourceForTypeInstanceDiskIO(t *testing.T) {
	tags := map[string]string{MetricType: TypeInstanceDiskIO, InstanceIdKey: "TestEC2InstanceId", ContainerInstanceIdKey: "TestContainerInstanceId", ClusterNameKey: "TestClusterName"}
	fields := map[string]interface{}{MetricName(TypeInstance, CpuUtilization): 0, MetricName(TypeInstance, MemUtilization): 0,
		MetricName(TypeInstance, NetTotalBytes): 0, MetricName(TypeInstance, CpuReservedCapacity): 0, MetricName(TypeInstance, MemReservedCapacity): 0,
		MetricName(TypeInstance, RunningTaskCount): 0, MetricName(TypeInstance, CpuTotal): 0,
		MetricName(TypeInstance, CpuLimit): 0, MetricName(TypeInstance, MemWorkingset): 0, MetricName(TypeInstance, MemLimit): 0}

	m := metric.New("test", tags, fields, time.Now())
	new(ECSDecorator).tagMetricSource(m, tags)

	assert.Equal(t, "Sources", m.Tags()["attributesInFields"], "Expected to be equal")
	assert.Equal(t, []string{"cadvisor"}, m.Fields()[SourcesKey], "Expected to be equal")
}

func TestTagLogGroup(t *testing.T) {
	tags := map[string]string{MetricType: TypeInstance, InstanceIdKey: "TestEC2InstanceId", ContainerInstanceIdKey: "TestContainerInstanceId", ClusterNameKey: "TestClusterName"}
	fields := map[string]interface{}{MetricName(TypeInstance, CpuUtilization): 0, MetricName(TypeInstance, MemUtilization): 0,
		MetricName(TypeInstance, NetTotalBytes): 0, MetricName(TypeInstance, CpuReservedCapacity): 0, MetricName(TypeInstance, MemReservedCapacity): 0,
		MetricName(TypeInstance, RunningTaskCount): 0, MetricName(TypeInstance, CpuTotal): 0,
		MetricName(TypeInstance, CpuLimit): 0, MetricName(TypeInstance, MemWorkingset): 0, MetricName(TypeInstance, MemLimit): 0}

	m := metric.New("test", tags, fields, time.Now())
	new(ECSDecorator).tagLogGroup(m, tags)

	assert.Equal(t, "/aws/ecs/containerinsights/TestClusterName/performance", m.Tags()[logscommon.LogGroupNameTag], "Expected to be equal")

}

func TestDecorateCpu(t *testing.T) {
	tags := map[string]string{MetricType: TypeInstance, ClusterNameKey: "TestClusterName"}
	fields := map[string]interface{}{MetricName(TypeInstance, CpuUtilization): 0, MetricName(TypeInstance, MemUtilization): 0,
		MetricName(TypeInstance, NetTotalBytes): 0, MetricName(TypeInstance, CpuReservedCapacity): 0, MetricName(TypeInstance, MemReservedCapacity): 0,
		MetricName(TypeInstance, RunningTaskCount): 0, MetricName(TypeInstance, CpuTotal): 1.0,
		MetricName(TypeInstance, CpuLimit): 0, MetricName(TypeInstance, MemWorkingset): 0, MetricName(TypeInstance, MemLimit): 0}

	m := metric.New("test", tags, fields, time.Now())
	(&ECSDecorator{ecsInfo: &ecsInfo{cpuReserved: 1024}, NodeCapacity: &NodeCapacity{CPUCapacity: 8}}).decorateCPU(m, fields)

	assert.Equal(t, 0.0125, m.Fields()[MetricName(TypeInstance, CpuUtilization)], "Expected to be equal")
	assert.Equal(t, 12.5, m.Fields()[MetricName(TypeInstance, CpuReservedCapacity)], "Expected to be equal")
}

func TestDecorateMem(t *testing.T) {
	tags := map[string]string{MetricType: TypeInstance, ClusterNameKey: "TestClusterName"}
	fields := map[string]interface{}{MetricName(TypeInstance, CpuUtilization): 0, MetricName(TypeInstance, MemUtilization): 0,
		MetricName(TypeInstance, NetTotalBytes): 0, MetricName(TypeInstance, CpuReservedCapacity): 0, MetricName(TypeInstance, MemReservedCapacity): 0,
		MetricName(TypeInstance, RunningTaskCount): 0, MetricName(TypeInstance, CpuTotal): 1.0,
		MetricName(TypeInstance, CpuLimit): 0, MetricName(TypeInstance, MemWorkingset): uint64(1), MetricName(TypeInstance, MemLimit): 0}

	m := metric.New("test", tags, fields, time.Now())
	(&ECSDecorator{ecsInfo: &ecsInfo{memReserved: 1}, NodeCapacity: &NodeCapacity{MemCapacity: 8}}).decorateMem(m, fields)

	assert.Equal(t, 12.5, m.Fields()[MetricName(TypeInstance, MemUtilization)], "Expected to be equal")
	assert.Equal(t, 12.5, m.Fields()[MetricName(TypeInstance, MemReservedCapacity)], "Expected to be equal")
}

func TestDecorateTaskCount(t *testing.T) {
	tags := map[string]string{MetricType: TypeInstance, ClusterNameKey: "TestClusterName"}
	fields := map[string]interface{}{MetricName(TypeInstance, CpuUtilization): 0, MetricName(TypeInstance, MemUtilization): 0,
		MetricName(TypeInstance, NetTotalBytes): 0, MetricName(TypeInstance, CpuReservedCapacity): 0, MetricName(TypeInstance, MemReservedCapacity): 0,
		MetricName(TypeInstance, RunningTaskCount): 0, MetricName(TypeInstance, CpuTotal): 1.0,
		MetricName(TypeInstance, CpuLimit): 0, MetricName(TypeInstance, MemWorkingset): uint64(1), MetricName(TypeInstance, MemLimit): 0}

	m := metric.New("test", tags, fields, time.Now())
	(&ECSDecorator{ecsInfo: &ecsInfo{runningTaskCount: 5}}).decorateTaskCount(m, tags)

	assert.Equal(t, int64(5), m.Fields()[MetricName(TypeInstance, RunningTaskCount)], "Expected to be equal")

}
