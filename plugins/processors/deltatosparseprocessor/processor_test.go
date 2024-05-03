// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package deltatosparseprocessor

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.uber.org/zap"
)

var (
	zeroFlag    = pmetric.DefaultDataPointFlags
	noValueFlag = pmetric.DefaultDataPointFlags.WithNoRecordedValue(true)
)

type testSumMetric struct {
	metricNames  []string
	metricValues [][]float64
	isDelta      []bool
	isMonotonic  []bool
	flags        [][]pmetric.DataPointFlags
}

type deltaToSparseTest struct {
	name       string
	include    []string
	inMetrics  pmetric.Metrics
	outMetrics pmetric.Metrics
}

func TestCumulativeToDeltaProcessor(t *testing.T) {
	testCases := []deltaToSparseTest{
		{
			name:    "delta_to_sparse_one_positive",
			include: []string{"metric_1"},
			inMetrics: generateTestSumMetrics(testSumMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{0, 100, 0, 500}, {0, 4}},
				isDelta:      []bool{true, true},
				isMonotonic:  []bool{true, true},
			}),
			outMetrics: generateTestSumMetrics(testSumMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100, 500}, {0, 4}},
				isDelta:      []bool{true, true},
				isMonotonic:  []bool{true, true},
			}),
		},
		{
			name:    "delta_to_sparse_nan_value",
			include: []string{"metric_1"},
			inMetrics: generateTestSumMetrics(testSumMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{0, 100, 200, math.NaN()}, {4}},
				isDelta:      []bool{true, true},
				isMonotonic:  []bool{true, true},
			}),
			outMetrics: generateTestSumMetrics(testSumMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100, 200, math.NaN()}, {4}},
				isDelta:      []bool{true, true},
				isMonotonic:  []bool{true, true},
			}),
		},
		{
			name: "delta_to_sparse_no_include_config",
			inMetrics: generateTestSumMetrics(testSumMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{0, 100, 0, 200, 400}, {0, 100, 0, 0, 400}},
				isDelta:      []bool{true, true},
				isMonotonic:  []bool{true, true},
				flags: [][]pmetric.DataPointFlags{
					{zeroFlag, zeroFlag, noValueFlag, zeroFlag, zeroFlag},
					{zeroFlag, zeroFlag, noValueFlag, noValueFlag, zeroFlag},
				},
			}),
			outMetrics: generateTestSumMetrics(testSumMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{0, 100, 0, 200, 400}, {0, 100, 0, 0, 400}},
				isDelta:      []bool{true, true},
				isMonotonic:  []bool{true, true},
				flags: [][]pmetric.DataPointFlags{
					{zeroFlag, zeroFlag, noValueFlag, zeroFlag, zeroFlag},
					{zeroFlag, zeroFlag, noValueFlag, noValueFlag, zeroFlag},
				},
			}),
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			// next stores the results of the filter metric processor
			next := new(consumertest.MetricsSink)
			cfg := &Config{
				Include: test.include,
			}
			factory := NewFactory()
			mgp, err := factory.CreateMetricsProcessor(
				context.Background(),
				processortest.NewNopCreateSettings(),
				cfg,
				next,
			)
			assert.NotNil(t, mgp)
			assert.Nil(t, err)

			caps := mgp.Capabilities()
			assert.True(t, caps.MutatesData)
			ctx := context.Background()
			require.NoError(t, mgp.Start(ctx, nil))

			cErr := mgp.ConsumeMetrics(context.Background(), test.inMetrics)
			assert.Nil(t, cErr)
			got := next.AllMetrics()

			require.Equal(t, 1, len(got))
			require.Equal(t, test.outMetrics.ResourceMetrics().Len(), got[0].ResourceMetrics().Len())

			expectedMetrics := test.outMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
			actualMetrics := got[0].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()

			require.Equal(t, expectedMetrics.Len(), actualMetrics.Len())

			for i := 0; i < expectedMetrics.Len(); i++ {
				eM := expectedMetrics.At(i)
				aM := actualMetrics.At(i)

				require.Equal(t, eM.Name(), aM.Name())

				if eM.Type() == pmetric.MetricTypeGauge {
					eDataPoints := eM.Gauge().DataPoints()
					aDataPoints := aM.Gauge().DataPoints()
					require.Equal(t, eDataPoints.Len(), aDataPoints.Len())

					for j := 0; j < eDataPoints.Len(); j++ {
						require.Equal(t, eDataPoints.At(j).DoubleValue(), aDataPoints.At(j).DoubleValue())
					}
				}

				if eM.Type() == pmetric.MetricTypeSum {
					eDataPoints := eM.Sum().DataPoints()
					aDataPoints := aM.Sum().DataPoints()

					require.Equal(t, eDataPoints.Len(), aDataPoints.Len())
					require.Equal(t, eM.Sum().AggregationTemporality(), aM.Sum().AggregationTemporality())

					for j := 0; j < eDataPoints.Len(); j++ {
						if math.IsNaN(eDataPoints.At(j).DoubleValue()) {
							assert.True(t, math.IsNaN(aDataPoints.At(j).DoubleValue()))
						} else {
							require.Equal(t, eDataPoints.At(j).DoubleValue(), aDataPoints.At(j).DoubleValue())
						}
						require.Equal(t, eDataPoints.At(j).Flags(), aDataPoints.At(j).Flags())
					}
				}

				if eM.Type() == pmetric.MetricTypeHistogram {
					eDataPoints := eM.Histogram().DataPoints()
					aDataPoints := aM.Histogram().DataPoints()

					require.Equal(t, eDataPoints.Len(), aDataPoints.Len())
					require.Equal(t, eM.Histogram().AggregationTemporality(), aM.Histogram().AggregationTemporality())

					for j := 0; j < eDataPoints.Len(); j++ {
						require.Equal(t, eDataPoints.At(j).Count(), aDataPoints.At(j).Count())
						require.Equal(t, eDataPoints.At(j).HasSum(), aDataPoints.At(j).HasSum())
						require.Equal(t, eDataPoints.At(j).HasMin(), aDataPoints.At(j).HasMin())
						require.Equal(t, eDataPoints.At(j).HasMax(), aDataPoints.At(j).HasMax())
						if math.IsNaN(eDataPoints.At(j).Sum()) {
							require.True(t, math.IsNaN(aDataPoints.At(j).Sum()))
						} else {
							require.Equal(t, eDataPoints.At(j).Sum(), aDataPoints.At(j).Sum())
						}
						require.Equal(t, eDataPoints.At(j).BucketCounts(), aDataPoints.At(j).BucketCounts())
						require.Equal(t, eDataPoints.At(j).Flags(), aDataPoints.At(j).Flags())
					}
				}
			}

			require.NoError(t, mgp.Shutdown(ctx))
		})
	}
}

