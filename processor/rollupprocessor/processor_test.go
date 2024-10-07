// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package rollupprocessor

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/aws/amazon-cloudwatch-agent/internal/metric"
)

func TestProcessor(t *testing.T) {
	cfg := &Config{
		AttributeGroups: [][]string{
			{"d1", "d2"},
			{"d1", "d2", "d3"},
			// filtered out
			{"d1", "d2", "d2"},
			{},
		},
		DropOriginal: []string{"drop-original"},
		CacheSize:    5,
	}
	testCases := map[string]struct {
		cfg            *Config
		metricName     string
		metricType     pmetric.MetricType
		rawAttributes  []map[string]any
		wantAttributes []map[string]any
	}{
		"Rollup/WithMoreAttributes": {
			cfg:        cfg,
			metricName: "rollup",
			metricType: pmetric.MetricTypeGauge,
			rawAttributes: []map[string]any{
				{
					"d1":   "v1",
					"d2":   "v2",
					"d3":   "v3",
					"drop": true,
				},
			},
			wantAttributes: []map[string]any{
				{
					"d1":   "v1",
					"d2":   "v2",
					"d3":   "v3",
					"drop": true,
				},
				{
					"d1": "v1",
					"d2": "v2",
				},
				{
					"d1": "v1",
					"d2": "v2",
					"d3": "v3",
				},
				{},
			},
		},
		"DropOriginal/WithMoreAttributes": {
			cfg:        cfg,
			metricName: "drop-original",
			metricType: pmetric.MetricTypeGauge,
			rawAttributes: []map[string]any{
				{
					"d1":   "v1",
					"d2":   "v2",
					"d3":   "v3",
					"drop": true,
				},
			},
			wantAttributes: []map[string]any{
				{
					"d1": "v1",
					"d2": "v2",
				},
				{
					"d1": "v1",
					"d2": "v2",
					"d3": "v3",
				},
				{},
			},
		},
		"Rollup/WithSameAttributes": {
			cfg:        cfg,
			metricName: "rollup",
			metricType: pmetric.MetricTypeSum,
			rawAttributes: []map[string]any{
				{
					"d1": "v1",
					"d2": "v2",
					"d3": "v3",
				},
			},
			wantAttributes: []map[string]any{
				// original attributes are always first
				{
					"d1": "v1",
					"d2": "v2",
					"d3": "v3",
				},
				{
					"d1": "v1",
					"d2": "v2",
				},
				{},
			},
		},
		"DropOriginal/WithSameAttributes": {
			cfg:        cfg,
			metricName: "drop-original",
			metricType: pmetric.MetricTypeSum,
			rawAttributes: []map[string]any{
				{
					"d1": "v1",
					"d2": "v2",
					"d3": "v3",
				},
			},
			wantAttributes: []map[string]any{
				{
					"d1": "v1",
					"d2": "v2",
				},
				{},
			},
		},
		"Rollup/WithMissingAttributes": {
			cfg:        cfg,
			metricName: "rollup",
			metricType: pmetric.MetricTypeHistogram,
			rawAttributes: []map[string]any{
				{
					"d1": "v1",
					"d3": "v3",
					"d4": "v4",
				},
			},
			wantAttributes: []map[string]any{
				{
					"d1": "v1",
					"d3": "v3",
					"d4": "v4",
				},
				{},
			},
		},
		"DropOriginal/WithMissingAttributes": {
			cfg:        cfg,
			metricName: "drop-original",
			metricType: pmetric.MetricTypeHistogram,
			rawAttributes: []map[string]any{
				{
					"d1": "v1",
					"d3": "v3",
					"d4": "v4",
				},
			},
			wantAttributes: []map[string]any{
				{},
			},
		},
		"Rollup/WithLessAttributes": {
			cfg:        cfg,
			metricName: "rollup",
			metricType: pmetric.MetricTypeExponentialHistogram,
			rawAttributes: []map[string]any{
				{
					"d1": "v1",
					"d2": "v2",
				},
			},
			wantAttributes: []map[string]any{
				{
					"d1": "v1",
					"d2": "v2",
				},
				{},
			},
		},
		"DropOriginal/WithLessAttributes": {
			cfg:        cfg,
			metricName: "drop-original",
			metricType: pmetric.MetricTypeExponentialHistogram,
			rawAttributes: []map[string]any{
				{
					"d1": "v1",
					"d2": "v2",
				},
			},
			wantAttributes: []map[string]any{
				{},
			},
		},
		"Rollup/WithMultipleDataPoints": {
			cfg:        cfg,
			metricName: "rollup",
			metricType: pmetric.MetricTypeSummary,
			rawAttributes: []map[string]any{
				{
					"d1": "1v1",
					"d2": "1v2",
					"d4": "1v4",
				},
				{
					"d1": "3v1",
					"d2": "3v2",
					"d3": "3v3",
					"d4": "3v4",
				},
				{
					"d1": "1v1",
					"d2": "1v2",
					"d4": "1v4",
				},
			},
			wantAttributes: []map[string]any{
				// datapoint 1
				{
					"d1": "1v1",
					"d2": "1v2",
					"d4": "1v4",
				},
				{
					"d1": "1v1",
					"d2": "1v2",
				},
				{},
				// datapoint 2
				{
					"d1": "3v1",
					"d2": "3v2",
					"d3": "3v3",
					"d4": "3v4",
				},
				{
					"d1": "3v1",
					"d2": "3v2",
				},
				{
					"d1": "3v1",
					"d2": "3v2",
					"d3": "3v3",
				},
				{},
				// datapoint 3
				{
					"d1": "1v1",
					"d2": "1v2",
					"d4": "1v4",
				},
				{
					"d1": "1v1",
					"d2": "1v2",
				},
				{},
			},
		},
		"DropOriginal/NoRollup": {
			cfg: &Config{
				DropOriginal: []string{"drop-original"},
			},
			metricName: "drop-original",
			metricType: pmetric.MetricTypeSummary,
			rawAttributes: []map[string]any{
				{
					"d1": "1v1",
					"d2": "1v2",
				},
				{
					"d1": "2v1",
					"d2": "2v2",
				},
				{
					"d1": "3v1",
					"d2": "3v2",
					"d3": "3v3",
				},
			},
			wantAttributes: []map[string]any{},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			p := newProcessor(testCase.cfg)
			assert.NoError(t, p.start(context.Background(), componenttest.NewNopHost()))
			defer assert.NoError(t, p.stop(context.Background()))
			orig := pmetric.NewMetrics()
			ms := orig.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
			buildTestMetric(t, ms.AppendEmpty(), testCase.metricName, testCase.metricType, testCase.rawAttributes)
			assert.Equal(t, len(testCase.rawAttributes), orig.DataPointCount())
			got, err := p.processMetrics(context.Background(), orig)
			assert.NoError(t, err)
			var gotAttributes []pcommon.Map
			metric.RangeMetrics(got, func(m pmetric.Metric) {
				validateMetric(t, m)
				metric.RangeDataPointAttributes(m, func(attrs pcommon.Map) {
					gotAttributes = append(gotAttributes, attrs)
				})
			})
			require.Equal(t, len(testCase.wantAttributes), len(gotAttributes))
			for index, gotAttribute := range gotAttributes {
				wantAttribute := testCase.wantAttributes[index]
				assert.Equal(t, len(wantAttribute), gotAttribute.Len())
				assert.Truef(t, reflect.DeepEqual(gotAttribute.AsRaw(), wantAttribute), "want: %v, got: %v", wantAttribute, gotAttribute.AsRaw())
			}
		})
	}
}

