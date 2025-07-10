// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheusadapter

import (
	"context"
	"maps"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func TestProcessMetrics(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	pap := newPrometheusAdapterProcessor(createDefaultConfig().(*Config), logger)
	require.NotNil(t, pap)

	ctx := context.Background()
	hostname, err := os.Hostname()
	require.NoError(t, err)

	resourceAttrsIn := map[string]string{
		"http.scheme":         "http",
		"server.port":         "8101",
		"net.host.port":       "8101",
		"url.scheme":          "http",
		"service.instance.id": "i-xxxx",
		"service.name":        "test_service",
	}

	datapointAttrsIn := map[string]string{
		"include":   "yes",
		"label1":    "test1",
		"my_name":   "prometheus_test_gauge",
		"prom_type": "gauge",
	}

	baseExpectedDatapointAttrs := map[string]string{
		"include":   "yes",
		"label1":    "test1",
		"my_name":   "prometheus_test_gauge",
		"prom_type": "gauge",
		"job":       "test_service",
		"host":      hostname,
		"instance":  "i-xxxx",
	}

	initialTime := time.Date(2025, 6, 26, 12, 30, 30, 0, time.UTC)

	tests := []struct {
		name     string
		metrics  pmetric.Metrics
		validate func(t *testing.T, orig, processed pmetric.Metrics)
	}{
		{
			name: "untyped",
			metrics: generateUntypedMetrics(
				"test_untyped_metrics",
				resourceAttrsIn,
				datapointAttrsIn,
			),
			validate: func(t *testing.T, orig, processed pmetric.Metrics) {
				assert.Equal(t, 0, processed.MetricCount(), "untyped metrics should be dropped by the processor")
			},
		},
		{
			name: "gauge",
			metrics: generateGaugeMetrics(
				"test_gauge_metrics",
				resourceAttrsIn,
				datapointAttrsIn,
			),
			validate: func(t *testing.T, orig, processed pmetric.Metrics) {
				expectedDatapointAttrs := maps.Clone(baseExpectedDatapointAttrs)
				expectedDatapointAttrs["prom_metric_type"] = "gauge"
				assert.Equal(t, orig.MetricCount(), processed.MetricCount())
				origDps := orig.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints()
				procDps := processed.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints()

				require.Equal(t, origDps.Len(), procDps.Len())
				for i := 0; i < origDps.Len(); i++ {
					assert.Equal(t, origDps.At(i).IntValue(), procDps.At(i).IntValue())
					assert.Equal(t, origDps.At(i).StartTimestamp(), procDps.At(i).StartTimestamp())
					assert.Equal(t, origDps.At(i).Timestamp(), procDps.At(i).Timestamp())
					verifyDatapointAttributes(t, procDps.At(i).Attributes(), expectedDatapointAttrs)
				}
			},
		},
		{
			name: "delta sum",
			metrics: generateCounterMetrics(
				"test_sum_metrics",
				10,
				100,
				initialTime,
				pmetric.AggregationTemporalityDelta,
				resourceAttrsIn,
				datapointAttrsIn,
			),
			validate: func(t *testing.T, orig, processed pmetric.Metrics) {
				expectedDatapointAttrs := maps.Clone(baseExpectedDatapointAttrs)
				expectedDatapointAttrs["prom_metric_type"] = "counter"
				assert.Equal(t, orig.MetricCount(), processed.MetricCount())
				origDps := orig.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints()
				procDps := processed.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints()

				require.Equal(t, origDps.Len(), procDps.Len())
				for i := 0; i < origDps.Len(); i++ {
					assert.Equal(t, origDps.At(i).IntValue(), procDps.At(i).IntValue())
					assert.Equal(t, origDps.At(i).StartTimestamp(), procDps.At(i).StartTimestamp())
					assert.Equal(t, origDps.At(i).Timestamp(), procDps.At(i).Timestamp())
					verifyDatapointAttributes(t, procDps.At(i).Attributes(), expectedDatapointAttrs)
				}
			},
		},
		{
			name: "cumulative sum",
			metrics: generateCounterMetrics(
				"test_sum_metrics",
				10,
				100,
				initialTime,
				pmetric.AggregationTemporalityCumulative,
				resourceAttrsIn,
				datapointAttrsIn,
			),
			validate: func(t *testing.T, orig, processed pmetric.Metrics) {
				expectedDatapointAttrs := maps.Clone(baseExpectedDatapointAttrs)
				expectedDatapointAttrs["prom_metric_type"] = "counter"
				assert.Equal(t, orig.MetricCount(), processed.MetricCount())
				origDps := orig.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints()
				procDps := processed.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints()

				require.Equal(t, origDps.Len()-1, procDps.Len())
				for i := 0; i < origDps.Len()-1; i++ {
					assert.Equal(t, int64(100), procDps.At(i).IntValue())
					assert.Equal(t, origDps.At(i+1).StartTimestamp(), procDps.At(i).StartTimestamp())
					assert.Equal(t, origDps.At(i+1).Timestamp(), procDps.At(i).Timestamp())
					verifyDatapointAttributes(t, procDps.At(i).Attributes(), expectedDatapointAttrs)
				}
			},
		},
		{
			name: "summary",
			metrics: generateSummaryMetrics(
				"test_summary_metrics",
				10,
				1000.0,
				1,
				initialTime,
				resourceAttrsIn,
				datapointAttrsIn,
			),
			validate: func(t *testing.T, orig, processed pmetric.Metrics) {
				expectedDatapointAttrs := maps.Clone(baseExpectedDatapointAttrs)
				expectedDatapointAttrs["prom_metric_type"] = "summary"
				assert.Equal(t, orig.MetricCount(), processed.MetricCount())
				origDps := orig.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Summary().DataPoints()
				procDps := processed.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Summary().DataPoints()

				// there should be one fewer summary datapoint due to delta calculations
				require.Equal(t, origDps.Len()-1, procDps.Len())
				for i := 0; i < origDps.Len()-1; i++ {
					assert.Equal(t, uint64(1), procDps.At(i).Count())
					assert.Equal(t, 1000.0, procDps.At(i).Sum())
					assert.Equal(t, origDps.At(i+1).StartTimestamp(), procDps.At(i).StartTimestamp())
					assert.Equal(t, origDps.At(i+1).Timestamp(), procDps.At(i).Timestamp())
					verifyDatapointAttributes(t, procDps.At(i).Attributes(), expectedDatapointAttrs)
				}
			},
		},
		{
			name: "histogram",
			metrics: generateHistogramMetrics(
				"test_histogram_metrics",
				resourceAttrsIn,
				datapointAttrsIn,
			),
			validate: func(t *testing.T, orig, processed pmetric.Metrics) {
				expectedDatapointAttrs := maps.Clone(baseExpectedDatapointAttrs)
				expectedDatapointAttrs["prom_metric_type"] = "histogram"
				assert.Equal(t, orig.MetricCount(), processed.MetricCount())
				origDps := orig.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints()
				procDps := processed.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints()

				require.Equal(t, origDps.Len(), procDps.Len())
				for i := 0; i < origDps.Len(); i++ {
					assert.Equal(t, origDps.At(i).Count(), procDps.At(i).Count())
					assert.Equal(t, origDps.At(i).Sum(), procDps.At(i).Sum())
					assert.Equal(t, origDps.At(i).Min(), procDps.At(i).Min())
					assert.Equal(t, origDps.At(i).Max(), procDps.At(i).Max())
					assert.Equal(t, origDps.At(i).BucketCounts().AsRaw(), procDps.At(i).BucketCounts().AsRaw())
					assert.Equal(t, origDps.At(i).ExplicitBounds().AsRaw(), procDps.At(i).ExplicitBounds().AsRaw())
					assert.Equal(t, origDps.At(i).StartTimestamp(), procDps.At(i).StartTimestamp())
					assert.Equal(t, origDps.At(i).Timestamp(), procDps.At(i).Timestamp())
					verifyDatapointAttributes(t, procDps.At(i).Attributes(), expectedDatapointAttrs)
				}
			},
		},
		{
			name: "exponential histogram",
			metrics: generateExponentialHistogramMetrics(
				"test_exponential_histogram_metrics",
				resourceAttrsIn,
				datapointAttrsIn,
			),
			validate: func(t *testing.T, orig, processed pmetric.Metrics) {
				expectedDatapointAttrs := maps.Clone(baseExpectedDatapointAttrs)
				expectedDatapointAttrs["prom_metric_type"] = "exponentialhistogram"
				assert.Equal(t, orig.MetricCount(), processed.MetricCount())
				origDps := orig.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram().DataPoints()
				procDps := processed.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram().DataPoints()

				require.Equal(t, origDps.Len(), procDps.Len())
				for i := 0; i < origDps.Len(); i++ {
					assert.Equal(t, origDps.At(i).Count(), procDps.At(i).Count())
					assert.Equal(t, origDps.At(i).Sum(), procDps.At(i).Sum())
					assert.Equal(t, origDps.At(i).Min(), procDps.At(i).Min())
					assert.Equal(t, origDps.At(i).Max(), procDps.At(i).Max())
					assert.Equal(t, origDps.At(i).Scale(), procDps.At(i).Scale())
					assert.Equal(t, origDps.At(i).ZeroCount(), procDps.At(i).ZeroCount())
					assert.Equal(t, origDps.At(i).Positive().BucketCounts().AsRaw(), procDps.At(i).Positive().BucketCounts().AsRaw())
					assert.Equal(t, origDps.At(i).Negative().BucketCounts().AsRaw(), procDps.At(i).Negative().BucketCounts().AsRaw())
					assert.Equal(t, origDps.At(i).StartTimestamp(), procDps.At(i).StartTimestamp())
					assert.Equal(t, origDps.At(i).Timestamp(), procDps.At(i).Timestamp())
					verifyDatapointAttributes(t, procDps.At(i).Attributes(), expectedDatapointAttrs)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.metrics
			orig := pmetric.NewMetrics()
			m.CopyTo(orig)
			pap.processMetrics(ctx, m)

			for i, rm := range m.ResourceMetrics().All() {
				attributes := rm.Resource().Attributes().AsRaw()
				assert.Len(t, attributes, 0, "There should be no resource attributes. resource %d has one or more attributes: %+v", i, attributes)
			}

			tt.validate(t, orig, m)
		})
	}

}

func TestDeltaCalculator(t *testing.T) {

	resourceAttrs := map[string]string{
		"http.scheme":         "http",
		"server.port":         "8101",
		"net.host.port":       "8101",
		"url.scheme":          "http",
		"service.instance.id": "i-xxxx",
		"service.name":        "test_service",
	}

	datapointAttrs := map[string]string{
		"include":   "yes",
		"label1":    "test1",
		"prom_type": "counter",
	}

	initialTime := time.Date(2025, 6, 26, 12, 30, 30, 0, time.UTC)

	t.Run("delta counter", func(t *testing.T) {
		logger, _ := zap.NewDevelopment()
		pap := newPrometheusAdapterProcessor(createDefaultConfig().(*Config), logger)
		require.NotNil(t, pap)

		ctx := context.Background()

		md := generateCounterMetrics("prometheus_test_counter", 1, 100, initialTime, pmetric.AggregationTemporalityDelta, resourceAttrs, datapointAttrs)
		pap.processMetrics(ctx, md)
		assert.NotZero(t, md.MetricCount(), "expected processor to not drop first delta metric value but metric count is %d", md.MetricCount())
		rms := md.ResourceMetrics()
		require.Equal(t, 1, rms.Len())
		sms := rms.At(0).ScopeMetrics()
		require.Equal(t, 1, sms.Len())
		ms := sms.At(0).Metrics()
		require.Equal(t, 1, ms.Len())
		dps := ms.At(0).Sum().DataPoints()
		require.Equal(t, 1, dps.Len())
		dp := dps.At(0)
		assert.Equal(t, int64(100), dp.IntValue(), "delta counter value should remain unchanged")
	})

	t.Run("cumulative counter", func(t *testing.T) {
		logger, _ := zap.NewDevelopment()
		pap := newPrometheusAdapterProcessor(createDefaultConfig().(*Config), logger)
		require.NotNil(t, pap)

		ctx := context.Background()

		md := generateCounterMetrics("prometheus_test_counter", 1, 100, initialTime, pmetric.AggregationTemporalityCumulative, resourceAttrs, datapointAttrs)
		pap.processMetrics(ctx, md)
		assert.Zero(t, md.MetricCount(), "expected processor to drop first cumulative metric value but metric count is %d", md.MetricCount())

		md = generateCounterMetrics("prometheus_test_counter", 1, 200, initialTime.Add(time.Minute), pmetric.AggregationTemporalityCumulative, resourceAttrs, datapointAttrs)
		pap.processMetrics(ctx, md)
		assert.NotZero(t, md.MetricCount(), "expected processor to not drop second cumulative metric value but metric count is %d", md.MetricCount())
		rms := md.ResourceMetrics()
		require.Equal(t, 1, rms.Len())
		sms := rms.At(0).ScopeMetrics()
		require.Equal(t, 1, sms.Len())
		ms := sms.At(0).Metrics()
		require.Equal(t, 1, ms.Len())
		dps := ms.At(0).Sum().DataPoints()
		require.Equal(t, 1, dps.Len())
		dp := dps.At(0)
		assert.Equal(t, int64(100), dp.IntValue())

		md = generateCounterMetrics("prometheus_test_counter", 10, 300, initialTime.Add(2*time.Minute), pmetric.AggregationTemporalityCumulative, resourceAttrs, datapointAttrs)
		pap.processMetrics(ctx, md)
		rms = md.ResourceMetrics()
		require.Equal(t, 1, rms.Len())
		sms = rms.At(0).ScopeMetrics()
		require.Equal(t, 1, sms.Len())
		ms = sms.At(0).Metrics()
		require.Equal(t, 1, ms.Len())
		dps = ms.At(0).Sum().DataPoints()
		require.Equal(t, 10, dps.Len())
		for _, dp := range dps.All() {
			assert.Equal(t, int64(100), dp.IntValue())
		}
	})

	t.Run("summary", func(t *testing.T) {

		logger, _ := zap.NewDevelopment()
		pap := newPrometheusAdapterProcessor(createDefaultConfig().(*Config), logger)
		require.NotNil(t, pap)

		ctx := context.Background()

		md := generateSummaryMetrics("prometheus_test_summary", 1, 1000.0, 1, initialTime, resourceAttrs, datapointAttrs)
		pap.processMetrics(ctx, md)
		assert.Zero(t, md.MetricCount(), "expected processor to drop first summary metric value but metric count is %d", md.MetricCount())

		md = generateSummaryMetrics("prometheus_test_summary", 1, 2000.0, 2, initialTime.Add(time.Minute), resourceAttrs, datapointAttrs)
		pap.processMetrics(ctx, md)
		assert.NotZero(t, md.MetricCount(), "expected processor to not drop second summary metric value but metric count is %d", md.MetricCount())
		rms := md.ResourceMetrics()
		require.Equal(t, 1, rms.Len())
		sms := rms.At(0).ScopeMetrics()
		require.Equal(t, 1, sms.Len())
		ms := sms.At(0).Metrics()
		require.Equal(t, 1, ms.Len())
		dps := ms.At(0).Summary().DataPoints()
		require.Equal(t, 1, dps.Len())
		dp := dps.At(0)
		assert.Equal(t, 1000.0, dp.Sum())
		assert.Equal(t, uint64(1), dp.Count())

		// now process multiple data points from one metric
		md = generateSummaryMetrics("prometheus_test_summary", 10, 3000.0, 3, initialTime.Add(2*time.Minute), resourceAttrs, datapointAttrs)
		pap.processMetrics(ctx, md)
		assert.Equal(t, 1, md.MetricCount(), "expected processor to drop first summary metric value but metric count is %d", md.MetricCount())
		rms = md.ResourceMetrics()
		require.Equal(t, 1, rms.Len())
		sms = rms.At(0).ScopeMetrics()
		require.Equal(t, 1, sms.Len())
		ms = sms.At(0).Metrics()
		require.Equal(t, 1, ms.Len())
		dps = ms.At(0).Summary().DataPoints()
		assert.Equal(t, 10, dps.Len())
		for _, dp := range dps.All() {
			assert.Equal(t, 1000.0, dp.Sum())
			assert.Equal(t, uint64(1), dp.Count())
		}
	})

}

func verifyDatapointAttributes(t *testing.T, attrs pcommon.Map, expectedDatapointAttrs map[string]string) {
	// ensure all expected attributes are present
	for k, v := range expectedDatapointAttrs {
		val, ok := attrs.Get(k)
		require.True(t, ok, "datapoint is missing attribute {%q}", k)
		assert.Equal(t, v, val.AsString(), "unexpected datapoint attribute {%q} value", k)
	}
	// ensure there are no other extra attributes
	for k, v := range attrs.AsRaw() {
		_, ok := expectedDatapointAttrs[k]
		assert.True(t, ok, "datapoint has attribute {%q=%q} which was not expected", k, v)
	}
}

func generateUntypedMetrics(metricName string, resourceAttrs map[string]string, datapointAttrs map[string]string) pmetric.Metrics {
	initialTime := time.Date(2025, 6, 26, 12, 30, 30, 0, time.UTC)
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rma := rm.Resource().Attributes()
	for k, v := range resourceAttrs {
		rma.PutStr(k, v)
	}
	ms := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	ms.SetName(metricName)
	dps := ms.SetEmptyGauge().DataPoints()
	ms.Metadata().PutStr("prometheus.type", "unknown")
	for i := 0; i < 10; i++ {
		dp := dps.AppendEmpty()
		dp.SetIntValue(int64(i))
		dp.SetTimestamp(pcommon.NewTimestampFromTime(initialTime.Add(time.Duration(i) * time.Minute)))
		for k, v := range datapointAttrs {
			dp.Attributes().PutStr(k, v)
		}
	}
	return md
}

func generateGaugeMetrics(metricName string, resourceAttrs map[string]string, datapointAttrs map[string]string) pmetric.Metrics {
	initialTime := time.Date(2025, 6, 26, 12, 30, 30, 0, time.UTC)
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rma := rm.Resource().Attributes()
	for k, v := range resourceAttrs {
		rma.PutStr(k, v)
	}
	ms := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	ms.SetName(metricName)
	dps := ms.SetEmptyGauge().DataPoints()
	for i := 0; i < 10; i++ {
		dp := dps.AppendEmpty()
		dp.SetIntValue(int64(i))
		dp.SetTimestamp(pcommon.NewTimestampFromTime(initialTime.Add(time.Duration(i) * time.Minute)))
		for k, v := range datapointAttrs {
			dp.Attributes().PutStr(k, v)
		}
	}
	return md
}

// createCounterMetric creates a new pmetric.Metrics which contains 1 resource and 1 scope with n Sum datapoints with
// the given temporality. the first data point value and time are given as inputs. subsequent datapoints' value is
// increased by 100, and the timestamp is increased by 1 minute. the one and only resource will have the given
// resourceAttrs. each datapoint will have the given datapointAttrs
func generateCounterMetrics(metricName string, n int, startValue int64, startTime time.Time, temporality pmetric.AggregationTemporality, resourceAttrs map[string]string, datapointAttrs map[string]string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rma := rm.Resource().Attributes()
	for k, v := range resourceAttrs {
		rma.PutStr(k, v)
	}
	ms := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	ms.SetName(metricName)
	sum := ms.SetEmptySum()
	sum.SetAggregationTemporality(temporality)
	dps := sum.DataPoints()
	for i := 0; i < n; i++ {
		dp := dps.AppendEmpty()
		dp.SetIntValue(startValue + 100*int64(i))
		dp.SetTimestamp(pcommon.NewTimestampFromTime(startTime.Add(time.Duration(i) * time.Minute)))
		for k, v := range datapointAttrs {
			dp.Attributes().PutStr(k, v)
		}
	}
	return md
}

// generateSummaryMetrics creates a new pmetric.Metrics which contains 1 resource and 1 scope with n Summary datapoints.
// the first data point value and time are given as inputs. subsequent datapoints' sum is increased by 1000.0, count
// increased by 1, and timestamp increased by 1 minute. the one and only resource will have the given resourceAttrs.
// each datapoint will have the given datapointsAttrs
func generateSummaryMetrics(metricName string, n int, startSum float64, startCont uint64, startTime time.Time, resourceAttrs map[string]string, datapointAttrs map[string]string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rma := rm.Resource().Attributes()
	for k, v := range resourceAttrs {
		rma.PutStr(k, v)
	}
	ms := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	ms.SetName(metricName)
	dps := ms.SetEmptySummary().DataPoints()
	for i := 0; i < n; i++ {
		dp := dps.AppendEmpty()
		dp.SetCount(startCont + uint64(i))
		dp.SetSum(startSum + 1000.0*float64(i))
		dp.SetTimestamp(pcommon.NewTimestampFromTime(startTime.Add(time.Duration(i) * time.Minute)))
		for k, v := range datapointAttrs {
			dp.Attributes().PutStr(k, v)
		}
	}
	return md
}

func generateHistogramMetrics(metricName string, resourceAttrs map[string]string, datapointAttrs map[string]string) pmetric.Metrics {
	initialTime := time.Date(2025, 6, 26, 12, 30, 30, 0, time.UTC)
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rma := rm.Resource().Attributes()
	for k, v := range resourceAttrs {
		rma.PutStr(k, v)
	}
	ms := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	ms.SetName(metricName)
	dps := ms.SetEmptyHistogram().DataPoints()
	for i := 0; i < 10; i++ {
		dp := dps.AppendEmpty()
		dp.SetMin(0.0)
		dp.SetMax(50.0)
		dp.SetCount(uint64(i))
		dp.SetSum(1000.0 * float64(i))
		dp.BucketCounts().Append(1, 2, 3, 4, 5)
		dp.SetTimestamp(pcommon.NewTimestampFromTime(initialTime.Add(time.Duration(i) * time.Minute)))
		for k, v := range datapointAttrs {
			dp.Attributes().PutStr(k, v)
		}
	}
	return md
}

func generateExponentialHistogramMetrics(metricName string, resourceAttrs map[string]string, datapointAttrs map[string]string) pmetric.Metrics {
	initialTime := time.Date(2025, 6, 26, 12, 30, 30, 0, time.UTC)
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rma := rm.Resource().Attributes()
	for k, v := range resourceAttrs {
		rma.PutStr(k, v)
	}
	ms := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	ms.SetName(metricName)
	dps := ms.SetEmptyExponentialHistogram().DataPoints()
	for i := 0; i < 10; i++ {
		dp := dps.AppendEmpty()
		dp.SetMin(0.0)
		dp.SetMax(50.0)
		dp.SetCount(uint64(i))
		dp.SetSum(1000.0 * float64(i))
		dp.SetScale(1)
		dp.SetZeroCount(1)
		dp.Positive().BucketCounts().Append(1, 2, 3, 4, 5)
		dp.Negative().BucketCounts().Append(1, 2, 3, 4, 5)
		dp.SetTimestamp(pcommon.NewTimestampFromTime(initialTime.Add(time.Duration(i) * time.Minute)))
		for k, v := range datapointAttrs {
			dp.Attributes().PutStr(k, v)
		}
	}
	return md
}
