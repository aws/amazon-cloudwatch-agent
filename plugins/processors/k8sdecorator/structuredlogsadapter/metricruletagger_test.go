// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package structuredlogsadapter

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
	tags := map[string]string{MetricType: TypeNode, NodeNameKey: "TestNodeName", ClusterNameKey: "TestClusterName", InstanceIdKey: "i-123"}
	fields := map[string]interface{}{MetricName(TypeNode, CpuUtilization): 0, MetricName(TypeNode, MemUtilization): 0,
		MetricName(TypeNode, NetTotalBytes): 0, MetricName(TypeNode, CpuReservedCapacity): 0, MetricName(TypeNode, MemReservedCapacity): 0,
		MetricName(TypeNode, RunningPodCount): 0, MetricName(TypeNode, RunningContainerCount): 0, MetricName(TypeNode, CpuTotal): 0,
		MetricName(TypeNode, CpuLimit): 0, MetricName(TypeNode, MemWorkingset): 0, MetricName(TypeNode, MemLimit): 0}
	m := metric.New("test", tags, fields, time.Now())
	TagMetricRule(m)
	actual := m.Fields()[structuredlogscommon.MetricRuleKey].([]structuredlogscommon.MetricRule)
	expected := []structuredlogscommon.MetricRule{}
	deepCopy(&expected, nodeMetricRules)
	assert.Equal(t, expected, actual, "Expected to be equal")
}

func TestNodeLackOfCpuUtilization(t *testing.T) {
	tags := map[string]string{MetricType: TypeNode, NodeNameKey: "TestNodeName", ClusterNameKey: "TestClusterName", InstanceIdKey: "i-123"}
	fields := map[string]interface{}{MetricName(TypeNode, MemUtilization): 0,
		MetricName(TypeNode, NetTotalBytes): 0, MetricName(TypeNode, CpuReservedCapacity): 0, MetricName(TypeNode, MemReservedCapacity): 0,
		MetricName(TypeNode, RunningPodCount): 0, MetricName(TypeNode, RunningContainerCount): 0, MetricName(TypeNode, CpuTotal): 0,
		MetricName(TypeNode, CpuLimit): 0, MetricName(TypeNode, MemWorkingset): 0, MetricName(TypeNode, MemLimit): 0}
	m := metric.New("test", tags, fields, time.Now())
	TagMetricRule(m)
	actual := m.Fields()[structuredlogscommon.MetricRuleKey].([]structuredlogscommon.MetricRule)

	expected := make([]structuredlogscommon.MetricRule, len(nodeMetricRules))
	deepCopy(&expected, nodeMetricRules)
	deleteMetricFromMetricRules(MetricName(TypeNode, CpuUtilization), expected)

	assert.Equal(t, expected, actual, "Expected to be equal")
}

func TestNodeLackOfNodeNameKey(t *testing.T) {
	tags := map[string]string{MetricType: TypeNode, ClusterNameKey: "TestClusterName", InstanceIdKey: "i-123"}
	fields := map[string]interface{}{MetricName(TypeNode, CpuUtilization): 0, MetricName(TypeNode, MemUtilization): 0,
		MetricName(TypeNode, NetTotalBytes): 0, MetricName(TypeNode, CpuReservedCapacity): 0, MetricName(TypeNode, MemReservedCapacity): 0,
		MetricName(TypeNode, RunningPodCount): 0, MetricName(TypeNode, RunningContainerCount): 0, MetricName(TypeNode, CpuTotal): 0,
		MetricName(TypeNode, CpuLimit): 0, MetricName(TypeNode, MemWorkingset): 0, MetricName(TypeNode, MemLimit): 0}
	m := metric.New("test", tags, fields, time.Now())
	TagMetricRule(m)
	actual := m.Fields()[structuredlogscommon.MetricRuleKey].([]structuredlogscommon.MetricRule)

	expected := make([]structuredlogscommon.MetricRule, len(nodeMetricRules))
	deepCopy(&expected, nodeMetricRules)
	expected = append(expected[:0], expected[1:]...)
	assert.Equal(t, expected, actual, "Expected to be equal")
}

