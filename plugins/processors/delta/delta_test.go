// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package delta

import (
	"testing"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/metric"
	"github.com/stretchr/testify/assert"
)

func deepCopy(original map[string]string) map[string]string {
	clone := make(map[string]string)
	for key, value := range original {
		clone[key] = value
	}
	return clone
}

func createTestMetric(reportDeltaTag string, ignoredFieldsTag string) []telegraf.Metric {
	tags := map[string]string{
		"metric_tag":               "from_metric",
		"report_deltas":            reportDeltaTag,
		"ignored_fields_for_delta": ignoredFieldsTag,
	}
	if reportDeltaTag == "NULL" {
		delete(tags, "report_deltas")
	}
	if ignoredFieldsTag == "NULL" {
		delete(tags, "ignored_fields_for_delta")
	}
	metric1 := metric.New("m1",
		deepCopy(tags),
		map[string]interface{}{
			"value1": int64(1),
			"value2": uint64(200),
			"value3": float64(20.0),
			"value4": int64(5),
		},
		time.Now(),
	)

	metric2 := metric.New("m1",
		deepCopy(tags),
		map[string]interface{}{
			"value1": int64(10),
			"value2": uint64(300),
			"value3": float64(40.0),
			"value4": int64(9),
		},
		time.Now(),
	)
	metric3 := metric.New("m1",
		deepCopy(tags),
		map[string]interface{}{
			"value1": int64(10),
			"value2": uint64(200),
			"value3": float64(30.0),
			"value4": int64(13),
		},
		time.Now(),
	)
	metric4 := metric.New("m1",
		deepCopy(tags),
		map[string]interface{}{
			"value1": int64(7),
			"value2": uint64(150),
			"value3": float64(33.0),
			"value4": int64(19),
		},
		time.Now(),
	)
	metric5 := metric.New("m1",
		deepCopy(tags),
		map[string]interface{}{
			"value1": int64(8),
			"value2": uint64(175),
			"value3": float64(46.0),
			"value4": int64(25),
		},
		time.Now(),
	)
	return []telegraf.Metric{metric1, metric2, metric3, metric4, metric5}
}

func computeDiffInt64(prevMetric, curMetric telegraf.Metric, fieldKey string) int64 {
	prev, _ := prevMetric.GetField(fieldKey)
	cur, _ := curMetric.GetField(fieldKey)
	pv, _ := prev.(int64)
	cv, _ := cur.(int64)
	return cv - pv
}

func computeDiffUint64(prevMetric, curMetric telegraf.Metric, fieldKey string) uint64 {
	prev, _ := prevMetric.GetField(fieldKey)
	cur, _ := curMetric.GetField(fieldKey)
	pv, _ := prev.(uint64)
	cv, _ := cur.(uint64)
	return cv - pv
}

func computeDiffFloat64(prevMetric, curMetric telegraf.Metric, fieldKey string) float64 {
	prev, _ := prevMetric.GetField(fieldKey)
	cur, _ := curMetric.GetField(fieldKey)
	pv, _ := prev.(float64)
	cv, _ := cur.(float64)
	return cv - pv
}

func checkValueInt64(t *testing.T, metric telegraf.Metric, fieldKey string, expected int64) {
	value, ok := metric.GetField(fieldKey)
	if !ok {
		assert.Fail(t, "Value is not set")
	}
	v, ok := value.(int64)
	if !ok {
		assert.Fail(t, "Value is of wrong type")
	}
	assert.Equal(t, expected, int64(v))
}

func checkValueUint64(t *testing.T, metric telegraf.Metric, fieldKey string, expected uint64) {
	value, ok := metric.GetField(fieldKey)
	if !ok {
		assert.Fail(t, "Value is not set")
	}
	v, ok := value.(uint64)
	if !ok {
		assert.Fail(t, "Value is of wrong type")
	}
	assert.Equal(t, expected, uint64(v))
}

func checkValueFloat64(t *testing.T, metric telegraf.Metric, fieldKey string, expected float64) {
	value, ok := metric.GetField(fieldKey)
	if !ok {
		assert.Fail(t, "Value is not set")
	}
	v, ok := value.(float64)
	if !ok {
		assert.Fail(t, "Value is of wrong type")
	}
	assert.Equal(t, expected, float64(v))
}

