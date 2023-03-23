// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8sdecorator

import (
	. "github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	"github.com/aws/amazon-cloudwatch-agent/internal/structuredlogscommon"
	"github.com/influxdata/telegraf/metric"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestDisableMetricExtraction(t *testing.T) {
	tags := map[string]string{MetricType: TypeCluster}
	fields := map[string]interface{}{MetricName(TypeCluster, NodeCount): 10, MetricName(TypeCluster, FailedNodeCount): 1}

	m1 := metric.New("testClusterMetric", tags, fields, time.Now())
	decorator := &K8sDecorator{
		started:                 true,
		DisableMetricExtraction: false,
		ClusterName:             "TestK8sCluster",
	}
	decorator.Apply(m1)
	assert.Equal(t, "Sources,CloudWatchMetrics", m1.Tags()["attributesInFields"])
	assert.Equal(t, []structuredlogscommon.MetricRule{
		{
			Metrics: []structuredlogscommon.MetricAttr{
				{
					Unit: "Count",
					Name: "cluster_node_count",
				},
				{
					Unit: "Count",
					Name: "cluster_failed_node_count",
				},
			},
			DimensionSets: [][]string{{"ClusterName"}},
			Namespace:     "ContainerInsights",
		},
	}, m1.Fields()["CloudWatchMetrics"])

	m2 := metric.New("testClusterMetric", tags, fields, time.Now())
	decorator = &K8sDecorator{
		started:                 true,
		DisableMetricExtraction: true,
		ClusterName:             "TestK8sCluster",
	}
	decorator.Apply(m2)
	assert.Equal(t, "Sources", m2.Tags()["attributesInFields"])
	assert.Equal(t, nil, m2.Fields()["CloudWatchMetrics"])
}
