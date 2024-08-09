// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsdecorator

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/influxdata/telegraf/metric"
	"github.com/stretchr/testify/assert"

	. "github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	"github.com/aws/amazon-cloudwatch-agent/internal/structuredlogscommon"
)

func TestNodeFull(t *testing.T) {
	tags := map[string]string{MetricType: TypeInstance, InstanceIdKey: "TestEC2InstanceId", ContainerInstanceIdKey: "TestContainerInstanceId", ClusterNameKey: "TestClusterName"}
	fields := map[string]interface{}{MetricName(TypeInstance, CpuUtilization): 0, MetricName(TypeInstance, MemUtilization): 0,
		MetricName(TypeInstance, NetTotalBytes): 0, MetricName(TypeInstance, CpuReservedCapacity): 0, MetricName(TypeInstance, MemReservedCapacity): 0,
		MetricName(TypeInstance, RunningTaskCount): 0, MetricName(TypeInstance, CpuTotal): 0,
		MetricName(TypeInstance, CpuLimit): 0, MetricName(TypeInstance, MemWorkingset): 0, MetricName(TypeInstance, MemLimit): 0}
	m := metric.New("test", tags, fields, time.Now())
	new(ECSDecorator).tagMetricRule(m)
	actual := m.Fields()[structuredlogscommon.MetricRuleKey].([]structuredlogscommon.MetricRule)
	expected := []structuredlogscommon.MetricRule{}
	deepCopy(&expected, nodeMetricRules)
	assert.Equal(t, expected, actual, "Expected to be equal")
}

func TestNodeLackOfCpuUtilization(t *testing.T) {
	tags := map[string]string{MetricType: TypeInstance, InstanceIdKey: "TestEC2InstanceId", ContainerInstanceIdKey: "TestContainerInstanceId", ClusterNameKey: "TestClusterName"}
	fields := map[string]interface{}{MetricName(TypeInstance, MemUtilization): 0,
		MetricName(TypeInstance, NetTotalBytes): 0, MetricName(TypeInstance, CpuReservedCapacity): 0, MetricName(TypeInstance, MemReservedCapacity): 0,
		MetricName(TypeInstance, RunningTaskCount): 0, MetricName(TypeInstance, CpuTotal): 0,
		MetricName(TypeInstance, CpuLimit): 0, MetricName(TypeInstance, MemWorkingset): 0, MetricName(TypeInstance, MemLimit): 0}
	m := metric.New("test", tags, fields, time.Now())
	new(ECSDecorator).tagMetricRule(m)
	actual := m.Fields()[structuredlogscommon.MetricRuleKey].([]structuredlogscommon.MetricRule)

	expected := make([]structuredlogscommon.MetricRule, len(nodeMetricRules))
	deepCopy(&expected, nodeMetricRules)
	deleteMetricFromMetricRules(MetricName(TypeInstance, CpuUtilization), expected)

	assert.Equal(t, expected, actual, "Expected to be equal")
}

func TestNodeLackOfInstanceId(t *testing.T) {
	tags := map[string]string{MetricType: TypeInstance, ContainerInstanceIdKey: "TestContainerInstanceId", ClusterNameKey: "TestClusterName"}
	fields := map[string]interface{}{MetricName(TypeInstance, CpuUtilization): 0, MetricName(TypeInstance, MemUtilization): 0,
		MetricName(TypeInstance, NetTotalBytes): 0, MetricName(TypeInstance, CpuReservedCapacity): 0, MetricName(TypeInstance, MemReservedCapacity): 0,
		MetricName(TypeInstance, RunningTaskCount): 0, MetricName(TypeInstance, CpuTotal): 0,
		MetricName(TypeInstance, CpuLimit): 0, MetricName(TypeInstance, MemWorkingset): 0, MetricName(TypeInstance, MemLimit): 0}
	m := metric.New("test", tags, fields, time.Now())
	new(ECSDecorator).tagMetricRule(m)
	actual := m.Fields()[structuredlogscommon.MetricRuleKey].([]structuredlogscommon.MetricRule)

	expected := make([]structuredlogscommon.MetricRule, len(nodeMetricRules))
	deepCopy(&expected, nodeMetricRules)
	expected = append(expected[:0], expected[1:]...)
	assert.Equal(t, expected, actual, "Expected to be equal")
}

func TestNodeFSFull(t *testing.T) {
	tags := map[string]string{MetricType: TypeInstanceFS, InstanceIdKey: "TestEC2InstanceId", ContainerInstanceIdKey: "TestContainerInstanceId", ClusterNameKey: "TestClusterName"}
	fields := map[string]interface{}{MetricName(TypeInstanceFS, FSUtilization): 0}
	m := metric.New("test", tags, fields, time.Now())
	new(ECSDecorator).tagMetricRule(m)
	actual := m.Fields()[structuredlogscommon.MetricRuleKey].([]structuredlogscommon.MetricRule)

	expected := make([]structuredlogscommon.MetricRule, len(nodeFSMetricRules))
	deepCopy(&expected, nodeFSMetricRules)
	assert.Equal(t, expected, actual, "Expected to be equal")
}

func deleteMetricFromMetricRules(metric string, rules []structuredlogscommon.MetricRule) {
	for i := 0; i < len(rules); i++ {
		rule := rules[i]
		metricAttrs := rule.Metrics
		idx := -1
		for i := 0; i < len(metricAttrs); i++ {
			if metricAttrs[i].Name == metric {
				idx = i
				break
			}
		}
		if idx != -1 {
			metricAttrs = append(metricAttrs[:idx], metricAttrs[idx+1:]...)
			rules[i].Metrics = metricAttrs
		}
	}
}

func deleteDimensionFromMetricRules(dimension string, rules []structuredlogscommon.MetricRule) {
	for i := 0; i < len(rules); i++ {
		rule := rules[i]
		var dimsSet [][]string
	loop:
		for _, dims := range rule.DimensionSets {
			for _, dim := range dims {
				if dim == dimension {
					continue loop
				}
			}
			dimsSet = append(dimsSet, dims)
		}
		rules[i].DimensionSets = dimsSet
	}
}

func deepCopy(dst interface{}, src interface{}) error {
	if dst == nil {
		return fmt.Errorf("dst cannot be nil")
	}
	if src == nil {
		return fmt.Errorf("src cannot be nil")
	}
	bytes, err := json.Marshal(src)
	if err != nil {
		return fmt.Errorf("Unable to marshal src: %s", err)
	}
	err = json.Unmarshal(bytes, dst)
	if err != nil {
		return fmt.Errorf("Unable to unmarshal into dst: %s", err)
	}
	return nil
}
