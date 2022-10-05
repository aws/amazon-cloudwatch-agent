// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package emfProcessor

import (
	"reflect"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/internal/structuredlogscommon"
)

func buildTestMetricDeclaration() *metricDeclaration {
	md := metricDeclaration{
		SourceLabels:    []string{"tagA", "tagB"},
		LabelMatcher:    "^v1;v2$",
		MetricSelectors: []string{"metric_a", "metric_b"},
		Dimensions:      [][]string{{"tagB", "tagA"}, {"tagA"}, {"tag2", "tag1"}},
	}
	md.init()
	return &md
}

func Test_metricDeclaration_init(t *testing.T) {
	md := buildTestMetricDeclaration()
	assert.Equal(t, ";", md.LabelSeparator)
	assert.Equal(t, [][]string{{"tagA", "tagB"}, {"tagA"}, {"tag1", "tag2"}}, md.Dimensions)
}

func Test_getConcatenatedLabels_complete(t *testing.T) {
	md := buildTestMetricDeclaration()

	metricTags := map[string]string{"tagA": "valueA", "tagB": "valueB"}
	result := md.getConcatenatedLabels(metricTags)

	assert.Equal(t, "valueA;valueB", result)
}

func Test_getConcatenatedLabels_incomplete(t *testing.T) {
	md := buildTestMetricDeclaration()

	metricTags := map[string]string{"tagC": "valueA", "tagD": "valueB"}
	result := md.getConcatenatedLabels(metricTags)

	assert.Equal(t, ";", result)
}

func Test_process_match(t *testing.T) {
	md := buildTestMetricDeclaration()

	metricTags := map[string]string{"tagA": "v1", "tagB": "v2"}
	metricFields := map[string]interface{}{"metric_a": "valueA", "metric_c": 10.0}
	namespace := "ContainerInsights/Prometheus"
	metricUnit := map[string]string{"metric_a": "Count"}

	result := md.process(metricTags, metricFields, namespace, metricUnit)
	assert.Equal(t, namespace, result.Namespace)
	assert.True(t, reflect.DeepEqual([][]string{{"tagA", "tagB"}, {"tagA"}, {"tag1", "tag2"}}, result.DimensionSets))
	assert.True(t, reflect.DeepEqual([]structuredlogscommon.MetricAttr{structuredlogscommon.MetricAttr{Name: "metric_a", Unit: "Count"}},
		result.Metrics))
}

func Test_process_mismatch(t *testing.T) {
	md := buildTestMetricDeclaration()

	metricTags := map[string]string{"tagA": "v1", "tagC": "v3"}
	metricFields := map[string]interface{}{"metric_a": "valueA", "metric_b": 10.0, "metric_c": 10.0}
	namespace := "ContainerInsights/Prometheus"
	metricUnit := map[string]string{"metric_a": "Count", "metric_b": "Percent", "metric_c": "Megabytes"}
	result := md.process(metricTags, metricFields, namespace, metricUnit)

	assert.True(t, reflect.ValueOf(result).IsNil())
}

func Test_process_metricunit(t *testing.T) {
	md := buildTestMetricDeclaration()

	metricTags := map[string]string{"tagA": "v1", "tagB": "v2"}
	metricFields := map[string]interface{}{"metric_a": "valueA", "metric_b": 10.0, "metric_c": 10.0}
	namespace := "ContainerInsights/Prometheus"
	metricUnit := map[string]string{"metric_a": "Count", "metric_c": "Megabytes", "metric_d": "Seconds"}

	result := md.process(metricTags, metricFields, namespace, metricUnit)
	assert.Equal(t, namespace, result.Namespace)
	assert.True(t, reflect.DeepEqual([][]string{{"tagA", "tagB"}, {"tagA"}, {"tag1", "tag2"}}, result.DimensionSets))
	metrics := result.Metrics
	assert.True(t, reflect.DeepEqual(sortMetrics([]structuredlogscommon.MetricAttr{
		{Name: "metric_a", Unit: "Count"},
		{Name: "metric_b"}}),
		sortMetrics(metrics)))
}

func sortMetrics(metrics []structuredlogscommon.MetricAttr) []structuredlogscommon.MetricAttr {
	sort.SliceStable(metrics, func(i, j int) bool {
		return metrics[i].Name < metrics[j].Name
	})
	return metrics
}
