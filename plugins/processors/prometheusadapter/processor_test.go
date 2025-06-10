// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheusadapter

import (
	"context"
	"os"
	"testing"

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

	expectedDatapointAttrs := map[string]string{
		"include":   "yes",
		"label1":    "test1",
		"my_name":   "prometheus_test_gauge",
		"prom_type": "gauge",
		"job":       "test_service",
		"host":      hostname,
		"instance":  "i-xxxx",
		"receiver":  "prometheus",
	}

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
				assert.Equal(t, orig.MetricCount(), processed.MetricCount())
				origDps := orig.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints()
				procDps := processed.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints()

				assert.Equal(t, origDps.Len(), procDps.Len())
				for i := 0; i < origDps.Len(); i++ {
					assert.Equal(t, origDps.At(i).IntValue(), procDps.At(i).IntValue())
					assert.Equal(t, origDps.At(i).StartTimestamp(), procDps.At(i).StartTimestamp())
					assert.Equal(t, origDps.At(i).Timestamp(), procDps.At(i).Timestamp())
					verifyDatapointAttributes(t, procDps.At(i).Attributes(), expectedDatapointAttrs)
				}
			},
		},
		{
			name: "sum",
			metrics: generateCounterMetrics(
				"test_sum_metrics",
				resourceAttrsIn,
				datapointAttrsIn,
			),
			validate: func(t *testing.T, orig, processed pmetric.Metrics) {
				assert.Equal(t, orig.MetricCount(), processed.MetricCount())
				origDps := orig.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints()
				procDps := processed.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints()

				assert.Equal(t, origDps.Len(), procDps.Len())
				for i := 0; i < origDps.Len(); i++ {
					assert.Equal(t, origDps.At(i).IntValue(), procDps.At(i).IntValue())
					assert.Equal(t, origDps.At(i).StartTimestamp(), procDps.At(i).StartTimestamp())
					assert.Equal(t, origDps.At(i).Timestamp(), procDps.At(i).Timestamp())
					verifyDatapointAttributes(t, procDps.At(i).Attributes(), expectedDatapointAttrs)
				}
			},
		},
		{
			name: "summary",
			metrics: generateSummaryMetrics(
				"test_summary_metrics",
				resourceAttrsIn,
				datapointAttrsIn,
			),
			validate: func(t *testing.T, orig, processed pmetric.Metrics) {
				assert.Equal(t, orig.MetricCount(), processed.MetricCount())
				origDps := orig.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Summary().DataPoints()
				procDps := processed.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Summary().DataPoints()

				assert.Equal(t, origDps.Len(), procDps.Len())
				for i := 0; i < origDps.Len(); i++ {
					assert.Equal(t, origDps.At(i).Count(), procDps.At(i).Count())
					assert.Equal(t, origDps.At(i).Sum(), procDps.At(i).Sum())
					assert.Equal(t, origDps.At(i).StartTimestamp(), procDps.At(i).StartTimestamp())
					assert.Equal(t, origDps.At(i).Timestamp(), procDps.At(i).Timestamp())
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
				assert.Equal(t, orig.MetricCount(), processed.MetricCount())
				origDps := orig.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints()
				procDps := processed.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints()

				assert.Equal(t, origDps.Len(), procDps.Len())
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
				assert.Equal(t, orig.MetricCount(), processed.MetricCount())
				origDps := orig.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram().DataPoints()
				procDps := processed.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram().DataPoints()

				assert.Equal(t, origDps.Len(), procDps.Len())
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

			for i := 0; i < m.ResourceMetrics().Len(); i++ {
				attributes := m.ResourceMetrics().At(0).Resource().Attributes().AsRaw()
				assert.Len(t, attributes, 0, "There should be no resource attributes. resource %d has one or more attributes: %+v", i, attributes)
			}

			tt.validate(t, orig, m)
		})
	}

}

func verifyDatapointAttributes(t *testing.T, attrs pcommon.Map, expectedDatapointAttrs map[string]string) {
	// ensure all expected attributes are present
	for k, v := range expectedDatapointAttrs {
		val, ok := attrs.Get(k)
		assert.True(t, ok)
		assert.Equal(t, v, val.AsString(), "unexpected datapoint attribute {%q} value", k)
	}
	// ensure there are no other extra attributes
	for k, v := range attrs.AsRaw() {
		_, ok := expectedDatapointAttrs[k]
		assert.True(t, ok, "datapoint has attribute {%q=%q} which was not expected", k, v)
	}
}

func generateUntypedMetrics(metricName string, resourceAttrs map[string]string, datapointAttrs map[string]string) pmetric.Metrics {
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
		for k, v := range datapointAttrs {
			dp.Attributes().PutStr(k, v)
		}
	}
	return md
}

func generateGaugeMetrics(metricName string, resourceAttrs map[string]string, datapointAttrs map[string]string) pmetric.Metrics {
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
		for k, v := range datapointAttrs {
			dp.Attributes().PutStr(k, v)
		}
	}
	return md
}

func generateCounterMetrics(metricName string, resourceAttrs map[string]string, datapointAttrs map[string]string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rma := rm.Resource().Attributes()
	for k, v := range resourceAttrs {
		rma.PutStr(k, v)
	}
	ms := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	ms.SetName(metricName)
	dps := ms.SetEmptySum().DataPoints()
	for i := 0; i < 10; i++ {
		dp := dps.AppendEmpty()
		dp.SetIntValue(int64(100 * i))
		for k, v := range datapointAttrs {
			dp.Attributes().PutStr(k, v)
		}
	}
	return md
}

func generateSummaryMetrics(metricName string, resourceAttrs map[string]string, datapointAttrs map[string]string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rma := rm.Resource().Attributes()
	for k, v := range resourceAttrs {
		rma.PutStr(k, v)
	}
	ms := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	ms.SetName(metricName)
	dps := ms.SetEmptySummary().DataPoints()
	for i := 0; i < 10; i++ {
		dp := dps.AppendEmpty()
		dp.SetCount(uint64(i))
		dp.SetSum(1000.0 * float64(i))
		for k, v := range datapointAttrs {
			dp.Attributes().PutStr(k, v)
		}
	}
	return md
}

func generateHistogramMetrics(metricName string, resourceAttrs map[string]string, datapointAttrs map[string]string) pmetric.Metrics {
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
		for k, v := range datapointAttrs {
			dp.Attributes().PutStr(k, v)
		}
	}
	return md
}

func generateExponentialHistogramMetrics(metricName string, resourceAttrs map[string]string, datapointAttrs map[string]string) pmetric.Metrics {
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
		for k, v := range datapointAttrs {
			dp.Attributes().PutStr(k, v)
		}
	}
	return md
}
