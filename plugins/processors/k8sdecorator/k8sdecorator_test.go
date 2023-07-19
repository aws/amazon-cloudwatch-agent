// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8sdecorator

import (
	"testing"
	"time"

	"github.com/influxdata/telegraf/metric"
	"github.com/stretchr/testify/assert"

	. "github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	"github.com/aws/amazon-cloudwatch-agent/internal/structuredlogscommon"
)

func TestDisableMetricExtraction(t *testing.T) {

	tags := map[string]string{MetricType: TypeCluster}
	fields := map[string]interface{}{MetricName(TypeCluster, NodeCount): 10, MetricName(TypeCluster, FailedNodeCount): 1}

	testCases := map[string]struct {
		k8sDecorator               *K8sDecorator
		expectedAttributesInFields string
		expectedCloudWatchMetrics  interface{}
	}{
		"WithDisableMetricExtractionDefault": {
			k8sDecorator: &K8sDecorator{
				started:     true,
				ClusterName: "TestK8sCluster",
			},
			expectedAttributesInFields: "Sources,CloudWatchMetrics",
			expectedCloudWatchMetrics: []structuredlogscommon.MetricRule{
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
			},
		},
		"WithDisableMetricExtractionFalse": {
			k8sDecorator: &K8sDecorator{
				started:                 true,
				DisableMetricExtraction: false,
				ClusterName:             "TestK8sCluster",
			},
			expectedAttributesInFields: "Sources,CloudWatchMetrics",
			expectedCloudWatchMetrics: []structuredlogscommon.MetricRule{
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
			},
		},
		"WithDisableMetricExtractionTrue": {
			k8sDecorator: &K8sDecorator{
				started:                 true,
				DisableMetricExtraction: true,
				ClusterName:             "TestK8sCluster",
			},
			expectedAttributesInFields: "Sources",
			expectedCloudWatchMetrics:  nil,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Given a metric & k8s decorator configuration,
			testMetric := metric.New("testClusterMetric", tags, fields, time.Now())
			// When the processor is applied,
			testCase.k8sDecorator.Apply(testMetric)
			// Then the metric is expected to be converted to EMF with the following fields & tags
			assert.Equal(t, testCase.expectedAttributesInFields, testMetric.Tags()["attributesInFields"])
			assert.Equal(t, testCase.expectedCloudWatchMetrics, testMetric.Fields()["CloudWatchMetrics"])
		})
	}
}
