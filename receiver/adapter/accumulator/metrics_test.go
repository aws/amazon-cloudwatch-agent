// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package accumulator

import (
	"math/rand"
	"testing"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/testutil"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/aws/amazon-cloudwatch-agent/internal/metric"
	"github.com/aws/amazon-cloudwatch-agent/internal/util"
	"github.com/aws/amazon-cloudwatch-agent/metric/distribution/regular"
)

func Test_ConvertToOtelMetrics_WithDifferentTypes(t *testing.T) {
	t.Helper()

	as := assert.New(t)
	now := time.Now()

	test_cases := []struct {
		name                     string
		telegrafMetric           telegraf.Metric
		expectedOtelRMAttributes pcommon.Map
		expectedMetrics          []map[string]interface{}
	}{
		{
			name: "Convert Telegraf Gauge with Empty Tags and Empty Fields",
			telegrafMetric: testutil.MustMetric(
				"cpu",
				map[string]string{},
				map[string]interface{}{},
				now,
				telegraf.Gauge,
			),
			expectedMetrics: []map[string]interface{}{},
		},
		{
			name: "Convert Telegraf Gauge to Otel Gauge",
			telegrafMetric: testutil.MustMetric(
				"cpu",
				map[string]string{
					defaultInstanceId: defaultInstanceIdValue,
				},
				map[string]interface{}{
					"usage_user": float64(42),
				},
				now,
				telegraf.Gauge,
			),
			expectedMetrics: []map[string]interface{}{
				{
					"name":       metric.DecorateMetricName("cpu", "usage_user"),
					"value":      float64(42),
					"attributes": generateExpectedAttributes(),
					"timestamp":  pcommon.NewTimestampFromTime(now),
					"type":       pmetric.MetricTypeGauge,
					"unit":       "Percent",
				},
			},
		},
		{
			name: "Convert Telegraf Counter to Otel Sum",
			telegrafMetric: testutil.MustMetric(
				"swap",
				map[string]string{
					defaultInstanceId: defaultInstanceIdValue,
				},
				map[string]interface{}{
					"Sin": float64(3),
				},
				now.UTC(),
				telegraf.Counter,
			),
			expectedMetrics: []map[string]interface{}{
				{
					"name":       metric.DecorateMetricName("swap", "Sin"),
					"value":      float64(3),
					"attributes": generateExpectedAttributes(),
					"timestamp":  pcommon.NewTimestampFromTime(now),
					"type":       pmetric.MetricTypeSum,
					"unit":       "",
				},
			},
		},
		{

			name: "Convert Telegraf Untype to Otel Gauge",
			telegrafMetric: testutil.MustMetric(
				"prometheus",
				map[string]string{
					"instance_id": "mock",
				},
				map[string]interface{}{
					"redis_tx": int32(4),
					"redis_rx": 2.3,
				},
				now.UTC(),
				telegraf.Untyped,
			),
			expectedMetrics: []map[string]interface{}{
				{
					"name":       metric.DecorateMetricName("prometheus", "redis_tx"),
					"value":      int64(4),
					"attributes": generateExpectedAttributes(),
					"timestamp":  pcommon.NewTimestampFromTime(now),
					"type":       pmetric.MetricTypeGauge,
					"unit":       "",
				},
				{
					"name":       metric.DecorateMetricName("prometheus", "redis_rx"),
					"value":      2.3,
					"attributes": generateExpectedAttributes(),
					"timestamp":  pcommon.NewTimestampFromTime(now),
					"type":       pmetric.MetricTypeGauge,
					"unit":       "",
				},
			},
		},
	}
	for _, tc := range test_cases {
		t.Run(tc.name, func(t *testing.T) {

			convertedOtelMetrics, err := ConvertTelegrafToOtelMetrics(tc.telegrafMetric.Name(), tc.telegrafMetric.Fields(), tc.telegrafMetric.Tags(), tc.telegrafMetric.Type(), tc.telegrafMetric.Time())
			as.NoError(err)
			as.Equal(len(tc.expectedMetrics), convertedOtelMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())

			// Since Map is unordered; therefore, to avoid flakiness we have to loop through every metric
			matchMetrics := len(tc.expectedMetrics)
			for index, expectedDp := range tc.expectedMetrics {
				metrics := convertedOtelMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
				for metricIndex := 0; metricIndex < metrics.Len(); metricIndex++ {
					metric := metrics.At(metricIndex)
					// Check name to decrease the match metrics since metric name is the only unique attribute
					// And ignore the rest checking
					if tc.expectedMetrics[index]["name"] != metric.Name() {
						continue
					}

					matchMetrics--

					as.Equal(tc.expectedMetrics[index]["name"], metric.Name())
					as.Equal(tc.expectedMetrics[index]["type"], metric.Type())
					as.Equal(tc.expectedMetrics[index]["unit"], metric.Unit())
					var datapoint pmetric.NumberDataPoint
					switch tc.telegrafMetric.Type() {
					case telegraf.Counter:
						datapoint = metric.Sum().DataPoints().At(0)
					case telegraf.Gauge, telegraf.Untyped:
						datapoint = metric.Gauge().DataPoints().At(0)
					}

					value := expectedDp["value"]
					switch value.(type) {
					case int64:
						as.Equal(value, datapoint.IntValue())
					case float64:
						as.Equal(value, datapoint.DoubleValue())
					}
					as.Equal(tc.expectedMetrics[index]["attributes"], datapoint.Attributes())
					as.Equal(tc.expectedMetrics[index]["timestamp"], datapoint.Timestamp())
				}
			}
			as.Equal(0, matchMetrics)

		})
	}
}

