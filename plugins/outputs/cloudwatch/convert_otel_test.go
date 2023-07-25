// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatch

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/aws/amazon-cloudwatch-agent/metric/distribution"
	"github.com/aws/amazon-cloudwatch-agent/metric/distribution/regular"
)

const (
	keyPrefix      = "key"
	valPrefix      = "val"
	namePrefix     = "metric_name_"
	metricValue    = 24.0
	histogramMin   = 3.0
	histogramMax   = 37.0
	histogramSum   = 4095.0
	histogramCount = 987
)

func addDimensions(attributes pcommon.Map, count int) {
	for i := 0; i < count; i++ {
		key := keyPrefix + strconv.Itoa(i)
		val := valPrefix + strconv.Itoa(i)
		attributes.PutStr(key, val)
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
				dp.SetIntValue(int64(metricValue))
			} else {
				dp = m.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleValue(metricValue)
			}

			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			addDimensions(dp.Attributes(), numDimensions)
		}
	}

	return metrics
}

func createTestHistogram(
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
		m.SetEmptyHistogram()
		for j := 0; j < numDatapoints; j++ {
			dp := m.Histogram().DataPoints().AppendEmpty()
			// Make the values match the count so it is easy to verify.
			dp.ExplicitBounds().Append(float64(1 + i))
			dp.ExplicitBounds().Append(float64(2 + 2*i))
			dp.BucketCounts().Append(uint64(1 + i))
			dp.BucketCounts().Append(uint64(2 + 2*i))
			dp.SetMax(histogramMax)
			dp.SetMin(histogramMin)
			dp.SetSum(histogramSum)
			dp.SetCount(histogramCount)
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			addDimensions(dp.Attributes(), numDimensions)
		}
	}
	return metrics
}

// checkDatum verifies unit, name, value, and dimension prefix.
func checkDatum(
	t *testing.T,
	d *aggregationDatum,
	unit string,
	numMetrics int,
) {
	assert.True(t, strings.HasPrefix(*d.MetricName, namePrefix))
	assert.Equal(t, unit, *d.Unit)
	if d.distribution == nil {
		// Verify single metric value.
		assert.Equal(t, metricValue, *d.Value)
	} else {
		// Verify distribution
		assert.Equal(t, float64(histogramMax), d.distribution.Maximum())
		assert.Equal(t, float64(histogramMin), d.distribution.Minimum())
		assert.Equal(t, float64(histogramSum), d.distribution.Sum())
		assert.Equal(t, float64(histogramCount), d.distribution.SampleCount())
		values, counts := d.distribution.ValuesAndCounts()
		assert.Equal(t, 2, len(values))
		assert.Equal(t, 2, len(counts))
		// Expect values and counts to match.
		// Refer to how createTestHistogram() sets them.
		assert.Equal(t, values[0], counts[0])
		assert.Equal(t, values[1], counts[1])
	}

	// Assuming unit test does not take more than 1 s.
	assert.Less(t, time.Since(*d.Timestamp), time.Second)
	for _, dim := range d.Dimensions {
		assert.True(t, strings.HasPrefix(*dim.Name, keyPrefix))
		assert.True(t, strings.HasPrefix(*dim.Value, valPrefix))
	}
}

func TestConvertOtelMetrics_NoDimensions(t *testing.T) {
	for i := 0; i < 100; i++ {
		metrics := createTestMetrics(i, i, 0, "Bytes")
		datums := ConvertOtelMetrics(metrics)
		// Expect nummetrics * numDatapointsPerMetric
		assert.Equal(t, i*i, len(datums))

		for _, d := range datums {
			assert.Equal(t, 0, len(d.Dimensions))
			checkDatum(t, d, "Bytes", i)

		}
	}
}

func TestConvertOtelMetrics_Histogram(t *testing.T) {
	for i := 0; i < 5; i++ {
		if i%2 == 0 {
			//distribution.NewDistribution = seh1.NewSEH1Distribution
			distribution.NewDistribution = regular.NewRegularDistribution
		} else {
			distribution.NewDistribution = regular.NewRegularDistribution
		}
		metrics := createTestHistogram(i, i, 0, "Bytes")
		datums := ConvertOtelMetrics(metrics)
		// Expect nummetrics * numDatapointsPerMetric
		assert.Equal(t, i*i, len(datums))

		// Verify dimensions per metric.
		for _, d := range datums {
			assert.Equal(t, 0, len(d.Dimensions))
			checkDatum(t, d, "Bytes", i)
		}
	}
}

func TestConvertOtelMetrics_Dimensions(t *testing.T) {
	for i := 0; i < 100; i++ {
		// 1 data point per metric, but vary the number dimensions.
		metrics := createTestMetrics(i, 1, i, "s")
		datums := ConvertOtelMetrics(metrics)
		// Expect nummetrics * numDatapointsPerMetric
		assert.Equal(t, i, len(datums))

		// Verify dimensions per metric.
		for _, d := range datums {
			expected := i
			if expected > 30 {
				expected = 30
			}
			assert.Equal(t, expected, len(d.Dimensions))
			checkDatum(t, d, "Seconds", i)
		}
	}
}

func TestInvalidMetric(t *testing.T) {
	m := pmetric.NewMetric()
	m.SetName("name")
	m.SetUnit("unit")
	assert.Empty(t, ConvertOtelMetric(m))
}
