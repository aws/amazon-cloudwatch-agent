// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus_scraper

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func buildTestData(pass int, drop int) (result PrometheusMetricBatch) {
	for i := 0; i < pass; i++ {
		pm := &PrometheusMetric{
			metricName: fmt.Sprintf("passed_id_%d", i),
			metricType: "counter",
		}
		result = append(result, pm)
	}

	for i := 0; i < drop; i++ {
		pm := &PrometheusMetric{
			metricName: fmt.Sprintf("dropped_id_%d", i),
			metricType: "histogram",
		}
		result = append(result, pm)
	}
	return
}

func TestMetricsFilterFilter_NoDropMetrics(t *testing.T) {
	var dropUnsupportedMetric bool
	//Filter Metric with DroppingUnsupportedMetric equals to true
	metricFilterWithDroppingUnsupportedMetric := &MetricsFilter{maxDropMetricsLogged: 3}

	dropUnsupportedMetric = true
	metricBatchWithDroppingUnsupportedMetric := buildTestData(1, 0)
	metricBatchWithDroppingUnsupportedMetric = metricFilterWithDroppingUnsupportedMetric.Filter(metricBatchWithDroppingUnsupportedMetric,dropUnsupportedMetric)


	assert.Equal(t, 3, metricFilterWithDroppingUnsupportedMetric.maxDropMetricsLogged)
	assert.Equal(t, 1, len(metricBatchWithDroppingUnsupportedMetric))
	assert.Equal(t, false, metricFilterWithDroppingUnsupportedMetric.hitMaxLimit)
	assert.Equal(t, 0, len(metricFilterWithDroppingUnsupportedMetric.droppedMetrics))

	//Filter Metric with DroppingUnsupportedMetric equals to false
	metricFilterWithoutDroppingUnsupportedMetric := &MetricsFilter{maxDropMetricsLogged: 3}

	dropUnsupportedMetric = false
	metricBatchWithoutDroppingUnsupportedMetric := buildTestData(1, 0)
	metricBatchWithoutDroppingUnsupportedMetric = metricFilterWithoutDroppingUnsupportedMetric.Filter(metricBatchWithoutDroppingUnsupportedMetric,dropUnsupportedMetric)


	assert.Equal(t, 3, metricFilterWithoutDroppingUnsupportedMetric.maxDropMetricsLogged)
	assert.Equal(t, 1, len(metricBatchWithoutDroppingUnsupportedMetric))
	assert.Equal(t, false, metricFilterWithoutDroppingUnsupportedMetric.hitMaxLimit)
	assert.Equal(t, 0, len(metricFilterWithoutDroppingUnsupportedMetric.droppedMetrics))
}

func TestMetricsFilterFilter_DropMetrics(t *testing.T) {
	var dropUnsupportedMetric bool
	//Filter Metric with DroppingUnsupportedMetric equals to true
	metricFilterWithDroppingUnsupportedMetric := &MetricsFilter{maxDropMetricsLogged: 3}

	dropUnsupportedMetric = true
	metricBatchWithDroppingUnsupportedMetric := buildTestData(2, 2)
	metricBatchWithDroppingUnsupportedMetric = metricFilterWithDroppingUnsupportedMetric.Filter(metricBatchWithDroppingUnsupportedMetric,dropUnsupportedMetric)


	assert.Equal(t, 3, metricFilterWithDroppingUnsupportedMetric.maxDropMetricsLogged)
	assert.Equal(t, 2, len(metricBatchWithDroppingUnsupportedMetric))
	assert.Equal(t, false, metricFilterWithDroppingUnsupportedMetric.hitMaxLimit)
	assert.Equal(t, 2, len(metricFilterWithDroppingUnsupportedMetric.droppedMetrics))

	//Filter Metric with DroppingUnsupportedMetric equals to false
	metricFilterWithoutDroppingUnsupportedMetric := &MetricsFilter{maxDropMetricsLogged: 3}

	dropUnsupportedMetric = false
	metricBatchWithoutDroppingUnsupportedMetric := buildTestData(2, 2)
	metricBatchWithoutDroppingUnsupportedMetric = metricFilterWithoutDroppingUnsupportedMetric.Filter(metricBatchWithoutDroppingUnsupportedMetric,dropUnsupportedMetric)


	assert.Equal(t, 3, metricFilterWithoutDroppingUnsupportedMetric.maxDropMetricsLogged)
	assert.Equal(t, 4, len(metricBatchWithoutDroppingUnsupportedMetric))
	assert.Equal(t, false, metricFilterWithoutDroppingUnsupportedMetric.hitMaxLimit)
	assert.Equal(t, 0, len(metricFilterWithoutDroppingUnsupportedMetric.droppedMetrics))
}