func TestReportDeltaWithSingleIgnoredField(t *testing.T) {
	processor := Delta{make(map[uint64]*metricFields)}
	input := createTestMetric("true", "value4")
	inputCopy := make([]telegraf.Metric, len(input))
	for i, metric := range input {
		inputCopy[i] = metric.Copy()
	}

	metrics := processor.Apply(input...)

	assert.Equal(t, len(inputCopy)-1, len(metrics))
	for i, metric := range metrics {
		diff1 := computeDiffInt64(inputCopy[i], inputCopy[i+1], "value1")
		checkValueInt64(t, metric, "value1", diff1)
		diff2 := computeDiffUint64(inputCopy[i], inputCopy[i+1], "value2")
		checkValueUint64(t, metric, "value2", diff2)
		diff3 := computeDiffFloat64(inputCopy[i], inputCopy[i+1], "value3")
		checkValueFloat64(t, metric, "value3", diff3)
		fv, _ := inputCopy[i+1].GetField("value4")
		prev, _ := fv.(int64)
		checkValueInt64(t, metric, "value4", prev)
		assert.Equal(t, metric.Time(), inputCopy[i+1].Time())
		assert.False(t, metric.HasTag(ReportDelta))
		assert.False(t, metric.HasTag(IgnoredFieldsForDelta))
	}
}

func TestReportDeltaWithTwoIgnoredFields(t *testing.T) {
	processor := Delta{make(map[uint64]*metricFields)}
	input := createTestMetric("true", "value1,value4")
	inputCopy := make([]telegraf.Metric, len(input))
	for i, metric := range input {
		inputCopy[i] = metric.Copy()
	}

	metrics := processor.Apply(input...)

	assert.Equal(t, len(inputCopy)-1, len(metrics))
	for i, metric := range metrics {
		fv1, _ := inputCopy[i+1].GetField("value1")
		prev1, _ := fv1.(int64)
		checkValueInt64(t, metric, "value1", prev1)
		diff2 := computeDiffUint64(inputCopy[i], inputCopy[i+1], "value2")
		checkValueUint64(t, metric, "value2", diff2)
		diff3 := computeDiffFloat64(inputCopy[i], inputCopy[i+1], "value3")
		checkValueFloat64(t, metric, "value3", diff3)
		fv4, _ := inputCopy[i+1].GetField("value4")
		prev4, _ := fv4.(int64)
		checkValueInt64(t, metric, "value4", prev4)
		assert.Equal(t, metric.Time(), inputCopy[i+1].Time())
		assert.False(t, metric.HasTag(ReportDelta))
		assert.False(t, metric.HasTag(IgnoredFieldsForDelta))
	}
}

func TestNotReportDelta(t *testing.T) {
	processor := Delta{make(map[uint64]*metricFields)}
	input := createTestMetric("false", "value1,value4")
	metrics := processor.Apply(input...)

	assert.Equal(t, len(input), len(metrics))
	for i := range input {
		assert.True(t, true, input[i] == metrics[i])
	}
}

func TestReportDeltaWithAllFieldsIgnored(t *testing.T) {
	processor := Delta{make(map[uint64]*metricFields)}
	input := createTestMetric("true", "value4,value1,value3,value2")
	inputCopy := make([]telegraf.Metric, len(input))
	for i, metric := range input {
		inputCopy[i] = metric.Copy()
	}

	metrics := processor.Apply(input...)

	assert.Equal(t, len(inputCopy)-1, len(metrics))
	for i, metric := range metrics {
		fv1, _ := inputCopy[i+1].GetField("value1")
		prev1, _ := fv1.(int64)
		checkValueInt64(t, metric, "value1", prev1)
		fv2, _ := inputCopy[i+1].GetField("value2")
		prev2, _ := fv2.(uint64)
		checkValueUint64(t, metric, "value2", prev2)
		fv3, _ := inputCopy[i+1].GetField("value3")
		prev3, _ := fv3.(float64)
		checkValueFloat64(t, metric, "value3", prev3)
		fv4, _ := inputCopy[i+1].GetField("value4")
		prev4, _ := fv4.(int64)
		checkValueInt64(t, metric, "value4", prev4)
		assert.Equal(t, metric.Time(), inputCopy[i+1].Time())
		assert.False(t, metric.HasTag(ReportDelta))
		assert.False(t, metric.HasTag(IgnoredFieldsForDelta))
	}
}

