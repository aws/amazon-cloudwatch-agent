// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatch

import (
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/aws/cloudwatch/histograms"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/aws/amazon-cloudwatch-agent/metric/distribution"
	"github.com/aws/amazon-cloudwatch-agent/metric/distribution/regular"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsentity/entityattributes"
	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatch"
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
	rm.Resource().Attributes().PutStr(entityattributes.AttributeEntityType, "Service")
	rm.Resource().Attributes().PutStr(entityattributes.AttributeEntityDeploymentEnvironment, "MyEnvironment")
	rm.Resource().Attributes().PutStr(entityattributes.AttributeEntityServiceName, "MyServiceName")
	rm.Resource().Attributes().PutStr(entityattributes.AttributeEntityInstanceID, "i-123456789")
	rm.Resource().Attributes().PutStr(entityattributes.AttributeEntityAwsAccountId, "0123456789012")
	rm.Resource().Attributes().PutStr(entityattributes.AttributeEntityAutoScalingGroup, "asg-123")
	rm.Resource().Attributes().PutStr(entityattributes.AttributeEntityPlatformType, "AWS::EC2")

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

func createTestExponentialHistogram(
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
		m.SetEmptyExponentialHistogram()
		for j := 0; j < numDatapoints; j++ {
			dp := m.ExponentialHistogram().DataPoints().AppendEmpty()
			// Make the values match the count so it is easy to verify.
			dp.SetMax(histogramMax)
			dp.SetMin(histogramMin)
			dp.SetSum(histogramSum)
			dp.SetCount(histogramCount)
			dp.SetZeroCount(5)
			dp.SetScale(0)
			dp.Positive().SetOffset(0)
			dp.Positive().BucketCounts().FromRaw([]uint64{
				4, 2, 1,
			})
			dp.Negative().SetOffset(0)
			dp.Negative().BucketCounts().FromRaw([]uint64{
				4, 2, 1,
			})

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
	if d.distribution != nil {
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
	} else if d.histogram != nil {
		// Verify histogram
		assert.Equal(t, float64(histogramMax), d.histogram.Max())
		assert.Equal(t, float64(histogramMin), d.histogram.Min())
		assert.Equal(t, float64(histogramSum), d.histogram.Sum())
		assert.Equal(t, uint64(histogramCount), d.histogram.Count())
		cwhist := histograms.ConvertOTelToCloudWatch(*d.histogram)
		values, counts := cwhist.ValuesAndCounts()
		assert.Equal(t, len(values), len(counts))
	} else {
		// Verify single metric value.
		assert.Equal(t, metricValue, *d.Value)
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
			distribution.NewClassicDistribution = regular.NewRegularDistribution
		} else {
			distribution.NewClassicDistribution = regular.NewRegularDistribution
		}
		metrics := createTestHistogram(i, i, 0, "Bytes")
		datums := ConvertOtelMetrics(metrics)
		// Expect nummetrics * numDatapointsPerMetric
		assert.Equal(t, i*i, len(datums))

		// Verify dimensions per metric.
		for _, d := range datums {
			assert.Len(t, d.Dimensions, 0)
			checkDatum(t, d, "Bytes", i)
		}
	}
}

func TestConvertOtelMetrics_ExponentialHistogram(t *testing.T) {
	for i := 0; i < 5; i++ {
		metrics := createTestExponentialHistogram(i, i, 0, "Bytes")
		datums := ConvertOtelMetrics(metrics)
		// Expect nummetrics * numDatapointsPerMetric
		assert.Equal(t, i*i, len(datums))

		// Verify dimensions per metric.
		for _, d := range datums {
			assert.True(t, strings.HasPrefix(*d.MetricName, namePrefix))
			assert.Equal(t, "Bytes", *d.Unit)
			assert.Equal(t, 0, len(d.Dimensions))
			// Verify distribution
			assert.Equal(t, float64(histogramMax), d.distribution.Maximum())
			assert.Equal(t, float64(histogramMin), d.distribution.Minimum())
			assert.Equal(t, float64(histogramSum), d.distribution.Sum())
			assert.Equal(t, float64(histogramCount), d.distribution.SampleCount())
			values, counts := d.distribution.ValuesAndCounts()
			assert.Equal(t, 7, len(values))
			assert.Equal(t, 7, len(counts))
			// Refer to how createTestExponentialHistogram() sets them.
			assert.Equal(t, []float64{6.0, 3.0, 1.5, 0, -1.5, -3.0, -6.0}, values)
			assert.Equal(t, []float64{1, 2, 4, 5, 4, 2, 1}, counts)

			// Assuming unit test does not take more than 1 s.
			assert.Less(t, time.Since(*d.Timestamp), time.Second)
			for _, dim := range d.Dimensions {
				assert.True(t, strings.HasPrefix(*dim.Name, keyPrefix))
				assert.True(t, strings.HasPrefix(*dim.Value, valPrefix))
			}
		}
	}
}

func TestConvertOtelExponentialHistogram(t *testing.T) {
	ts := time.Date(2025, time.March, 31, 22, 6, 30, 0, time.UTC)
	entity := cloudwatch.Entity{
		KeyAttributes: map[string]*string{
			"Type":         aws.String("Service"),
			"Environment":  aws.String("MyEnvironment"),
			"Name":         aws.String("MyServiceName"),
			"AwsAccountId": aws.String("0123456789012"),
		},
		Attributes: map[string]*string{
			"EC2.InstanceId":       aws.String("i-123456789"),
			"PlatformType":         aws.String("AWS::EC2"),
			"EC2.AutoScalingGroup": aws.String("asg-123"),
		},
	}

	testCases := []struct {
		name           string
		histogramDPS   pmetric.ExponentialHistogramDataPointSlice
		expected       []*aggregationDatum
		expectedValues [][]float64
		expectedCounts [][]float64
	}{

		{
			name: "Exponential histogram with positive buckets",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				posBucketCounts := make([]uint64, 10)
				for i := range posBucketCounts {
					posBucketCounts[i] = uint64(i) //nolint:gosec
				}
				histogramDP.Positive().BucketCounts().FromRaw(posBucketCounts)
				histogramDP.SetSum(1000)
				histogramDP.SetMin(2)
				histogramDP.SetMax(800)
				histogramDP.SetCount(uint64(55))
				histogramDP.SetScale(0)
				histogramDP.Attributes().PutStr("label1", "value1")
				histogramDP.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				return histogramDPS
			}(),
			expected: []*aggregationDatum{
				{
					MetricDatum: cloudwatch.MetricDatum{
						Dimensions: []*cloudwatch.Dimension{
							{Name: aws.String("label1"), Value: aws.String("value1")},
						},
						MetricName:        aws.String("foo"),
						Unit:              aws.String("none"),
						Timestamp:         aws.Time(ts),
						StorageResolution: aws.Int64(60),
						StatisticValues: &cloudwatch.StatisticSet{
							Maximum:     aws.Float64(800),
							Minimum:     aws.Float64(2),
							SampleCount: aws.Float64(55),
							Sum:         aws.Float64(1000),
						},
					},
					entity:              entity,
					aggregationInterval: 0,
				},
			},
			expectedValues: [][]float64{
				{
					768, 384, 192, 96, 48, 24, 12, 6, 3, 1.5,
				},
			},
			expectedCounts: [][]float64{
				{
					9, 8, 7, 6, 5, 4, 3, 2, 1, 0,
				},
			},
		},
		{
			name: "Exponential histogram with negative buckets",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				negBucketCounts := make([]uint64, 10)
				for i := range negBucketCounts {
					negBucketCounts[i] = uint64(i) //nolint:gosec
				}
				histogramDP.Negative().BucketCounts().FromRaw(negBucketCounts)
				histogramDP.SetSum(-1000)
				histogramDP.SetMin(-800)
				histogramDP.SetMax(0)
				histogramDP.SetCount(uint64(55))
				histogramDP.SetScale(0)
				histogramDP.Attributes().PutStr("label1", "value1")
				histogramDP.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				return histogramDPS
			}(),
			expected: []*aggregationDatum{
				{
					MetricDatum: cloudwatch.MetricDatum{
						Dimensions: []*cloudwatch.Dimension{
							{Name: aws.String("label1"), Value: aws.String("value1")},
						},
						MetricName:        aws.String("foo"),
						Unit:              aws.String("none"),
						Timestamp:         aws.Time(ts),
						StorageResolution: aws.Int64(60),
						StatisticValues: &cloudwatch.StatisticSet{
							Maximum:     aws.Float64(0),
							Minimum:     aws.Float64(-800),
							SampleCount: aws.Float64(55),
							Sum:         aws.Float64(-1000),
						},
					},
					entity:              entity,
					aggregationInterval: 0,
				},
			},
			expectedValues: [][]float64{
				{
					-1.5, -3, -6, -12, -24, -48, -96, -192, -384, -768,
				},
			},
			expectedCounts: [][]float64{
				{
					0, 1, 2, 3, 4, 5, 6, 7, 8, 9,
				},
			},
		},
		{
			name: "Exponential histogram with zero count bucket",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				histogramDP.SetZeroCount(5)
				histogramDP.SetSum(0)
				histogramDP.SetMin(0)
				histogramDP.SetMax(0)
				histogramDP.SetCount(uint64(5))
				histogramDP.SetScale(0)
				histogramDP.Attributes().PutStr("label1", "value1")
				histogramDP.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				return histogramDPS
			}(),
			expected: []*aggregationDatum{
				{
					MetricDatum: cloudwatch.MetricDatum{
						Dimensions: []*cloudwatch.Dimension{
							{Name: aws.String("label1"), Value: aws.String("value1")},
						},
						MetricName:        aws.String("foo"),
						Unit:              aws.String("none"),
						Timestamp:         aws.Time(ts),
						StorageResolution: aws.Int64(60),
						StatisticValues: &cloudwatch.StatisticSet{
							Maximum:     aws.Float64(0),
							Minimum:     aws.Float64(0),
							SampleCount: aws.Float64(5),
							Sum:         aws.Float64(0),
						},
					},
					entity:              entity,
					aggregationInterval: 0,
				},
			},
			expectedValues: [][]float64{
				{
					0,
				},
			},
			expectedCounts: [][]float64{
				{
					5,
				},
			},
		},
		{
			name: "Exponential histogram with positive and zero buckets",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				posBucketCounts := make([]uint64, 10)
				for i := range posBucketCounts {
					posBucketCounts[i] = uint64(i + 1) //nolint:gosec
				}
				histogramDP.Positive().BucketCounts().FromRaw(posBucketCounts)
				histogramDP.SetZeroCount(2)
				histogramDP.SetSum(10000)
				histogramDP.SetMin(0)
				histogramDP.SetMax(1000)
				histogramDP.SetCount(uint64(67))
				histogramDP.SetScale(0)
				histogramDP.Attributes().PutStr("label1", "value1")
				histogramDP.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				return histogramDPS
			}(),
			expected: []*aggregationDatum{
				{
					MetricDatum: cloudwatch.MetricDatum{
						Dimensions: []*cloudwatch.Dimension{
							{Name: aws.String("label1"), Value: aws.String("value1")},
						},
						MetricName:        aws.String("foo"),
						Unit:              aws.String("none"),
						Timestamp:         aws.Time(ts),
						StorageResolution: aws.Int64(60),
						StatisticValues: &cloudwatch.StatisticSet{
							Maximum:     aws.Float64(1000),
							Minimum:     aws.Float64(0),
							SampleCount: aws.Float64(67),
							Sum:         aws.Float64(10000),
						},
					},
					entity:              entity,
					aggregationInterval: 0,
				},
			},
			expectedValues: [][]float64{
				{
					768, 384, 192, 96, 48, 24, 12, 6, 3, 1.5, 0,
				},
			},
			expectedCounts: [][]float64{
				{
					10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 2,
				},
			},
		},
		{
			name: "Exponential histogram with negative and zero buckets",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				negBucketCounts := make([]uint64, 10)
				for i := range negBucketCounts {
					negBucketCounts[i] = uint64(i + 1) //nolint:gosec
				}
				histogramDP.Negative().BucketCounts().FromRaw(negBucketCounts)
				histogramDP.SetZeroCount(2)
				histogramDP.SetSum(-10000)
				histogramDP.SetMin(-1000)
				histogramDP.SetMax(0)
				histogramDP.SetCount(uint64(67))
				histogramDP.SetScale(0)
				histogramDP.Attributes().PutStr("label1", "value1")
				histogramDP.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				return histogramDPS
			}(),
			expected: []*aggregationDatum{
				{
					MetricDatum: cloudwatch.MetricDatum{
						Dimensions: []*cloudwatch.Dimension{
							{Name: aws.String("label1"), Value: aws.String("value1")},
						},
						MetricName:        aws.String("foo"),
						Unit:              aws.String("none"),
						Timestamp:         aws.Time(ts),
						StorageResolution: aws.Int64(60),
						StatisticValues: &cloudwatch.StatisticSet{
							Maximum:     aws.Float64(0),
							Minimum:     aws.Float64(-1000),
							SampleCount: aws.Float64(67),
							Sum:         aws.Float64(-10000),
						},
					},
					entity:              entity,
					aggregationInterval: 0,
				},
			},
			expectedValues: [][]float64{
				{
					0, -1.5, -3, -6, -12, -24, -48, -96, -192, -384, -768,
				},
			},
			expectedCounts: [][]float64{
				{
					2, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10,
				},
			},
		},
		{
			name: "Exponential histogram with positive and negative buckets",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				posBucketCounts := make([]uint64, 10)
				for i := range posBucketCounts {
					posBucketCounts[i] = uint64(i + 1) //nolint:gosec
				}
				histogramDP.Positive().BucketCounts().FromRaw(posBucketCounts)
				negBucketCounts := make([]uint64, 10)
				for i := range negBucketCounts {
					negBucketCounts[i] = uint64(i + 1) //nolint:gosec
				}
				histogramDP.Negative().BucketCounts().FromRaw(negBucketCounts)
				histogramDP.SetSum(0)
				histogramDP.SetMin(-1000)
				histogramDP.SetMax(1000)
				histogramDP.SetCount(uint64(150))
				histogramDP.SetScale(0)
				histogramDP.Attributes().PutStr("label1", "value1")
				histogramDP.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				return histogramDPS
			}(),
			expected: []*aggregationDatum{
				{
					MetricDatum: cloudwatch.MetricDatum{
						Dimensions: []*cloudwatch.Dimension{
							{Name: aws.String("label1"), Value: aws.String("value1")},
						},
						MetricName:        aws.String("foo"),
						Unit:              aws.String("none"),
						Timestamp:         aws.Time(ts),
						StorageResolution: aws.Int64(60),
						StatisticValues: &cloudwatch.StatisticSet{
							Maximum:     aws.Float64(1000),
							Minimum:     aws.Float64(-1000),
							SampleCount: aws.Float64(150),
							Sum:         aws.Float64(0),
						},
					},
					entity:              entity,
					aggregationInterval: 0,
				},
			},
			expectedValues: [][]float64{
				{
					768, 384, 192, 96, 48, 24, 12, 6, 3, 1.5, -1.5, -3, -6, -12, -24, -48, -96, -192, -384, -768,
				},
			},
			expectedCounts: [][]float64{
				{
					10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10,
				},
			},
		},
		{
			name: "Exponential histogram with positive, negative and zero buckets",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				posBucketCounts := make([]uint64, 10)
				for i := range posBucketCounts {
					posBucketCounts[i] = uint64(i + 1) //nolint:gosec
				}
				histogramDP.Positive().BucketCounts().FromRaw(posBucketCounts)
				histogramDP.SetZeroCount(2)
				negBucketCounts := make([]uint64, 10)
				for i := range negBucketCounts {
					negBucketCounts[i] = uint64(i + 1) //nolint:gosec
				}
				histogramDP.Negative().BucketCounts().FromRaw(negBucketCounts)
				histogramDP.SetSum(0)
				histogramDP.SetMin(-1000)
				histogramDP.SetMax(1000)
				histogramDP.SetCount(uint64(152))
				histogramDP.SetScale(0)
				histogramDP.Attributes().PutStr("label1", "value1")
				histogramDP.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				return histogramDPS
			}(),
			expected: []*aggregationDatum{
				{
					MetricDatum: cloudwatch.MetricDatum{
						Dimensions: []*cloudwatch.Dimension{
							{Name: aws.String("label1"), Value: aws.String("value1")},
						},
						MetricName:        aws.String("foo"),
						Unit:              aws.String("none"),
						Timestamp:         aws.Time(ts),
						StorageResolution: aws.Int64(60),
						StatisticValues: &cloudwatch.StatisticSet{
							Maximum:     aws.Float64(1000),
							Minimum:     aws.Float64(-1000),
							SampleCount: aws.Float64(152),
							Sum:         aws.Float64(0),
						},
					},
					entity:              entity,
					aggregationInterval: 0,
				},
			},
			expectedValues: [][]float64{
				{
					768, 384, 192, 96, 48, 24, 12, 6, 3, 1.5, 0, -1.5, -3, -6, -12, -24, -48, -96, -192, -384, -768,
				},
			},
			expectedCounts: [][]float64{
				{
					10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 2, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10,
				},
			},
		},
		{
			name: "Exponential histogram with no buckets",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				histogramDP.SetSum(0)
				histogramDP.SetMin(0)
				histogramDP.SetMax(0)
				histogramDP.SetCount(uint64(0))
				histogramDP.SetScale(0)
				histogramDP.Attributes().PutStr("label1", "value1")
				histogramDP.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				return histogramDPS
			}(),
			expected: []*aggregationDatum{
				{
					MetricDatum: cloudwatch.MetricDatum{
						Dimensions: []*cloudwatch.Dimension{
							{Name: aws.String("label1"), Value: aws.String("value1")},
						},
						MetricName:        aws.String("foo"),
						Unit:              aws.String("none"),
						Timestamp:         aws.Time(ts),
						StorageResolution: aws.Int64(60),
						StatisticValues: &cloudwatch.StatisticSet{
							Maximum:     aws.Float64(0),
							Minimum:     aws.Float64(0),
							SampleCount: aws.Float64(0),
							Sum:         aws.Float64(0),
						},
					},
					entity:              entity,
					aggregationInterval: 0,
				},
			},
			expectedValues: [][]float64{{}},
			expectedCounts: [][]float64{{}},
		},
		{
			name: "Exponential histogram with with StorageResolution",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				posBucketCounts := make([]uint64, 10)
				for i := range posBucketCounts {
					posBucketCounts[i] = uint64(i + 1) //nolint:gosec
				}
				histogramDP.Positive().BucketCounts().FromRaw(posBucketCounts)
				histogramDP.SetZeroCount(2)
				negBucketCounts := make([]uint64, 10)
				for i := range negBucketCounts {
					negBucketCounts[i] = uint64(i + 1) //nolint:gosec
				}
				histogramDP.Negative().BucketCounts().FromRaw(negBucketCounts)
				histogramDP.SetSum(0)
				histogramDP.SetMin(-1000)
				histogramDP.SetMax(1000)
				histogramDP.SetCount(uint64(152))
				histogramDP.SetScale(0)
				histogramDP.Attributes().PutStr("label1", "value1")
				histogramDP.Attributes().PutStr("aws:StorageResolution", "true")
				histogramDP.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				return histogramDPS
			}(),
			expected: []*aggregationDatum{
				{
					MetricDatum: cloudwatch.MetricDatum{
						Dimensions: []*cloudwatch.Dimension{
							{Name: aws.String("label1"), Value: aws.String("value1")},
						},
						MetricName:        aws.String("foo"),
						Unit:              aws.String("none"),
						Timestamp:         aws.Time(ts),
						StorageResolution: aws.Int64(1),
						StatisticValues: &cloudwatch.StatisticSet{
							Maximum:     aws.Float64(1000),
							Minimum:     aws.Float64(-1000),
							SampleCount: aws.Float64(152),
							Sum:         aws.Float64(0),
						},
					},
					entity:              entity,
					aggregationInterval: 0,
				},
			},
			expectedValues: [][]float64{
				{
					768, 384, 192, 96, 48, 24, 12, 6, 3, 1.5, 0, -1.5, -3, -6, -12, -24, -48, -96, -192, -384, -768,
				},
			},
			expectedCounts: [][]float64{
				{
					10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 2, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10,
				},
			},
		},
		{
			name: "Exponential histogram with AggregationInterval",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				posBucketCounts := make([]uint64, 10)
				for i := range posBucketCounts {
					posBucketCounts[i] = uint64(i + 1) //nolint:gosec
				}
				histogramDP.Positive().BucketCounts().FromRaw(posBucketCounts)
				histogramDP.SetZeroCount(2)
				negBucketCounts := make([]uint64, 10)
				for i := range negBucketCounts {
					negBucketCounts[i] = uint64(i + 1) //nolint:gosec
				}
				histogramDP.Negative().BucketCounts().FromRaw(negBucketCounts)
				histogramDP.SetSum(0)
				histogramDP.SetMin(-1000)
				histogramDP.SetMax(1000)
				histogramDP.SetCount(uint64(152))
				histogramDP.SetScale(0)
				histogramDP.Attributes().PutStr("label1", "value1")
				histogramDP.Attributes().PutStr("aws:AggregationInterval", "5m")
				histogramDP.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				return histogramDPS
			}(),
			expected: []*aggregationDatum{
				{
					MetricDatum: cloudwatch.MetricDatum{
						Dimensions: []*cloudwatch.Dimension{
							{Name: aws.String("label1"), Value: aws.String("value1")},
						},
						MetricName:        aws.String("foo"),
						Unit:              aws.String("none"),
						Timestamp:         aws.Time(ts),
						StorageResolution: aws.Int64(1),
						StatisticValues: &cloudwatch.StatisticSet{
							Maximum:     aws.Float64(1000),
							Minimum:     aws.Float64(-1000),
							SampleCount: aws.Float64(152),
							Sum:         aws.Float64(0),
						},
					},
					entity:              entity,
					aggregationInterval: 5 * time.Minute,
				},
			},
			expectedValues: [][]float64{
				{
					768, 384, 192, 96, 48, 24, 12, 6, 3, 1.5, 0, -1.5, -3, -6, -12, -24, -48, -96, -192, -384, -768,
				},
			},
			expectedCounts: [][]float64{
				{
					10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 2, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10,
				},
			},
		},
		{
			name: "Exponential histogram with positive scale",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				posBucketCounts := make([]uint64, 10)
				for i := range posBucketCounts {
					posBucketCounts[i] = uint64(i + 1) //nolint:gosec
				}
				histogramDP.Positive().BucketCounts().FromRaw(posBucketCounts)
				histogramDP.SetZeroCount(2)
				negBucketCounts := make([]uint64, 10)
				for i := range negBucketCounts {
					negBucketCounts[i] = uint64(i + 1) //nolint:gosec
				}
				histogramDP.Negative().BucketCounts().FromRaw(negBucketCounts)
				histogramDP.SetSum(0)
				histogramDP.SetMin(-1000)
				histogramDP.SetMax(1000)
				histogramDP.SetCount(uint64(152))
				histogramDP.SetScale(1)
				histogramDP.Attributes().PutStr("label1", "value1")
				histogramDP.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				return histogramDPS
			}(),
			expected: []*aggregationDatum{
				{
					MetricDatum: cloudwatch.MetricDatum{
						Dimensions: []*cloudwatch.Dimension{
							{Name: aws.String("label1"), Value: aws.String("value1")},
						},
						MetricName:        aws.String("foo"),
						Unit:              aws.String("none"),
						Timestamp:         aws.Time(ts),
						StorageResolution: aws.Int64(60),
						StatisticValues: &cloudwatch.StatisticSet{
							Maximum:     aws.Float64(1000),
							Minimum:     aws.Float64(-1000),
							SampleCount: aws.Float64(152),
							Sum:         aws.Float64(0),
						},
					},
					entity:              entity,
					aggregationInterval: 0,
				},
			},
			expectedValues: [][]float64{
				{
					27.31370849898476, 19.31370849898476, 13.656854249492378, 9.656854249492378, 6.828427124746189,
					4.82842712474619, 3.414213562373095, 2.414213562373095, 1.7071067811865475, 1.2071067811865475, 0,
					-1.2071067811865475, -1.7071067811865475, -2.414213562373095, -3.414213562373095, -4.82842712474619,
					-6.828427124746189, -9.656854249492378, -13.656854249492378, -19.31370849898476, -27.31370849898476,
				},
			},
			expectedCounts: [][]float64{
				{
					10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 2, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10,
				},
			},
		},
		{
			name: "Exponential histogram with negative scale",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				posBucketCounts := make([]uint64, 10)
				for i := range posBucketCounts {
					posBucketCounts[i] = uint64(i + 1) //nolint:gosec
				}
				histogramDP.Positive().BucketCounts().FromRaw(posBucketCounts)
				histogramDP.SetZeroCount(2)
				negBucketCounts := make([]uint64, 10)
				for i := range negBucketCounts {
					negBucketCounts[i] = uint64(i + 1) //nolint:gosec
				}
				histogramDP.Negative().BucketCounts().FromRaw(negBucketCounts)
				histogramDP.SetSum(0)
				histogramDP.SetMin(-1000)
				histogramDP.SetMax(1000)
				histogramDP.SetCount(uint64(152))
				histogramDP.SetScale(-1)
				histogramDP.Attributes().PutStr("label1", "value1")
				histogramDP.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				return histogramDPS
			}(),
			expected: []*aggregationDatum{
				{
					MetricDatum: cloudwatch.MetricDatum{
						Dimensions: []*cloudwatch.Dimension{
							{Name: aws.String("label1"), Value: aws.String("value1")},
						},
						MetricName:        aws.String("foo"),
						Unit:              aws.String("none"),
						Timestamp:         aws.Time(ts),
						StorageResolution: aws.Int64(60),
						StatisticValues: &cloudwatch.StatisticSet{
							Maximum:     aws.Float64(1000),
							Minimum:     aws.Float64(-1000),
							SampleCount: aws.Float64(152),
							Sum:         aws.Float64(0),
						},
					},
					entity:              entity,
					aggregationInterval: 0,
				},
			},
			expectedValues: [][]float64{
				{
					655360, 163840, 40960, 10240, 2560, 640, 160, 40, 10, 2.5, 0, -2.5, -10, -40, -160, -640, -2560, -10240, -40960, -163840, -655360,
				},
			},
			expectedCounts: [][]float64{
				{
					10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 2, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10,
				},
			},
		},
		{
			name: "Exponential histogram with offsets",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				posBucketCounts := make([]uint64, 10)
				for i := range posBucketCounts {
					posBucketCounts[i] = uint64(i + 1) //nolint:gosec
				}
				histogramDP.Positive().BucketCounts().FromRaw(posBucketCounts)
				histogramDP.Positive().SetOffset(1)
				histogramDP.SetZeroCount(2)
				negBucketCounts := make([]uint64, 10)
				for i := range negBucketCounts {
					negBucketCounts[i] = uint64(i + 1) //nolint:gosec
				}
				histogramDP.Negative().BucketCounts().FromRaw(negBucketCounts)
				histogramDP.Negative().SetOffset(2)
				histogramDP.SetSum(0)
				histogramDP.SetMin(-1000)
				histogramDP.SetMax(1000)
				histogramDP.SetCount(uint64(152))
				histogramDP.SetScale(-1)
				histogramDP.Attributes().PutStr("label1", "value1")
				histogramDP.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				return histogramDPS
			}(),
			expected: []*aggregationDatum{
				{
					MetricDatum: cloudwatch.MetricDatum{
						Dimensions: []*cloudwatch.Dimension{
							{Name: aws.String("label1"), Value: aws.String("value1")},
						},
						MetricName:        aws.String("foo"),
						Unit:              aws.String("none"),
						Timestamp:         aws.Time(ts),
						StorageResolution: aws.Int64(60),
						StatisticValues: &cloudwatch.StatisticSet{
							Maximum:     aws.Float64(1000),
							Minimum:     aws.Float64(-1000),
							SampleCount: aws.Float64(152),
							Sum:         aws.Float64(0),
						},
					},
					entity:              entity,
					aggregationInterval: 0,
				},
			},
			expectedValues: [][]float64{
				{
					2621440, 655360, 163840, 40960, 10240, 2560, 640, 160, 40, 10, 0, -40, -160, -640, -2560, -10240, -40960, -163840, -655360, -2621440, -10485760,
				},
			},
			expectedCounts: [][]float64{
				{
					10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 2, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10,
				},
			},
		},
	}

	// Since some of the values could be zero, we can't use InEpsilon (requires non-zero values to form relative error)
	// We calculate the relative error directly and then use InDelta instead
	const epsilon = 0.0001

	for _, tc := range testCases {
		t.Run(tc.name, func(_ *testing.T) {
			dps := convertOtelExponentialHistogramDataPoints(tc.histogramDPS, "foo", "none", 1, entity)

			assert.Equal(t, 1, len(dps))
			for i, expectedDP := range tc.expected {
				assert.InDelta(t, *expectedDP.StatisticValues.Maximum, dps[i].distribution.Maximum(), math.Abs(*expectedDP.StatisticValues.Maximum)*epsilon, "datapoint Maximum mismatch at index %d", i)
				assert.InDelta(t, *expectedDP.StatisticValues.Minimum, dps[i].distribution.Minimum(), math.Abs(*expectedDP.StatisticValues.Minimum)*epsilon, "datapoint Minimum mismatch at index %d", i)
				assert.InDelta(t, *expectedDP.StatisticValues.Sum, dps[i].distribution.Sum(), math.Abs(*expectedDP.StatisticValues.Sum)*epsilon, "datapoint Sum mismatch at index %d", i)
				assert.Equal(t, dps[i].distribution.SampleCount(), *expectedDP.StatisticValues.SampleCount, "datapoint Samplecount mismatch at index %d", i)

				values, counts := dps[i].distribution.ValuesAndCounts()
				for j, expectedValue := range tc.expectedValues[i] {
					assert.InDelta(t, expectedValue, values[j], math.Abs(expectedValue)*epsilon, "datapoint values mismatch at index %d, value %d", i, j)
				}
				for j, expectedCount := range tc.expectedCounts[i] {
					assert.InDelta(t, expectedCount, counts[j], math.Abs(expectedCount)*epsilon, "datapoint counts mismatch at index %d, count %d", i, j)
				}
			}
		})
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

func TestConvertOtelMetrics_Entity(t *testing.T) {
	metrics := createTestMetrics(1, 1, 1, "s")
	datums := ConvertOtelMetrics(metrics)
	expectedEntity := cloudwatch.Entity{
		KeyAttributes: map[string]*string{
			"Type":         aws.String("Service"),
			"Environment":  aws.String("MyEnvironment"),
			"Name":         aws.String("MyServiceName"),
			"AwsAccountId": aws.String("0123456789012"),
		},
		Attributes: map[string]*string{
			"EC2.InstanceId":       aws.String("i-123456789"),
			"PlatformType":         aws.String("AWS::EC2"),
			"EC2.AutoScalingGroup": aws.String("asg-123"),
		},
	}
	assert.Equal(t, 1, len(datums))
	assert.Equal(t, expectedEntity, datums[0].entity)

}

func TestInvalidMetric(t *testing.T) {
	m := pmetric.NewMetric()
	m.SetName("name")
	m.SetUnit("unit")
	assert.Empty(t, ConvertOtelMetric(m, cloudwatch.Entity{}))
}
