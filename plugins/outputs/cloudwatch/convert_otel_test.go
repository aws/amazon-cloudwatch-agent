// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatch

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

const (
	keyPrefix          = "key"
	valPrefix          = "val"
	namePrefix         = "metric_name_"
	val        float64 = 24
)

func addDimensions(dp pmetric.NumberDataPoint, count int) {
	for i := 0; i < count; i++ {
		key := keyPrefix + strconv.Itoa(i)
		val := valPrefix + strconv.Itoa(i)
		dp.Attributes().PutStr(key, val)
	}
}

// createTestMetrics will create the numMetrics metrics.
// Each metric will have numDatapoint datapoints.
// Each dp will have numDimensions dimensions.
// Each metric will have the same unit, and value.
// But the value type will alternate between float and int.
// The metric data type will also alternative between gauge and sum.
// The timestamp on each datapoint will be the current time.
func createTestMetrics(
	numMetrics int,
	numDatapoints int,
	numDimensions int,
	unit string,
) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	for i := 0; i < numMetrics; i++ {
		m := sm.Metrics().AppendEmpty()
		m.SetDescription("my description")
		m.SetName(namePrefix + strconv.Itoa(i))
		m.SetUnit(unit)

		if i%2 == 0 {
			m.SetEmptyGauge()
		} else {
			m.SetEmptySum()
		}

		for j := 0; j < numDatapoints; j++ {
			var dp pmetric.NumberDataPoint
			if i%2 == 0 {
				dp = m.Gauge().DataPoints().AppendEmpty()
				dp.SetIntValue(int64(val))
			} else {
				dp = m.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleValue(val)
			}

			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			addDimensions(dp, numDimensions)
		}
	}

	return metrics
}

// checkDatum verified the given datum has the given unit as well as a
// hardcoded name, value, and dimension prefix.
func checkDatum(t *testing.T, d *cloudwatch.MetricDatum, unit string) {
	assert.True(t, strings.HasPrefix(*d.MetricName, namePrefix))
	assert.Equal(t, unit, *d.Unit)
	assert.Equal(t, val, *d.Value)
	// Assuming unit test does not take more than 1 s.
	assert.Less(t, time.Since(*d.Timestamp), time.Second)
	for _, dim := range d.Dimensions {
		assert.True(t, strings.HasPrefix(*dim.Name, keyPrefix))
		assert.True(t, strings.HasPrefix(*dim.Value, valPrefix))
	}
}

func TestConvertOtelMetrics_NoDimensions(t *testing.T) {
	c := CloudWatch{
		config: &Config{},
	}

	for i := 0; i < 100; i++ {
		metrics := createTestMetrics(i, i, 0, "other")
		datums := c.ConvertOtelMetrics(metrics)
		// Expect nummetrics * numDatapointsPerMetric
		assert.Equal(t, i*i, len(datums))

		// Verify dimensions per metric.
		for _, d := range datums {
			assert.Equal(t, 0, len(d.Dimensions))
			checkDatum(t, d, "None")
		}
	}
}

func TestConvertOtelMetrics_Dimensions(t *testing.T) {
	c := CloudWatch{
		config: &Config{},
	}

	for i := 0; i < 100; i++ {
		// 1 data point per metric, but vary the number dimensions.
		metrics := createTestMetrics(i, 1, i, "s")
		datums := c.ConvertOtelMetrics(metrics)
		// Expect nummetrics * numDatapointsPerMetric
		assert.Equal(t, i, len(datums))

		// Verify dimensions per metric.
		for _, d := range datums {
			expected := i
			if expected > 30 {
				expected = 30
			}
			assert.Equal(t, expected, len(d.Dimensions))
			checkDatum(t, d, "Seconds")
		}
	}
}

func TestInvalidMetric(t *testing.T) {
	c := CloudWatch{
		config: &Config{},
	}

	m := pmetric.NewMetric()
	m.SetName("name")
	m.SetUnit("unit")

	assert.Empty(t, c.ConvertOtelMetric(m))
}