func validateMetric(t *testing.T, m pmetric.Metric) {
	t.Helper()

	switch m.Type() {
	case pmetric.MetricTypeSum:
		assert.True(t, m.Sum().IsMonotonic())
		assert.Equal(t, pmetric.AggregationTemporalityCumulative, m.Sum().AggregationTemporality())
	case pmetric.MetricTypeHistogram:
		assert.Equal(t, pmetric.AggregationTemporalityDelta, m.Histogram().AggregationTemporality())
	case pmetric.MetricTypeExponentialHistogram:
		assert.Equal(t, pmetric.AggregationTemporalityCumulative, m.ExponentialHistogram().AggregationTemporality())
	}
}

func buildTestMetric(
	t *testing.T,
	m pmetric.Metric,
	name string,
	metricType pmetric.MetricType,
	rawAttributes []map[string]any,
) {
	t.Helper()

	m.SetName(name)
	switch metricType {
	case pmetric.MetricTypeGauge:
		m.SetEmptyGauge()
		buildTestDataPoints[pmetric.NumberDataPoint](t, m.Gauge().DataPoints(), rawAttributes)
	case pmetric.MetricTypeSum:
		m.SetEmptySum()
		m.Sum().SetIsMonotonic(true)
		m.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		buildTestDataPoints[pmetric.NumberDataPoint](t, m.Sum().DataPoints(), rawAttributes)
	case pmetric.MetricTypeHistogram:
		m.SetEmptyHistogram()
		m.Histogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		buildTestDataPoints[pmetric.HistogramDataPoint](t, m.Histogram().DataPoints(), rawAttributes)
	case pmetric.MetricTypeExponentialHistogram:
		m.SetEmptyExponentialHistogram()
		m.ExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		buildTestDataPoints[pmetric.ExponentialHistogramDataPoint](t, m.ExponentialHistogram().DataPoints(), rawAttributes)
	case pmetric.MetricTypeSummary:
		m.SetEmptySummary()
		buildTestDataPoints[pmetric.SummaryDataPoint](t, m.Summary().DataPoints(), rawAttributes)
	}
}

func buildTestDataPoints[T metric.DataPoint[T]](
	t *testing.T,
	dps metric.DataPoints[T],
	rawAttributes []map[string]any,
) {
	t.Helper()

	for _, rawAttribute := range rawAttributes {
		dp := dps.AppendEmpty()
		assert.NoError(t, dp.Attributes().FromRaw(rawAttribute))
	}
}