func Test_ConvertTelegrafToOtelMetrics_WithUnsupportTyped(t *testing.T) {
	t.Helper()

	as := assert.New(t)
	tMetric := testutil.MustMetric(
		"prometheus",
		map[string]string{
			"instance_id": "mock",
		},
		map[string]interface{}{
			"redis_tx": int32(4),
			"redis_rx": int64(2),
		},
		time.Now().UTC(),
		telegraf.Summary,
	)

	convertedOtelMetrics, err := ConvertTelegrafToOtelMetrics(tMetric.Name(), tMetric.Fields(), tMetric.Tags(), tMetric.Type(), tMetric.Time())
	as.Error(err)
	as.Equal(pmetric.Metrics{}, convertedOtelMetrics)
}

func Test_PopulateNumberDataPoint_WithDifferentValueType(t *testing.T) {
	t.Helper()

	as := assert.New(t)

	test_cases := []struct {
		name                       string
		telegrafDataPointValue     interface{}
		expectedOtelDataPointValue interface{}
	}{
		{
			name:                       "Convert Telegraf Int to Otel Int64",
			telegrafDataPointValue:     int(42),
			expectedOtelDataPointValue: int64(42),
		},

		{
			name:                       "Convert Telegraf Int64 to Otel Int64",
			telegrafDataPointValue:     int64(5968846374),
			expectedOtelDataPointValue: int64(5968846374),
		},
		{
			name:                       "Convert Telegraf Uint to Otel Int64",
			telegrafDataPointValue:     uint(0),
			expectedOtelDataPointValue: int64(0),
		},

		{
			name:                       "Convert Telegraf Uint64 to Otel Int64",
			telegrafDataPointValue:     uint64(5968846374),
			expectedOtelDataPointValue: int64(5968846374),
		},
		{
			name:                       "Convert Telegraf Float32 to Otel Float64",
			telegrafDataPointValue:     float32(11234.500253),
			expectedOtelDataPointValue: float64(11234.5),
		},

		{
			name:                       "Convert Telegraf Float64 to Otel Float64",
			telegrafDataPointValue:     float64(2944405.500253),
			expectedOtelDataPointValue: float64(2944405.500253),
		},
	}

	for _, tc := range test_cases {
		t.Run(tc.name, func(t *testing.T) {

			otelValue, err := util.ToOtelValue(tc.telegrafDataPointValue)
			as.NoError(err)
			as.NotNil(otelValue)

			switch v := tc.expectedOtelDataPointValue.(type) {
			case int64:
				as.Equal(v, otelValue)
			case float64:
				as.Equal(v, otelValue)
			default:
				t.Fatalf("Invalid data type for datapoint %v", v)
			}
		})
	}
}

