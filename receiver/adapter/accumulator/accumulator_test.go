// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package accumulator

import (
	"fmt"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/testutil"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"testing"
	"time"
)

func Test_Accumulator_AddCounterGaugeFields(t *testing.T) {
	t.Helper()

	as := assert.New(t)

	test_cases := []struct {
		name                   string
		telegrafMetricName     string
		telegrafMetricTags     map[string]string
		telegrafMetricType     telegraf.ValueType
		expectedOtelMetricType pmetric.MetricDataType
		expectedDPAttributes   pcommon.Map
	}{
		{
			name:                   "OtelAccumulator with AddGauge",
			telegrafMetricName:     "acc_gauge_test",
			telegrafMetricTags:     map[string]string{defaultInstanceId: defaultInstanceIdValue},
			telegrafMetricType:     telegraf.Gauge,
			expectedOtelMetricType: pmetric.MetricDataTypeGauge,
			expectedDPAttributes:   generateExpectedAttributesWithMeasurement("acc_gauge_test"),
		},
		{
			name:                   "OtelAccumulator with AddCounter",
			telegrafMetricName:     "acc_counter_test",
			telegrafMetricTags:     map[string]string{defaultInstanceId: defaultInstanceIdValue},
			telegrafMetricType:     telegraf.Counter,
			expectedOtelMetricType: pmetric.MetricDataTypeSum,
			expectedDPAttributes:   generateExpectedAttributesWithMeasurement("acc_counter_test"),
		},
		{
			name:                   "OtelAccumulator with AddFields",
			telegrafMetricName:     "acc_field_test",
			telegrafMetricTags:     map[string]string{defaultInstanceId: defaultInstanceIdValue},
			telegrafMetricType:     telegraf.Untyped,
			expectedOtelMetricType: pmetric.MetricDataTypeGauge,
			expectedDPAttributes:   generateExpectedAttributesWithMeasurement("acc_field_test"),
		},
	}
	for _, tc := range test_cases {
		t.Run(tc.name, func(t *testing.T) {

			acc := newOtelAccumulatorWithTestRunningInputs(as)

			now := time.Now()
			telegrafMetricFields := map[string]interface{}{"time": float64(3.5), "error": false}

			switch tc.telegrafMetricType {
			case telegraf.Counter:
				acc.AddCounter(tc.telegrafMetricName, telegrafMetricFields, tc.telegrafMetricTags)
			case telegraf.Untyped:
				acc.AddFields(tc.telegrafMetricName, telegrafMetricFields, tc.telegrafMetricTags, now)
			case telegraf.Gauge:
				acc.AddGauge(tc.telegrafMetricName, telegrafMetricFields, tc.telegrafMetricTags, now)
			}
			otelMetrics := acc.GetOtelMetrics()

			metrics := otelMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
			as.Equal(2, metrics.Len())

			for i := 0; i < metrics.Len(); i++ {
				metric := metrics.At(i)
				as.Equal(tc.expectedOtelMetricType, metric.DataType())
				var datapoint pmetric.NumberDataPoint
				switch tc.telegrafMetricType {
				case telegraf.Counter:
					datapoint = metric.Sum().DataPoints().At(0)
				case telegraf.Gauge, telegraf.Untyped:
					datapoint = metric.Gauge().DataPoints().At(0)
				}

				as.Equal(tc.expectedDPAttributes, datapoint.Attributes())
			}

		})
	}
}

func Test_Accumulator_WithUnsupportedValueAndEmptyFields(t *testing.T) {
	t.Helper()

	as := assert.New(t)

	acc := newOtelAccumulatorWithTestRunningInputs(as)

	//Unsupported fields - string value field
	acc.AddFields("foo", map[string]interface{}{"client": "redis", "client2": "redis2"}, map[string]string{defaultInstanceId: defaultInstanceIdValue}, time.Now())

	otelMetrics := acc.GetOtelMetrics()
	// Ensure no metrics are built when value from fields are unsupported
	as.Equal(pmetric.NewMetrics(), otelMetrics)
	as.Equal(0, otelMetrics.ResourceMetrics().Len())

	// Empty fields
	acc.AddFields("foo", map[string]interface{}{}, map[string]string{}, time.Now())

	otelMetrics = acc.GetOtelMetrics()
	// Ensure no metrics are built when value from fields are unsupported
	as.Equal(pmetric.NewMetrics(), otelMetrics)
	as.Equal(0, otelMetrics.ResourceMetrics().Len())
}

