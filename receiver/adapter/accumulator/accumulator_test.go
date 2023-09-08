// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package accumulator

import (
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"testing"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/models"
	"github.com/influxdata/telegraf/testutil"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/aws/amazon-cloudwatch-agent/metric/distribution/regular"
)

func Test_Accumulator_AddCounterGaugeFields(t *testing.T) {
	t.Helper()

	as := assert.New(t)

	testCases := []struct {
		name                   string
		telegrafMetricName     string
		telegrafMetricTags     map[string]string
		telegrafMetricType     telegraf.ValueType
		expectedOtelMetricType pmetric.MetricType
		expectedDPAttributes   pcommon.Map
		isServiceInput         bool
	}{
		{
			name:                   "OtelAccumulator with AddGauge",
			telegrafMetricName:     "acc_gauge_test",
			telegrafMetricTags:     map[string]string{defaultInstanceId: defaultInstanceIdValue},
			telegrafMetricType:     telegraf.Gauge,
			expectedOtelMetricType: pmetric.MetricTypeGauge,
			expectedDPAttributes:   generateExpectedAttributes(),
			isServiceInput:         false,
		},
		{
			name:                   "OtelAccumulator with AddCounter",
			telegrafMetricName:     "acc_counter_test",
			telegrafMetricTags:     map[string]string{defaultInstanceId: defaultInstanceIdValue},
			telegrafMetricType:     telegraf.Counter,
			expectedOtelMetricType: pmetric.MetricTypeSum,
			expectedDPAttributes:   generateExpectedAttributes(),
			isServiceInput:         false,
		},
		{
			name:                   "OtelAccumulator with AddFields",
			telegrafMetricName:     "acc_field_test",
			telegrafMetricTags:     map[string]string{defaultInstanceId: defaultInstanceIdValue},
			telegrafMetricType:     telegraf.Untyped,
			expectedOtelMetricType: pmetric.MetricTypeGauge,
			expectedDPAttributes:   generateExpectedAttributes(),
			isServiceInput:         false,
		},
		{
			name:                   "OtelAccumulator with AddGauge For ServiceInput",
			telegrafMetricName:     "acc_gauge_test",
			telegrafMetricTags:     map[string]string{defaultInstanceId: defaultInstanceIdValue},
			telegrafMetricType:     telegraf.Gauge,
			expectedOtelMetricType: pmetric.MetricTypeGauge,
			expectedDPAttributes:   generateExpectedAttributes(),
			isServiceInput:         true,
		},
		{
			name:                   "OtelAccumulator with AddCounter For ServiceInput",
			telegrafMetricName:     "acc_counter_test",
			telegrafMetricTags:     map[string]string{defaultInstanceId: defaultInstanceIdValue},
			telegrafMetricType:     telegraf.Counter,
			expectedOtelMetricType: pmetric.MetricTypeSum,
			expectedDPAttributes:   generateExpectedAttributes(),
			isServiceInput:         true,
		},
		{
			name:                   "OtelAccumulator with AddFields For ServiceInput",
			telegrafMetricName:     "acc_field_test",
			telegrafMetricTags:     map[string]string{defaultInstanceId: defaultInstanceIdValue},
			telegrafMetricType:     telegraf.Untyped,
			expectedOtelMetricType: pmetric.MetricTypeGauge,
			expectedDPAttributes:   generateExpectedAttributes(),
			isServiceInput:         true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(_ *testing.T) {

			sink := new(consumertest.MetricsSink)
			acc := newOtelAccumulatorWithTestRunningInputs(as, sink, tc.isServiceInput)

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
			var otelMetrics pmetric.Metrics
			if tc.isServiceInput {
				as.Len(sink.AllMetrics(), 1)
				otelMetrics = sink.AllMetrics()[0]
			} else {
				as.Len(sink.AllMetrics(), 0)
				otelMetrics = acc.GetOtelMetrics()
			}

			metrics := otelMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
			as.Equal(2, metrics.Len())

			for i := 0; i < metrics.Len(); i++ {
				metric := metrics.At(i)
				as.Equal(tc.expectedOtelMetricType, metric.Type())
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

func TestAddHistogram(t *testing.T) {
	name := "banana"
	now := time.Now()
	dist := regular.NewRegularDistribution()
	// Random data
	for i := 0; i < 1000; i++ {
		dist.AddEntry(rand.Float64()*1000, float64(1+rand.Intn(1000)))
	}
	fields := map[string]interface{}{}
	fields["peel"] = dist
	tags := map[string]string{defaultInstanceId: defaultInstanceIdValue}
	as := assert.New(t)
	acc := newOtelAccumulatorWithTestRunningInputs(as, nil, false)

	acc.AddHistogram(name, fields, tags, now)

	otelMetrics := acc.GetOtelMetrics().ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	as.Equal(1, otelMetrics.Len())
	m := otelMetrics.At(0)
	as.Equal(pmetric.MetricTypeHistogram, m.Type())
	if runtime.GOOS == "windows" {
		as.Equal("banana peel", m.Name())
	} else {
		as.Equal("banana_peel", m.Name())
	}
	dp := m.Histogram().DataPoints().At(0)
	as.Equal(1, dp.Attributes().Len())
	as.Equal(dist.Minimum(), dp.Min())
	as.Equal(dist.Maximum(), dp.Max())
	as.Equal(dist.Sum(), dp.Sum())
	as.Equal(dist.SampleCount(), float64(dp.Count()))
}

func Test_Accumulator_WithUnsupportedValueAndEmptyFields(t *testing.T) {
	t.Helper()

	as := assert.New(t)

	acc := newOtelAccumulatorWithTestRunningInputs(as, nil, false)

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

func Test_ModifyMetricAndConvertMetricValue(t *testing.T) {
	as := assert.New(t)
	cfg := &models.InputConfig{
		Filter: models.Filter{
			FieldDrop: []string{"filtered_field"},
		},
	}
	acc := newOtelAccumulatorWithConfig(as, nil, false, cfg)

	testCases := map[string]struct {
		metric            telegraf.Metric
		wantErrStr        string
		wantFields        map[string]interface{}
		wantDroppedFields []string
	}{
		"WithEmpty": {
			metric: testutil.MustMetric(
				"cpu",
				map[string]string{},
				map[string]interface{}{},
				time.Now(),
				telegraf.Gauge,
			),
		},
		"WithFiltered": {
			metric: testutil.MustMetric(
				"cpu",
				map[string]string{},
				map[string]interface{}{
					"filtered_field": 1,
				},
				time.Now(),
				telegraf.Gauge,
			),
		},
		"WithInvalidConvert": {
			metric: testutil.MustMetric(
				"cpu",
				map[string]string{},
				map[string]interface{}{
					"client": "redis",
					"nan":    math.NaN(),
				},
				time.Now(),
				telegraf.Gauge,
			),
			wantErrStr: "empty metrics after converting fields",
		},
		"WithValid": {
			metric: testutil.MustMetric(
				"cpu",
				map[string]string{
					"instance_id": "mock",
				},
				map[string]interface{}{
					"tx":     4.5,
					"rx":     int32(3),
					"error":  false,
					"client": "redis",
				},
				time.Now(),
				telegraf.Gauge,
			),
			wantFields: map[string]interface{}{
				"tx":    4.5,
				"rx":    int64(3),
				"error": int64(0),
			},
			wantDroppedFields: []string{"client"},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := acc.modifyMetricAndConvertToOtelValue(testCase.metric)
			if testCase.wantErrStr != "" {
				as.Error(err)
				as.ErrorContains(err, testCase.wantErrStr)
				as.Nil(got)
			} else {
				as.NoError(err)
				for field, wantValue := range testCase.wantFields {
					value, ok := got.GetField(field)
					as.True(ok)
					as.Equal(wantValue, value)
				}
				for _, field := range testCase.wantDroppedFields {
					_, ok := got.GetField(field)
					as.False(ok)
				}
			}
		})
	}
}

func Test_Accumulator_AddMetric(t *testing.T) {
	t.Helper()

	as := assert.New(t)

	acc := newOtelAccumulatorWithTestRunningInputs(as, nil, false)

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
		as.Equal(pmetric.MetricTypeGauge, metric.Type())
	}

	acc.AddMetric(telegrafMetric)
	as.Equal(2, otelMetrics.ResourceMetrics().Len())

}

func Test_Accumulator_AddMetric_ServiceInput(t *testing.T) {
	t.Helper()

	as := assert.New(t)

	sink := new(consumertest.MetricsSink)
	acc := newOtelAccumulatorWithTestRunningInputs(as, sink, true)

	telegrafMetric := testutil.MustMetric(
		"acc_metric_test",
		map[string]string{defaultInstanceId: defaultInstanceIdValue},
		map[string]interface{}{"sin": int32(4)}, time.Now().UTC(),
		telegraf.Untyped)

	acc.SetPrecision(time.Microsecond)
	acc.AddMetric(telegrafMetric)
	acc.AddMetric(telegrafMetric)

	otelMetrics := sink.AllMetrics()
	as.Len(otelMetrics, 2)
	as.Equal(1, otelMetrics[0].ResourceMetrics().Len())
	as.Equal(1, otelMetrics[0].ResourceMetrics().At(0).ScopeMetrics().Len())
	as.Equal(1, otelMetrics[1].ResourceMetrics().Len())
	as.Equal(1, otelMetrics[1].ResourceMetrics().At(0).ScopeMetrics().Len())

	pMetrics := pmetric.NewMetricSlice()
	otelMetrics[0].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().MoveAndAppendTo(pMetrics)
	otelMetrics[1].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().MoveAndAppendTo(pMetrics)
	as.Equal(2, pMetrics.Len())

	for i := 0; i < pMetrics.Len(); i++ {
		metric := pMetrics.At(i)
		as.Equal(pmetric.MetricTypeGauge, metric.Type())
	}

	acc.AddMetric(telegrafMetric)
	as.Len(sink.AllMetrics(), 3)

	as.Equal(pmetric.NewMetrics(), acc.GetOtelMetrics())
}

func Test_Accumulator_AddSum(t *testing.T) {
	t.Helper()
	as := assert.New(t)
	acc := newOtelAccumulatorWithTestRunningInputs(as, nil, false)
	now := time.Now()
	telegrafMetricTags := map[string]string{defaultInstanceId: defaultInstanceIdValue}
	telegrafMetricFields := map[string]interface{}{"usage": uint32(20)}

	acc.AddSummary("acc_summary_test", telegrafMetricFields, telegrafMetricTags, now)

	otelMetrics := acc.GetOtelMetrics()
	as.Equal(0, otelMetrics.ResourceMetrics().Len())
	as.Equal(pmetric.NewMetrics(), otelMetrics)
}

func Test_Accumulator_AddError(t *testing.T) {
	t.Helper()
	as := assert.New(t)

	acc := newOtelAccumulatorWithTestRunningInputs(as, nil, false)
	acc.AddError(nil)
	acc.AddError(fmt.Errorf("foo"))
	acc.AddError(fmt.Errorf("bar"))
	acc.AddError(fmt.Errorf("baz"))

	// Output:
	// {"level":"error","msg":"Error with adapter","error":"foo"}
	// {"level":"error","msg":"Error with adapter","error":"bar"}
	// {"level":"error","msg":"Error with adapter","error":"baz"}
}