func TestMetricsFilterFilter_HitMaxDroppedMetrics(t *testing.T) {
	var dropUnsupportedMetric bool
	//Filter Metric with DroppingUnsupportedMetric equals to true
	metricFilterWithDroppingUnsupportedMetric := &MetricsFilter{maxDropMetricsLogged: 3}

	dropUnsupportedMetric = true
	metricBatchWithDroppingUnsupportedMetric := buildTestData(7, 3)
	metricBatchWithDroppingUnsupportedMetric = metricFilterWithDroppingUnsupportedMetric.Filter(metricBatchWithDroppingUnsupportedMetric,dropUnsupportedMetric)


	assert.Equal(t, 3, metricFilterWithDroppingUnsupportedMetric.maxDropMetricsLogged)
	assert.Equal(t, 7, len(metricBatchWithDroppingUnsupportedMetric))
	assert.Equal(t, true, metricFilterWithDroppingUnsupportedMetric.hitMaxLimit)
	assert.Equal(t, 0, len(metricFilterWithDroppingUnsupportedMetric.droppedMetrics))

	//Filter Metric with DroppingUnsupportedMetric equals to false
	metricFilterWithoutDroppingUnsupportedMetric := &MetricsFilter{maxDropMetricsLogged: 3}

	dropUnsupportedMetric = false
	metricBatchWithoutDroppingUnsupportedMetric := buildTestData(7, 3)
	metricBatchWithoutDroppingUnsupportedMetric = metricFilterWithoutDroppingUnsupportedMetric.Filter(metricBatchWithoutDroppingUnsupportedMetric,dropUnsupportedMetric)


	assert.Equal(t, 3, metricFilterWithoutDroppingUnsupportedMetric.maxDropMetricsLogged)
	assert.Equal(t, 10, len(metricBatchWithoutDroppingUnsupportedMetric))
	assert.Equal(t, false, metricFilterWithoutDroppingUnsupportedMetric.hitMaxLimit)
	assert.Equal(t, 0, len(metricFilterWithoutDroppingUnsupportedMetric.droppedMetrics))
}

func TestMetricsFilterFilter_ExceedMaxDroppedMetrics(t *testing.T) {
	var dropUnsupportedMetric bool
	//Filter Metric with DroppingUnsupportedMetric equals to true
	metricFilterWithDroppingUnsupportedMetric := &MetricsFilter{maxDropMetricsLogged: 3}

	dropUnsupportedMetric = true
	metricBatchWithDroppingUnsupportedMetric := buildTestData(2, 12)
	metricBatchWithDroppingUnsupportedMetric = metricFilterWithDroppingUnsupportedMetric.Filter(metricBatchWithDroppingUnsupportedMetric,dropUnsupportedMetric)


	assert.Equal(t, 3, metricFilterWithDroppingUnsupportedMetric.maxDropMetricsLogged)
	assert.Equal(t, 2, len(metricBatchWithDroppingUnsupportedMetric))
	assert.Equal(t, true, metricFilterWithDroppingUnsupportedMetric.hitMaxLimit)
	assert.Equal(t, 0, len(metricFilterWithDroppingUnsupportedMetric.droppedMetrics))

	//Filter Metric with DroppingUnsupportedMetric equals to false
	metricFilterWithoutDroppingUnsupportedMetric := &MetricsFilter{maxDropMetricsLogged: 3}

	dropUnsupportedMetric = false
	metricBatchWithoutDroppingUnsupportedMetric := buildTestData(2, 12)
	metricBatchWithoutDroppingUnsupportedMetric = metricFilterWithoutDroppingUnsupportedMetric.Filter(metricBatchWithoutDroppingUnsupportedMetric,dropUnsupportedMetric)


	assert.Equal(t, 3, metricFilterWithoutDroppingUnsupportedMetric.maxDropMetricsLogged)
	assert.Equal(t, 14, len(metricBatchWithoutDroppingUnsupportedMetric))
	assert.Equal(t, false, metricFilterWithoutDroppingUnsupportedMetric.hitMaxLimit)
	assert.Equal(t, 0, len(metricFilterWithoutDroppingUnsupportedMetric.droppedMetrics))
}

func TestMetricsFilterFilter_MetricsFilter(t *testing.T) {
	mf := NewMetricsFilter()
	assert.Equal(t, MaxDropMetricsLogged, mf.maxDropMetricsLogged)
}