func TestPodFull(t *testing.T) {
	tags := map[string]string{MetricType: TypePod, PodNameKey: "TestPodName", ClusterNameKey: "TestClusterName", TypeService: "TestServiceName", K8sNamespace: "TestNamespace"}
	fields := map[string]interface{}{MetricName(TypePod, CpuUtilization): 0, MetricName(TypePod, MemUtilization): 0,
		MetricName(TypePod, NetRxBytes): 0, MetricName(TypePod, NetTxBytes): 0, MetricName(TypePod, CpuUtilizationOverPodLimit): 0,
		MetricName(TypePod, MemUtilizationOverPodLimit): 0, MetricName(TypePod, CpuReservedCapacity): 0, MetricName(TypePod, MemReservedCapacity): 0, MetricName(TypePod, ContainerRestartCount): 0}
	m := metric.New("test", tags, fields, time.Now())
	TagMetricRule(m)
	actual := m.Fields()[structuredlogscommon.MetricRuleKey].([]structuredlogscommon.MetricRule)
	expected := []structuredlogscommon.MetricRule{}
	deepCopy(&expected, podMetricRules)
	assert.Equal(t, expected, actual, "Expected to be equal")
}

func TestPodFullLackOfService(t *testing.T) {
	tags := map[string]string{MetricType: TypePod, PodNameKey: "TestPodName", ClusterNameKey: "TestClusterName", K8sNamespace: "TestNamespace"}
	fields := map[string]interface{}{MetricName(TypePod, CpuUtilization): 0, MetricName(TypePod, MemUtilization): 0,
		MetricName(TypePod, NetRxBytes): 0, MetricName(TypePod, NetTxBytes): 0, MetricName(TypePod, CpuUtilizationOverPodLimit): 0,
		MetricName(TypePod, MemUtilizationOverPodLimit): 0, MetricName(TypePod, CpuReservedCapacity): 0, MetricName(TypePod, MemReservedCapacity): 0, MetricName(TypePod, ContainerRestartCount): 0}
	m := metric.New("test", tags, fields, time.Now())
	TagMetricRule(m)
	actual := m.Fields()[structuredlogscommon.MetricRuleKey].([]structuredlogscommon.MetricRule)
	expected := []structuredlogscommon.MetricRule{}
	deepCopy(&expected, podMetricRules)
	deleteDimensionFromMetricRules(TypeService, expected)
	assert.Equal(t, expected, actual, "Expected to be equal")
}

func TestNodeFSFull(t *testing.T) {
	tags := map[string]string{MetricType: TypeNodeFS, NodeNameKey: "TestNodeName", ClusterNameKey: "TestClusterName", InstanceIdKey: "i-123"}
	fields := map[string]interface{}{MetricName(TypeNodeFS, FSUtilization): 0}
	m := metric.New("test", tags, fields, time.Now())
	TagMetricRule(m)
	actual := m.Fields()[structuredlogscommon.MetricRuleKey].([]structuredlogscommon.MetricRule)

	expected := make([]structuredlogscommon.MetricRule, len(nodeFSMetricRules))
	deepCopy(&expected, nodeFSMetricRules)
	assert.Equal(t, expected, actual, "Expected to be equal")
}

func TestClusterFull(t *testing.T) {
	tags := map[string]string{MetricType: TypeCluster, ClusterNameKey: "TestClusterName"}
	fields := map[string]interface{}{MetricName(TypeCluster, NodeCount): 0, MetricName(TypeCluster, FailedNodeCount): 0}
	m := metric.New("test", tags, fields, time.Now())
	TagMetricRule(m)
	actual := m.Fields()[structuredlogscommon.MetricRuleKey].([]structuredlogscommon.MetricRule)

	expected := make([]structuredlogscommon.MetricRule, len(clusterMetricRules))
	deepCopy(&expected, clusterMetricRules)
	assert.Equal(t, expected, actual, "Expected to be equal")
}

func TestClusterServiceFull(t *testing.T) {
	tags := map[string]string{MetricType: TypeClusterService, ClusterNameKey: "TestClusterName", TypeService: "TestServiceName", K8sNamespace: "default"}
	fields := map[string]interface{}{MetricName(TypeService, RunningPodCount): 0}
	m := metric.New("test", tags, fields, time.Now())
	TagMetricRule(m)
	actual := m.Fields()[structuredlogscommon.MetricRuleKey].([]structuredlogscommon.MetricRule)

	expected := make([]structuredlogscommon.MetricRule, len(serviceMetricRules))
	deepCopy(&expected, serviceMetricRules)
	assert.Equal(t, expected, actual, "Expected to be equal")
}

func TestClusterNamespaceFull(t *testing.T) {
	tags := map[string]string{MetricType: TypeClusterNamespace, ClusterNameKey: "TestClusterName", K8sNamespace: "TestNamespace"}
	fields := map[string]interface{}{MetricName(K8sNamespace, RunningPodCount): 0}
	m := metric.New("test", tags, fields, time.Now())
	TagMetricRule(m)
	actual := m.Fields()[structuredlogscommon.MetricRuleKey].([]structuredlogscommon.MetricRule)

	expected := make([]structuredlogscommon.MetricRule, len(namespaceMetricRules))
	deepCopy(&expected, namespaceMetricRules)
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