func Test_ModifyMetricandConvertMetricValue(t *testing.T) {
	t.Helper()

	as := assert.New(t)

	acc := newOtelAccumulatorWithTestRunningInputs(as)

	metric := testutil.MustMetric(
		"cpu",
		map[string]string{
			"instance_id": "mock",
		},
		map[string]interface{}{
			"tx":     float64(4.5),
			"rx":     int32(3),
			"error":  false,
			"client": "redis",
		},
		time.Now(),
		telegraf.Gauge,
	)

	modifiedMetric, err := acc.modifyMetricandConvertToOtelValue(metric)
	as.NoError(err)

	txMetricValue, txMetricExist := modifiedMetric.GetField("tx")
	as.True(txMetricExist)
	as.Equal(float64(4.5), txMetricValue)

	rxMetricValue, rxMetricExist := modifiedMetric.GetField("rx")
	as.True(rxMetricExist)
	as.Equal(int64(3), rxMetricValue)

	errorMetricValue, errorMetricExist := modifiedMetric.GetField("error")
	as.True(errorMetricExist)
	as.Equal(int64(0), errorMetricValue)

	_, clientMetricExist := modifiedMetric.GetField("client")
	as.False(clientMetricExist)

}

func Test_Accumulator_AddMetric(t *testing.T) {
	t.Helper()

	as := assert.New(t)

	acc := newOtelAccumulatorWithTestRunningInputs(as)

	telegrafMetric := testutil.MustMetric(
		"acc_metric_test",
		map[string]string{defaultInstanceId: defaultInstanceIdValue},
		map[string]interface{}{"sin": int32(4)}, time.Now().UTC(),
		telegraf.Untyped)

	acc.SetPrecision(time.Microsecond)
	acc.AddMetric(telegrafMetric)
	acc.AddMetric(telegrafMetric)

	otelMetrics := acc.GetOtelMetrics()

	as.Equal(2, otelMetrics.ResourceMetrics().Len())

	metrics := otelMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	as.Equal(1, metrics.Len())

	for i := 0; i < metrics.Len(); i++ {
		metric := metrics.At(i)
		as.Equal(pmetric.MetricDataTypeGauge, metric.DataType())
	}

	acc.AddMetric(telegrafMetric)
	as.Equal(2, otelMetrics.ResourceMetrics().Len())

}

func Test_Accumulator_AddHistogramSum(t *testing.T) {
	t.Helper()

	as := assert.New(t)

	test_cases := []struct {
		name               string
		telegrafMetricType telegraf.ValueType
	}{
		{
			name:               "OtelAccumulator with AddHistogram",
			telegrafMetricType: telegraf.Histogram,
		},
		{
			name:               "OtelAccumulator with AddSummary",
			telegrafMetricType: telegraf.Summary,
		},
	}
	for _, tc := range test_cases {
		t.Run(tc.name, func(t *testing.T) {
			acc := newOtelAccumulatorWithTestRunningInputs(as)
			now := time.Now()
			telegrafMetricTags := map[string]string{defaultInstanceId: defaultInstanceIdValue}
			telegrafMetricFields := map[string]interface{}{"usage": uint32(20)}

			switch tc.telegrafMetricType {
			case telegraf.Histogram:
				acc.AddHistogram("acc_histogram_test", telegrafMetricFields, telegrafMetricTags, now)
			case telegraf.Summary:
				acc.AddSummary("acc_summary_test", telegrafMetricFields, telegrafMetricTags, now)
			}

			otelMetrics := acc.GetOtelMetrics()
			as.Equal(0, otelMetrics.ResourceMetrics().Len())
			as.Equal(pmetric.NewMetrics(), otelMetrics)
		})
	}
}

func Test_Accumulator_AddError(t *testing.T) {
	t.Helper()
	as := assert.New(t)

	acc := newOtelAccumulatorWithTestRunningInputs(as)
	acc.AddError(nil)
	acc.AddError(fmt.Errorf("foo"))
	acc.AddError(fmt.Errorf("bar"))
	acc.AddError(fmt.Errorf("baz"))

	// Output:
	// {"level":"error","msg":"Error with adapter","error":"foo"}
	// {"level":"error","msg":"Error with adapter","error":"bar"}
	// {"level":"error","msg":"Error with adapter","error":"baz"}
}