func generateTestSumMetrics(tm testSumMetric) pmetric.Metrics {
	md := pmetric.NewMetrics()
	now := time.Now()

	rm := md.ResourceMetrics().AppendEmpty()
	ms := rm.ScopeMetrics().AppendEmpty().Metrics()
	for i, name := range tm.metricNames {
		m := ms.AppendEmpty()
		m.SetName(name)
		sum := m.SetEmptySum()
		sum.SetIsMonotonic(tm.isMonotonic[i])

		if tm.isDelta[i] {
			sum.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		} else {
			sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		}

		for index, value := range tm.metricValues[i] {
			dp := m.Sum().DataPoints().AppendEmpty()
			dp.SetTimestamp(pcommon.NewTimestampFromTime(now.Add(10 * time.Second)))
			dp.SetDoubleValue(value)
			if len(tm.flags) > i && len(tm.flags[i]) > index {
				dp.SetFlags(tm.flags[i][index])
			}
		}
	}

	return md
}

func BenchmarkConsumeMetrics(b *testing.B) {
	c := consumertest.NewNop()
	params := processor.CreateSettings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.NewNop(),
		},
		BuildInfo: component.BuildInfo{},
	}
	cfg := createDefaultConfig().(*Config)
	p, err := createMetricsProcessor(context.Background(), params, cfg, c)
	if err != nil {
		b.Fatal(err)
	}

	metrics := pmetric.NewMetrics()
	rms := metrics.ResourceMetrics().AppendEmpty()
	r := rms.Resource()
	r.Attributes().PutBool("resource", true)
	ilms := rms.ScopeMetrics().AppendEmpty()
	ilms.Scope().SetName("test")
	ilms.Scope().SetVersion("0.1")
	m := ilms.Metrics().AppendEmpty()
	m.SetEmptySum().SetIsMonotonic(true)
	dp := m.Sum().DataPoints().AppendEmpty()
	dp.Attributes().PutStr("tag", "value")

	reset := func() {
		m.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		dp.SetDoubleValue(100.0)
	}

	// Load initial value
	reset()
	assert.NoError(b, p.ConsumeMetrics(context.Background(), metrics))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reset()
		assert.NoError(b, p.ConsumeMetrics(context.Background(), metrics))
	}
}
