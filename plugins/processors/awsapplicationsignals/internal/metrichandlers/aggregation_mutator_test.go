// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metrichandlers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestAggregationMutator_ProcessMetrics(t *testing.T) {
	tests := []struct {
		name                string
		config              map[string]aggregationType
		metrics             []pmetric.Metric
		expectedTemporality map[string]pmetric.AggregationTemporality
	}{
		{
			"testCumulativeToDelta",
			map[string]aggregationType{
				"test0": lastValueAggregation,
			},

			[]pmetric.Metric{
				generateMetricWithSumAggregation("test0", pmetric.AggregationTemporalityCumulative),
			},
			map[string]pmetric.AggregationTemporality{
				"test0": pmetric.AggregationTemporalityDelta,
			},
		},
		{
			"testNoChange",
			map[string]aggregationType{
				"test0": lastValueAggregation,
				"test1": defaultAggregation,
			},
			[]pmetric.Metric{
				generateMetricWithSumAggregation("test0", pmetric.AggregationTemporalityDelta),
				generateMetricWithSumAggregation("test1", pmetric.AggregationTemporalityCumulative),
				generateMetricWithSumAggregation("test2", pmetric.AggregationTemporalityCumulative),
			},
			map[string]pmetric.AggregationTemporality{
				"test0": pmetric.AggregationTemporalityDelta,
				"test1": pmetric.AggregationTemporalityCumulative,
				"test2": pmetric.AggregationTemporalityCumulative,
			},
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t1 *testing.T) {
			mutator := newAggregationMutatorWithConfig(tt.config)

			for _, m := range tt.metrics {
				mutator.ProcessMetrics(ctx, m, pcommon.NewMap())
				assert.Equal(t1, tt.expectedTemporality[m.Name()], m.Sum().AggregationTemporality())
			}
		})
	}

	mutator := NewAggregationMutator()

	m := generateMetricWithSumAggregation("DotNetGCGen0HeapSize", pmetric.AggregationTemporalityCumulative)
	mutator.ProcessMetrics(ctx, m, pcommon.NewMap())
	assert.Equal(t, pmetric.MetricTypeSum, m.Type())
	assert.Equal(t, pmetric.AggregationTemporalityDelta, m.Sum().AggregationTemporality())

	m.SetEmptyHistogram()
	m.Histogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	mutator.ProcessMetrics(ctx, m, pcommon.NewMap())
	assert.Equal(t, pmetric.MetricTypeHistogram, m.Type())
	assert.Equal(t, pmetric.AggregationTemporalityCumulative, m.Histogram().AggregationTemporality())

}

func generateMetricWithSumAggregation(metricName string, temporality pmetric.AggregationTemporality) pmetric.Metric {
	m := pmetric.NewMetrics().ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName(metricName)
	m.SetEmptySum()
	m.Sum().SetAggregationTemporality(temporality)
	return m
}