func TestReportDeltaWithOneMetricAndWithMore(t *testing.T) {
	processor := Delta{make(map[uint64]*metricFields)}
	input := createTestMetric("true", "value4")
	inputCopy := make([]telegraf.Metric, len(input))
	for i, metric := range input {
		inputCopy[i] = metric.Copy()
	}

	//process only one metric at the beginning, should return nothing
	oneMetric := input[0:1]
	metrics := processor.Apply(oneMetric...)
	assert.Equal(t, 0, len(metrics))

	//continue to process more metric
	moreMetrics := input[1:]
	metrics = processor.Apply(moreMetrics...)
	assert.Equal(t, len(inputCopy)-1, len(metrics))
	for i, metric := range metrics {
		diff1 := computeDiffInt64(inputCopy[i], inputCopy[i+1], "value1")
		checkValueInt64(t, metric, "value1", diff1)
		diff2 := computeDiffUint64(inputCopy[i], inputCopy[i+1], "value2")
		checkValueUint64(t, metric, "value2", diff2)
		diff3 := computeDiffFloat64(inputCopy[i], inputCopy[i+1], "value3")
		checkValueFloat64(t, metric, "value3", diff3)
		fv, _ := inputCopy[i+1].GetField("value4")
		prev, _ := fv.(int64)
		checkValueInt64(t, metric, "value4", prev)
		assert.Equal(t, metric.Time(), inputCopy[i+1].Time())
		assert.False(t, metric.HasTag(ReportDelta))
		assert.False(t, metric.HasTag(IgnoredFieldsForDelta))
	}
}

func TestReportDeltaWithRandomIgnoredFields(t *testing.T) {
	processor := Delta{make(map[uint64]*metricFields)}
	input := createTestMetric("true", "xwyfa;g,afthqt")
	inputCopy := make([]telegraf.Metric, len(input))
	for i, metric := range input {
		inputCopy[i] = metric.Copy()
	}

	metrics := processor.Apply(input...)

	assert.Equal(t, len(inputCopy)-1, len(metrics))
	for i, metric := range metrics {
		diff1 := computeDiffInt64(inputCopy[i], inputCopy[i+1], "value1")
		checkValueInt64(t, metric, "value1", diff1)
		diff2 := computeDiffUint64(inputCopy[i], inputCopy[i+1], "value2")
		checkValueUint64(t, metric, "value2", diff2)
		diff3 := computeDiffFloat64(inputCopy[i], inputCopy[i+1], "value3")
		checkValueFloat64(t, metric, "value3", diff3)
		diff4 := computeDiffInt64(inputCopy[i], inputCopy[i+1], "value4")
		checkValueInt64(t, metric, "value4", diff4)
		assert.Equal(t, metric.Time(), inputCopy[i+1].Time())
		assert.False(t, metric.HasTag(ReportDelta))
		assert.False(t, metric.HasTag(IgnoredFieldsForDelta))
	}
}

func TestWithRandomTags(t *testing.T) {
	processor := Delta{make(map[uint64]*metricFields)}
	input := createTestMetric("afnbqltyqm&5q529", "xwyfa;g,afthqt")
	metrics := processor.Apply(input...)

	assert.Equal(t, len(input), len(metrics))
	for i := range input {
		assert.True(t, true, input[i] == metrics[i])
	}
}

func TestWithNoTags(t *testing.T) {
	processor := Delta{make(map[uint64]*metricFields)}
	input := createTestMetric("NULL", "NULL")
	metrics := processor.Apply(input...)

	assert.Equal(t, len(input), len(metrics))
	for i := range input {
		assert.True(t, true, input[i] == metrics[i])
	}
}

func TestWithOnlyIgnoredFieldsTag(t *testing.T) {
	processor := Delta{make(map[uint64]*metricFields)}
	input := createTestMetric("NULL", "value4")
	metrics := processor.Apply(input...)

	assert.Equal(t, len(input), len(metrics))
	for i := range input {
		assert.True(t, true, input[i] == metrics[i])
	}
}

func TestWithOnlyReportDeltaTag(t *testing.T) {
	processor := Delta{make(map[uint64]*metricFields)}
	input := createTestMetric("true", "NULL")
	inputCopy := make([]telegraf.Metric, len(input))
	for i, metric := range input {
		inputCopy[i] = metric.Copy()
	}

	metrics := processor.Apply(input...)

	assert.Equal(t, len(inputCopy)-1, len(metrics))
	for i, metric := range metrics {
		diff1 := computeDiffInt64(inputCopy[i], inputCopy[i+1], "value1")
		checkValueInt64(t, metric, "value1", diff1)
		diff2 := computeDiffUint64(inputCopy[i], inputCopy[i+1], "value2")
		checkValueUint64(t, metric, "value2", diff2)
		diff3 := computeDiffFloat64(inputCopy[i], inputCopy[i+1], "value3")
		checkValueFloat64(t, metric, "value3", diff3)
		diff4 := computeDiffInt64(inputCopy[i], inputCopy[i+1], "value4")
		checkValueInt64(t, metric, "value4", diff4)
		assert.Equal(t, metric.Time(), inputCopy[i+1].Time())
		assert.False(t, metric.HasTag(ReportDelta))
		assert.False(t, metric.HasTag(IgnoredFieldsForDelta))
	}
}
