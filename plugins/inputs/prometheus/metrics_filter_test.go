// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

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
	p := &MetricsFilter{maxDropMetricsLogged: 3}

	batch := buildTestData(1, 0)
	batch = p.Filter(batch)

	assert.Equal(t, 3, p.maxDropMetricsLogged)
	assert.Equal(t, 1, len(batch))
	assert.Equal(t, false, p.hitMaxLimit)
	assert.Equal(t, 0, len(p.droppedMetrics))
}

func TestMetricsFilterFilter_DropMetrics(t *testing.T) {

	p := &MetricsFilter{maxDropMetricsLogged: 3}

	batch := buildTestData(2, 2)
	batch = p.Filter(batch)

	assert.Equal(t, 3, p.maxDropMetricsLogged)
	assert.Equal(t, 2, len(batch))
	assert.Equal(t, false, p.hitMaxLimit)
	assert.Equal(t, 2, len(p.droppedMetrics))
}

func TestMetricsFilterFilter_HitMaxDroppedMetrics(t *testing.T) {

	p := &MetricsFilter{maxDropMetricsLogged: 3}

	batch := buildTestData(7, 3)
	batch = p.Filter(batch)

	assert.Equal(t, 3, p.maxDropMetricsLogged)
	assert.Equal(t, 7, len(batch))
	assert.Equal(t, true, p.hitMaxLimit)
	assert.Equal(t, 0, len(p.droppedMetrics))
}

func TestMetricsFilterFilter_ExceedMaxDroppedMetrics(t *testing.T) {

	p := &MetricsFilter{maxDropMetricsLogged: 3}

	batch := buildTestData(2, 12)
	batch = p.Filter(batch)

	assert.Equal(t, 3, p.maxDropMetricsLogged)
	assert.Equal(t, 2, len(batch))
	assert.Equal(t, true, p.hitMaxLimit)
	assert.Equal(t, 0, len(p.droppedMetrics))
}

func TestMetricsFilterFilter_MetricsFilter(t *testing.T) {
	mf := NewMetricsFilter()
	assert.Equal(t, MaxDropMetricsLogged, mf.maxDropMetricsLogged)
}