func TestProcstatDefaultUnit(t *testing.T) {
	testCases := []struct {
		category string
		field    string
		want     string
	}{
		{"procstat", "cpu_usage", "Percent"},
		{"procstat", "memory_data", "Bytes"},
		{"procstat", "memory_locked", "Bytes"},
		{"procstat", "memory_rss", "Bytes"},
		{"procstat", "memory_stack", "Bytes"},
		{"procstat", "memory_swap", "Bytes"},
		{"procstat", "memory_vms", "Bytes"},
		{"procstat", "read_bytes", "Bytes"},
		{"procstat", "write_bytes", "Bytes"},
		{"procstat", "rlimit_memory_data_hard", "Bytes"},
		{"procstat", "rlimit_memory_data_soft", "Bytes"},
		{"procstat", "rlimit_memory_locked_hard", "Bytes"},
		{"procstat", "rlimit_memory_locked_soft", "Bytes"},
		{"procstat", "rlimit_memory_rss_hard", "Bytes"},
		{"procstat", "rlimit_memory_rss_soft", "Bytes"},
		{"procstat", "rlimit_memory_stack_hard", "Bytes"},
		{"procstat", "rlimit_memory_stack_soft", "Bytes"},
		{"procstat", "rlimit_memory_vms_hard", "Bytes"},
		{"procstat", "rlimit_memory_vms_soft", "Bytes"},
	}

	for _, testCase := range testCases {
		tMetric := testutil.MustMetric(
			testCase.category,
			map[string]string{
				"instance_id": "mock",
			},
			map[string]interface{}{
				testCase.field: 4,
			},
			time.Now().UTC(),
			telegraf.Gauge,
		)
		convertedOtelMetrics, err := ConvertTelegrafToOtelMetrics(tMetric.Name(), tMetric.Fields(), tMetric.Tags(), tMetric.Type(), tMetric.Time())
		assert.NoError(t, err)
		assert.Equal(t, 1, convertedOtelMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())

		got := convertedOtelMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
		assert.Equal(t, testCase.want, got.Unit())
	}
}

func TestPopulateDataPointsForHistogram(t *testing.T) {
	metricName := "MyMetric"
	fieldName := "MyField"
	timestamp := pcommon.NewTimestampFromTime(time.Now())
	tags := map[string]string{}
	fields := map[string]interface{}{}
	dist := regular.NewRegularDistribution()
	fields[fieldName] = dist
	// Random data
	for i := 0; i < 1000; i++ {
		dist.AddEntry(rand.Float64()*1000, float64(1+rand.Intn(1000)))
	}
	values, counts := dist.ValuesAndCounts()
	otelMetrics := pmetric.NewMetrics().ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()

	populateDataPointsForHistogram(metricName, otelMetrics, fields, tags, timestamp)

	assert.Equal(t, 1, otelMetrics.Len())
	// Assume there is a data point.
	dp := otelMetrics.At(0).Histogram().DataPoints().At(0)
	assert.Equal(t, len(counts), dp.BucketCounts().Len())
	assert.Equal(t, len(values), dp.ExplicitBounds().Len())
	assert.Equal(t, dist.Minimum(), dp.Min())
	assert.Equal(t, dist.Maximum(), dp.Max())
	assert.Equal(t, dist.Sum(), dp.Sum())
}
