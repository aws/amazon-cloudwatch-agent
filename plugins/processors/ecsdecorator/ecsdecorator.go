// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsdecorator

import (
	"fmt"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/processors"

	. "github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	"github.com/aws/amazon-cloudwatch-agent/internal/logscommon"
	"github.com/aws/amazon-cloudwatch-agent/internal/structuredlogscommon"
)

const (
	cloudwatchNamespace = "ECS/ContainerInsights"
)

type ECSDecorator struct {
	HostIP  string `toml:"host_ip"`
	ecsInfo *ecsInfo
	*NodeCapacity
}

func (e *ECSDecorator) Description() string {
	return "Decorate metrics collected by cadivisor with ecs metadata"
}

var sampleConfig = `
  ## ecs ec2 node private ip
  host_ip = "10.13.14.15"
`

func (e *ECSDecorator) SampleConfig() string {
	return sampleConfig
}

func (e *ECSDecorator) Init() error {
	e.ecsInfo = newECSInfo(e.HostIP)
	if e.ecsInfo.clusterName == "" {
		return fmt.Errorf("ECSDecorator failed to get cluster name of ecs")
	}
	return nil
}

func (e *ECSDecorator) Stop() {
	e.ecsInfo.shutdown()
}

func (e *ECSDecorator) Apply(in ...telegraf.Metric) []telegraf.Metric {
	var out []telegraf.Metric

	for _, metric := range in {
		metric.AddTag(ClusterNameKey, e.ecsInfo.clusterName)
		tags := metric.Tags()
		fields := metric.Fields()

		e.tagLogGroup(metric, tags)
		e.tagLogStream(metric)
		e.tagContainerInstanceId(metric)
		e.tagMetricSource(metric, tags)
		e.tagVersion(metric)

		e.decorateCPU(metric, fields)
		e.decorateMem(metric, fields)
		e.decorateTaskCount(metric, tags)
		e.tagMetricRule(metric)
		out = append(out, metric)
	}

	return out
}

func (e *ECSDecorator) tagLogGroup(metric telegraf.Metric, tags map[string]string) {
	logGroup := fmt.Sprintf("/aws/ecs/containerinsights/%s/performance", tags[ClusterNameKey])
	metric.AddTag(logscommon.LogGroupNameTag, logGroup)
}

func (e *ECSDecorator) tagLogStream(metric telegraf.Metric) {
	logStream := fmt.Sprintf("NodeTelemetry-%s", e.ecsInfo.containerInstanceId)
	metric.AddTag(logscommon.LogStreamNameTag, logStream)
}

func (e *ECSDecorator) tagContainerInstanceId(metric telegraf.Metric) {
	metric.AddTag(ContainerInstanceIdKey, e.ecsInfo.containerInstanceId)
}

func (e *ECSDecorator) decorateCPU(metric telegraf.Metric, fields map[string]interface{}) {
	if cpuTotal, ok := fields[MetricName(TypeInstance, CpuTotal)]; ok && e.CPUCapacity > 0 {
		metric.AddField(MetricName(TypeInstance, CpuLimit), e.getCPUCapacityInCadvisorStandard())
		metric.AddField(MetricName(TypeInstance, CpuUtilization), cpuTotal.(float64)/float64(e.getCPUCapacityInCadvisorStandard())*100)
		metric.AddField(MetricName(TypeInstance, CpuReservedCapacity), float64(e.ecsInfo.getCpuReserved())/float64(e.getCPUCapacityInCgroupStandard())*100)
	}
}

func (e *ECSDecorator) decorateMem(metric telegraf.Metric, fields map[string]interface{}) {
	if memWorkingset, ok := fields[MetricName(TypeInstance, MemWorkingset)]; ok && e.getMemCapacity() > 0 {
		metric.AddField(MetricName(TypeInstance, MemLimit), e.getMemCapacity())
		metric.AddField(MetricName(TypeInstance, MemUtilization), float64(memWorkingset.(uint64))/float64(e.getMemCapacity())*100)
		metric.AddField(MetricName(TypeInstance, MemReservedCapacity), float64(e.ecsInfo.getMemReserved())/float64(e.getMemCapacity())*100)
	}
}

func (e *ECSDecorator) decorateTaskCount(metric telegraf.Metric, tags map[string]string) {
	if metricType := tags[MetricType]; metricType == TypeInstance {
		metric.AddField(MetricName(TypeInstance, RunningTaskCount), e.ecsInfo.getRunningTaskCount())
	}
}

func (e *ECSDecorator) tagMetricRule(metric telegraf.Metric) {
	rules, ok := staticMetricRule[metric.Tags()[MetricType]]
	if !ok {
		return
	}
	structuredlogscommon.AttachMetricRule(metric, rules)
}

func (e *ECSDecorator) tagMetricSource(metric telegraf.Metric, tags map[string]string) {
	metricType, ok := tags[MetricType]
	if !ok {
		return
	}

	var sources []string
	switch metricType {
	case TypeInstance:
		sources = append(sources, []string{"cadvisor", "/proc", "ecsagent", "calculated"}...)
	case TypeInstanceFS:
		sources = append(sources, []string{"cadvisor", "calculated"}...)
	case TypeInstanceNet:
		sources = append(sources, []string{"cadvisor", "calculated"}...)
	case TypeInstanceDiskIO:
		sources = append(sources, []string{"cadvisor"}...)
	}

	if len(sources) > 0 {
		structuredlogscommon.AppendAttributesInFields(SourcesKey, sources, metric)
	}
}

func (e *ECSDecorator) tagVersion(metric telegraf.Metric) {
	structuredlogscommon.AddVersion(metric)
}

func (e *ECSDecorator) getCPUCapacityInCadvisorStandard() int64 {
	// cadvisor treat 1 core as 1000 millicores
	return e.CPUCapacity * 1000
}

func (e *ECSDecorator) getCPUCapacityInCgroupStandard() int64 {
	// cgroup treat one core as 1024 cpu unit
	return e.CPUCapacity * 1024
}

func (e *ECSDecorator) getMemCapacity() int64 {
	return e.MemCapacity
}

func init() {
	processors.Add("ecsdecorator", func() telegraf.Processor {
		return &ECSDecorator{NodeCapacity: NewNodeCapacity()}
	})
}
