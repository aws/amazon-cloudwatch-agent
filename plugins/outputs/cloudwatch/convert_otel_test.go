// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatch

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
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
			dp.SetCount(4)
			dp.SetSum(0)
			dp.SetMin(-4)
			dp.SetMax(4)
			dp.SetZeroCount(0)
			dp.SetScale(1)
			dp.Positive().SetOffset(1)
			dp.Positive().BucketCounts().FromRaw([]uint64{
				1, 0, 1,
			})
			dp.Negative().SetOffset(1)
			dp.Negative().BucketCounts().FromRaw([]uint64{
				1, 0, 1,
			})
			dp.Attributes().PutStr("label1", "value1")

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

func TestConvertOtelMetrics_ExponentialHistogram(t *testing.T) {
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
		name         string
		histogramDPS pmetric.ExponentialHistogramDataPointSlice
		expected     []*aggregationDatum
	}{

		// {
		// 	name: "Exponential histogram with more than 100 buckets, including positive and negative buckets",
		// 	histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
		// 		histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
		// 		histogramDP := histogramDPS.AppendEmpty()
		// 		posBucketCounts := make([]uint64, 60)
		// 		for i := range posBucketCounts {
		// 			posBucketCounts[i] = uint64(i + 1)
		// 		}
		// 		histogramDP.Positive().BucketCounts().FromRaw(posBucketCounts)
		// 		negBucketCounts := make([]uint64, 60)
		// 		for i := range negBucketCounts {
		// 			negBucketCounts[i] = uint64(i + 1)
		// 		}
		// 		histogramDP.Negative().BucketCounts().FromRaw(negBucketCounts)
		// 		histogramDP.SetSum(1000)
		// 		histogramDP.SetMin(-9e+17)
		// 		histogramDP.SetMax(9e+17)
		// 		histogramDP.SetCount(uint64(3660))
		// 		histogramDP.Attributes().PutStr("label1", "value1")
		// 		return histogramDPS
		// 	}(),
		// 	expectedDatapoints: []dataPoint{
		// 		{
		// 			name: "foo",
		// 			value: &cWMetricHistogram{
		// 				Values: []float64{
		// 					8.646911284551352e+17, 4.323455642275676e+17, 2.161727821137838e+17, 1.080863910568919e+17, 5.404319552844595e+16, 2.7021597764222976e+16,
		// 					1.3510798882111488e+16, 6.755399441055744e+15, 3.377699720527872e+15, 1.688849860263936e+15, 8.44424930131968e+14, 4.22212465065984e+14,
		// 					2.11106232532992e+14, 1.05553116266496e+14, 5.2776558133248e+13, 2.6388279066624e+13, 1.3194139533312e+13, 6.597069766656e+12, 3.298534883328e+12,
		// 					1.649267441664e+12, 8.24633720832e+11, 4.12316860416e+11, 2.06158430208e+11, 1.03079215104e+11, 5.1539607552e+10, 2.5769803776e+10,
		// 					1.2884901888e+10, 6.442450944e+09, 3.221225472e+09, 1.610612736e+09, 8.05306368e+08, 4.02653184e+08, 2.01326592e+08, 1.00663296e+08,
		// 					5.0331648e+07, 2.5165824e+07, 1.2582912e+07, 6.291456e+06, 3.145728e+06, 1.572864e+06, 786432, 393216, 196608, 98304, 49152, 24576,
		// 					12288, 6144, 3072, 1536, 768, 384, 192, 96, 48, 24, 12, 6, 3, 1.5, -1.5, -3, -6, -12, -24, -48, -96, -192, -384, -768, -1536, -3072,
		// 					-6144, -12288, -24576, -49152, -98304, -196608, -393216, -786432, -1.572864e+06, -3.145728e+06, -6.291456e+06, -1.2582912e+07, -2.5165824e+07,
		// 					-5.0331648e+07, -1.00663296e+08, -2.01326592e+08, -4.02653184e+08, -8.05306368e+08, -1.610612736e+09, -3.221225472e+09, -6.442450944e+09,
		// 					-1.2884901888e+10, -2.5769803776e+10, -5.1539607552e+10, -1.03079215104e+11, -2.06158430208e+11, -4.12316860416e+11, -8.24633720832e+11,
		// 				},
		// 				Counts: []float64{
		// 					60, 59, 58, 57, 56, 55, 54, 53, 52, 51, 50, 49, 48, 47, 46, 45, 44, 43, 42, 41, 40, 39, 38, 37, 36, 35, 34, 33, 32, 31, 30, 29, 28, 27, 26, 25, 24, 23, 22, 21, 20, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10, 9, 8, 7,
		// 					6, 5, 4, 3, 2, 1, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33,
		// 					34, 35, 36, 37, 38, 39, 40,
		// 				},
		// 				Sum: 1000, Count: 2650, Min: -1.099511627776e+12, Max: 9e+17,
		// 			},
		// 			labels: map[string]string{"label1": "value1"},
		// 		},
		// 		{
		// 			name: "foo",
		// 			value: &cWMetricHistogram{
		// 				Values: []float64{
		// 					-1.649267441664e+12, -3.298534883328e+12, -6.597069766656e+12, -1.3194139533312e+13, -2.6388279066624e+13, -5.2776558133248e+13,
		// 					-1.05553116266496e+14, -2.11106232532992e+14, -4.22212465065984e+14, -8.44424930131968e+14, -1.688849860263936e+15, -3.377699720527872e+15,
		// 					-6.755399441055744e+15, -1.3510798882111488e+16, -2.7021597764222976e+16, -5.404319552844595e+16, -1.080863910568919e+17, -2.161727821137838e+17,
		// 					-4.323455642275676e+17, -8.646911284551352e+17,
		// 				},
		// 				Counts: []float64{41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60},
		// 				Sum:    0, Count: 1010, Min: -9e+17, Max: -1.099511627776e+12,
		// 			},
		// 			labels: map[string]string{"label1": "value1"},
		// 		},
		// 	},
		// },
		// {
		// 	name: "Exponential histogram with exact 200 buckets, including positive, negative buckets",
		// 	histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
		// 		histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
		// 		histogramDP := histogramDPS.AppendEmpty()
		// 		posBucketCounts := make([]uint64, 100)
		// 		for i := range posBucketCounts {
		// 			posBucketCounts[i] = uint64(i + 1)
		// 		}
		// 		histogramDP.Positive().BucketCounts().FromRaw(posBucketCounts)
		// 		negBucketCounts := make([]uint64, 100)
		// 		for i := range negBucketCounts {
		// 			negBucketCounts[i] = uint64(i + 1)
		// 		}
		// 		histogramDP.Negative().BucketCounts().FromRaw(negBucketCounts)
		// 		histogramDP.SetSum(100000)
		// 		histogramDP.SetMin(-9e+36)
		// 		histogramDP.SetMax(9e+36)
		// 		histogramDP.SetCount(uint64(3662))
		// 		histogramDP.Attributes().PutStr("label1", "value1")
		// 		return histogramDPS
		// 	}(),
		// 	expectedDatapoints: []dataPoint{
		// 		{
		// 			name: "foo",
		// 			value: &cWMetricHistogram{
		// 				Values: []float64{
		// 					9.50737950171172e+29, 4.75368975085586e+29, 2.37684487542793e+29, 1.188422437713965e+29, 5.942112188569825e+28,
		// 					2.9710560942849127e+28, 1.4855280471424563e+28, 7.427640235712282e+27, 3.713820117856141e+27, 1.8569100589280704e+27,
		// 					9.284550294640352e+26, 4.642275147320176e+26, 2.321137573660088e+26, 1.160568786830044e+26, 5.80284393415022e+25,
		// 					2.90142196707511e+25, 1.450710983537555e+25, 7.253554917687775e+24, 3.6267774588438875e+24, 1.8133887294219438e+24,
		// 					9.066943647109719e+23, 4.5334718235548594e+23, 2.2667359117774297e+23, 1.1333679558887149e+23, 5.666839779443574e+22,
		// 					2.833419889721787e+22, 1.4167099448608936e+22, 7.083549724304468e+21, 3.541774862152234e+21, 1.770887431076117e+21,
		// 					8.854437155380585e+20, 4.4272185776902924e+20, 2.2136092888451462e+20, 1.1068046444225731e+20, 5.5340232221128655e+19,
		// 					2.7670116110564327e+19, 1.3835058055282164e+19, 6.917529027641082e+18, 3.458764513820541e+18, 1.7293822569102705e+18,
		// 					8.646911284551352e+17, 4.323455642275676e+17, 2.161727821137838e+17, 1.080863910568919e+17, 5.404319552844595e+16,
		// 					2.7021597764222976e+16, 1.3510798882111488e+16, 6.755399441055744e+15, 3.377699720527872e+15, 1.688849860263936e+15,
		// 					8.44424930131968e+14, 4.22212465065984e+14, 2.11106232532992e+14, 1.05553116266496e+14, 5.2776558133248e+13,
		// 					2.6388279066624e+13, 1.3194139533312e+13, 6.597069766656e+12, 3.298534883328e+12, 1.649267441664e+12, 8.24633720832e+11,
		// 					4.12316860416e+11, 2.06158430208e+11, 1.03079215104e+11, 5.1539607552e+10, 2.5769803776e+10, 1.2884901888e+10, 6.442450944e+09,
		// 					3.221225472e+09, 1.610612736e+09, 8.05306368e+08, 4.02653184e+08, 2.01326592e+08, 1.00663296e+08, 5.0331648e+07,
		// 					2.5165824e+07, 1.2582912e+07, 6.291456e+06, 3.145728e+06, 1.572864e+06, 786432, 393216, 196608, 98304, 49152, 24576, 12288,
		// 					6144, 3072, 1536, 768, 384, 192, 96, 48, 24, 12, 6, 3, 1.5,
		// 				},
		// 				Counts: []float64{
		// 					100, 99, 98, 97, 96, 95, 94,
		// 					93, 92, 91, 90, 89, 88, 87, 86, 85, 84, 83, 82, 81, 80, 79, 78, 77, 76, 75, 74, 73, 72, 71, 70, 69, 68, 67, 66, 65, 64, 63, 62, 61,
		// 					60, 59, 58, 57, 56, 55, 54, 53, 52, 51, 50, 49, 48, 47, 46, 45, 44, 43, 42, 41, 40, 39, 38, 37, 36, 35, 34, 33, 32, 31, 30, 29, 28,
		// 					27, 26, 25, 24, 23, 22, 21, 20, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1,
		// 				},
		// 				Sum: 100000, Count: 5050, Min: 1, Max: 9e+36,
		// 			},
		// 			labels: map[string]string{"label1": "value1"},
		// 		},
		// 		{
		// 			name: "foo",
		// 			value: &cWMetricHistogram{
		// 				Values: []float64{
		// 					-1.5, -3, -6, -12, -24, -48, -96, -192, -384, -768, -1536, -3072, -6144, -12288, -24576, -49152, -98304, -196608, -393216,
		// 					-786432, -1.572864e+06, -3.145728e+06, -6.291456e+06, -1.2582912e+07, -2.5165824e+07, -5.0331648e+07, -1.00663296e+08,
		// 					-2.01326592e+08, -4.02653184e+08, -8.05306368e+08, -1.610612736e+09, -3.221225472e+09, -6.442450944e+09, -1.2884901888e+10,
		// 					-2.5769803776e+10, -5.1539607552e+10, -1.03079215104e+11, -2.06158430208e+11, -4.12316860416e+11, -8.24633720832e+11,
		// 					-1.649267441664e+12, -3.298534883328e+12, -6.597069766656e+12, -1.3194139533312e+13, -2.6388279066624e+13, -5.2776558133248e+13,
		// 					-1.05553116266496e+14, -2.11106232532992e+14, -4.22212465065984e+14, -8.44424930131968e+14, -1.688849860263936e+15,
		// 					-3.377699720527872e+15, -6.755399441055744e+15, -1.3510798882111488e+16, -2.7021597764222976e+16,
		// 					-5.404319552844595e+16, -1.080863910568919e+17, -2.161727821137838e+17, -4.323455642275676e+17, -8.646911284551352e+17,
		// 					-1.7293822569102705e+18, -3.458764513820541e+18, -6.917529027641082e+18, -1.3835058055282164e+19, -2.7670116110564327e+19,
		// 					-5.5340232221128655e+19, -1.1068046444225731e+20, -2.2136092888451462e+20, -4.4272185776902924e+20, -8.854437155380585e+20,
		// 					-1.770887431076117e+21, -3.541774862152234e+21, -7.083549724304468e+21, -1.4167099448608936e+22, -2.833419889721787e+22,
		// 					-5.666839779443574e+22, -1.1333679558887149e+23, -2.2667359117774297e+23, -4.5334718235548594e+23, -9.066943647109719e+23,
		// 					-1.8133887294219438e+24, -3.6267774588438875e+24, -7.253554917687775e+24, -1.450710983537555e+25, -2.90142196707511e+25,
		// 					-5.80284393415022e+25, -1.160568786830044e+26, -2.321137573660088e+26, -4.642275147320176e+26, -9.284550294640352e+26,
		// 					-1.8569100589280704e+27, -3.713820117856141e+27, -7.427640235712282e+27, -1.4855280471424563e+28, -2.9710560942849127e+28,
		// 					-5.942112188569825e+28, -1.188422437713965e+29, -2.37684487542793e+29, -4.75368975085586e+29, -9.50737950171172e+29,
		// 				},
		// 				Counts: []float64{
		// 					1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35,
		// 					36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63, 64, 65, 66, 67, 68,
		// 					69, 70, 71, 72, 73, 74, 75, 76, 77, 78, 79, 80, 81, 82, 83, 84, 85, 86, 87, 88, 89, 90, 91, 92, 93, 94, 95, 96, 97, 98, 99, 100,
		// 				},
		// 				Sum: 0, Count: 5050, Min: -9e+36, Max: -1,
		// 			},
		// 			labels: map[string]string{"label1": "value1"},
		// 		},
		// 	},
		// },
		// {
		// 	name: "Exponential histogram with more than 200 buckets, including positive, negative and zero buckets",
		// 	histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
		// 		histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
		// 		histogramDP := histogramDPS.AppendEmpty()
		// 		posBucketCounts := make([]uint64, 120)
		// 		for i := range posBucketCounts {
		// 			posBucketCounts[i] = uint64(i + 1)
		// 		}
		// 		histogramDP.Positive().BucketCounts().FromRaw(posBucketCounts)
		// 		histogramDP.SetZeroCount(2)
		// 		negBucketCounts := make([]uint64, 120)
		// 		for i := range negBucketCounts {
		// 			negBucketCounts[i] = uint64(i + 1)
		// 		}
		// 		histogramDP.Negative().BucketCounts().FromRaw(negBucketCounts)
		// 		histogramDP.SetSum(100000)
		// 		histogramDP.SetMin(-9e+36)
		// 		histogramDP.SetMax(9e+36)
		// 		histogramDP.SetCount(uint64(3662))
		// 		histogramDP.Attributes().PutStr("label1", "value1")
		// 		return histogramDPS
		// 	}(),
		// 	expectedDatapoints: []dataPoint{
		// 		{
		// 			name: "foo",
		// 			value: &cWMetricHistogram{
		// 				Values: []float64{
		// 					9.969209968386869e+35, 4.9846049841934345e+35, 2.4923024920967173e+35, 1.2461512460483586e+35, 6.230756230241793e+34,
		// 					3.1153781151208966e+34, 1.5576890575604483e+34, 7.788445287802241e+33, 3.894222643901121e+33, 1.9471113219505604e+33,
		// 					9.735556609752802e+32, 4.867778304876401e+32, 2.4338891524382005e+32, 1.2169445762191002e+32, 6.084722881095501e+31,
		// 					3.0423614405477506e+31, 1.5211807202738753e+31, 7.605903601369376e+30, 3.802951800684688e+30, 1.901475900342344e+30,
		// 					9.50737950171172e+29, 4.75368975085586e+29, 2.37684487542793e+29, 1.188422437713965e+29, 5.942112188569825e+28,
		// 					2.9710560942849127e+28, 1.4855280471424563e+28, 7.427640235712282e+27, 3.713820117856141e+27, 1.8569100589280704e+27,
		// 					9.284550294640352e+26, 4.642275147320176e+26, 2.321137573660088e+26, 1.160568786830044e+26, 5.80284393415022e+25,
		// 					2.90142196707511e+25, 1.450710983537555e+25, 7.253554917687775e+24, 3.6267774588438875e+24, 1.8133887294219438e+24,
		// 					9.066943647109719e+23, 4.5334718235548594e+23, 2.2667359117774297e+23, 1.1333679558887149e+23, 5.666839779443574e+22,
		// 					2.833419889721787e+22, 1.4167099448608936e+22, 7.083549724304468e+21, 3.541774862152234e+21, 1.770887431076117e+21,
		// 					8.854437155380585e+20, 4.4272185776902924e+20, 2.2136092888451462e+20, 1.1068046444225731e+20, 5.5340232221128655e+19,
		// 					2.7670116110564327e+19, 1.3835058055282164e+19, 6.917529027641082e+18, 3.458764513820541e+18, 1.7293822569102705e+18,
		// 					8.646911284551352e+17, 4.323455642275676e+17, 2.161727821137838e+17, 1.080863910568919e+17, 5.404319552844595e+16,
		// 					2.7021597764222976e+16, 1.3510798882111488e+16, 6.755399441055744e+15, 3.377699720527872e+15, 1.688849860263936e+15,
		// 					8.44424930131968e+14, 4.22212465065984e+14, 2.11106232532992e+14, 1.05553116266496e+14, 5.2776558133248e+13,
		// 					2.6388279066624e+13, 1.3194139533312e+13, 6.597069766656e+12, 3.298534883328e+12, 1.649267441664e+12, 8.24633720832e+11,
		// 					4.12316860416e+11, 2.06158430208e+11, 1.03079215104e+11, 5.1539607552e+10, 2.5769803776e+10, 1.2884901888e+10,
		// 					6.442450944e+09, 3.221225472e+09, 1.610612736e+09, 8.05306368e+08, 4.02653184e+08, 2.01326592e+08, 1.00663296e+08, 5.0331648e+07,
		// 					2.5165824e+07, 1.2582912e+07, 6.291456e+06, 3.145728e+06, 1.572864e+06,
		// 				},
		// 				Counts: []float64{
		// 					120, 119, 118, 117, 116, 115, 114, 113, 112, 111, 110, 109, 108, 107, 106, 105, 104, 103, 102, 101, 100, 99, 98, 97, 96, 95, 94,
		// 					93, 92, 91, 90, 89, 88, 87, 86, 85, 84, 83, 82, 81, 80, 79, 78, 77, 76, 75, 74, 73, 72, 71, 70, 69, 68, 67, 66, 65, 64, 63, 62, 61,
		// 					60, 59, 58, 57, 56, 55, 54, 53, 52, 51, 50, 49, 48, 47, 46, 45, 44, 43, 42, 41, 40, 39, 38, 37, 36, 35, 34, 33, 32, 31, 30, 29, 28,
		// 					27, 26, 25, 24, 23, 22, 21,
		// 				},
		// 				Sum: 100000, Count: 7050, Min: 1048576, Max: 9e+36,
		// 			},
		// 			labels: map[string]string{"label1": "value1"},
		// 		},
		// 		{
		// 			name: "foo",
		// 			value: &cWMetricHistogram{
		// 				Values: []float64{
		// 					786432, 393216, 196608, 98304, 49152, 24576, 12288, 6144, 3072, 1536, 768, 384, 192, 96, 48, 24,
		// 					12, 6, 3, 1.5, 0, -1.5, -3, -6, -12, -24, -48, -96, -192, -384, -768, -1536,
		// 					-3072, -6144, -12288, -24576, -49152, -98304, -196608, -393216, -786432, -1.572864e+06, -3.145728e+06, -6.291456e+06,
		// 					-1.2582912e+07, -2.5165824e+07, -5.0331648e+07, -1.00663296e+08, -2.01326592e+08, -4.02653184e+08, -8.05306368e+08,
		// 					-1.610612736e+09, -3.221225472e+09, -6.442450944e+09, -1.2884901888e+10, -2.5769803776e+10, -5.1539607552e+10,
		// 					-1.03079215104e+11, -2.06158430208e+11, -4.12316860416e+11, -8.24633720832e+11, -1.649267441664e+12,
		// 					-3.298534883328e+12, -6.597069766656e+12, -1.3194139533312e+13, -2.6388279066624e+13, -5.2776558133248e+13,
		// 					-1.05553116266496e+14, -2.11106232532992e+14, -4.22212465065984e+14, -8.44424930131968e+14,
		// 					-1.688849860263936e+15, -3.377699720527872e+15, -6.755399441055744e+15, -1.3510798882111488e+16,
		// 					-2.7021597764222976e+16, -5.404319552844595e+16, -1.080863910568919e+17, -2.161727821137838e+17,
		// 					-4.323455642275676e+17, -8.646911284551352e+17, -1.7293822569102705e+18, -3.458764513820541e+18,
		// 					-6.917529027641082e+18, -1.3835058055282164e+19, -2.7670116110564327e+19, -5.5340232221128655e+19,
		// 					-1.1068046444225731e+20, -2.2136092888451462e+20, -4.4272185776902924e+20, -8.854437155380585e+20,
		// 					-1.770887431076117e+21, -3.541774862152234e+21, -7.083549724304468e+21, -1.4167099448608936e+22,
		// 					-2.833419889721787e+22, -5.666839779443574e+22, -1.1333679558887149e+23, -2.2667359117774297e+23,
		// 					-4.5334718235548594e+23,
		// 				},
		// 				Counts: []float64{
		// 					20, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 2, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10,
		// 					11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36,
		// 					37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62,
		// 					63, 64, 65, 66, 67, 68, 69, 70, 71, 72, 73, 74, 75, 76, 77, 78, 79,
		// 				},
		// 				Sum: 0, Count: 3372, Min: -6.044629098073146e+23, Max: 1048576,
		// 			},
		// 			labels: map[string]string{"label1": "value1"},
		// 		},
		// 		{
		// 			name: "foo",
		// 			value: &cWMetricHistogram{
		// 				Values: []float64{
		// 					-9.066943647109719e+23, -1.8133887294219438e+24, -3.6267774588438875e+24, -7.253554917687775e+24, -1.450710983537555e+25,
		// 					-2.90142196707511e+25, -5.80284393415022e+25, -1.160568786830044e+26, -2.321137573660088e+26, -4.642275147320176e+26,
		// 					-9.284550294640352e+26, -1.8569100589280704e+27, -3.713820117856141e+27, -7.427640235712282e+27, -1.4855280471424563e+28,
		// 					-2.9710560942849127e+28, -5.942112188569825e+28, -1.188422437713965e+29, -2.37684487542793e+29, -4.75368975085586e+29,
		// 					-9.50737950171172e+29, -1.901475900342344e+30, -3.802951800684688e+30, -7.605903601369376e+30, -1.5211807202738753e+31,
		// 					-3.0423614405477506e+31, -6.084722881095501e+31, -1.2169445762191002e+32, -2.4338891524382005e+32, -4.867778304876401e+32,
		// 					-9.735556609752802e+32, -1.9471113219505604e+33, -3.894222643901121e+33, -7.788445287802241e+33, -1.5576890575604483e+34,
		// 					-3.1153781151208966e+34, -6.230756230241793e+34, -1.2461512460483586e+35, -2.4923024920967173e+35, -4.9846049841934345e+35,
		// 					-9.969209968386869e+35,
		// 				},
		// 				Counts: []float64{
		// 					80, 81, 82, 83, 84, 85, 86, 87, 88, 89, 90, 91, 92, 93, 94, 95, 96, 97, 98, 99, 100, 101, 102, 103, 104, 105, 106, 107, 108, 109,
		// 					110, 111, 112, 113, 114, 115, 116, 117, 118, 119, 120,
		// 				},
		// 				Sum: 0, Count: 4100, Min: -9e+36, Max: -6.044629098073146e+23,
		// 			},
		// 			labels: map[string]string{"label1": "value1"},
		// 		},
		// 	},
		// },
		{
			name: "Exponential histogram with positive buckets",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				posBucketCounts := make([]uint64, 60)
				for i := range posBucketCounts {
					posBucketCounts[i] = uint64(i % 5)
				}
				histogramDP.Positive().BucketCounts().FromRaw(posBucketCounts)
				histogramDP.SetSum(1000)
				histogramDP.SetMin(2)
				histogramDP.SetMax(9e+17)
				histogramDP.SetCount(uint64(3662))
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
							Maximum:     aws.Float64(9e+17),
							Minimum:     aws.Float64(2),
							SampleCount: aws.Float64(120),
							Sum:         aws.Float64(1000),
						},
						Values: aws.Float64Slice([]float64{
							8.646911284551352e+17, 4.323455642275676e+17, 2.161727821137838e+17, 1.080863910568919e+17, 2.7021597764222976e+16,
							1.3510798882111488e+16, 6.755399441055744e+15, 3.377699720527872e+15, 8.44424930131968e+14, 4.22212465065984e+14, 2.11106232532992e+14,
							1.05553116266496e+14, 2.6388279066624e+13, 1.3194139533312e+13, 6.597069766656e+12, 3.298534883328e+12, 8.24633720832e+11, 4.12316860416e+11,
							2.06158430208e+11, 1.03079215104e+11, 2.5769803776e+10, 1.2884901888e+10, 6.442450944e+09, 3.221225472e+09, 8.05306368e+08, 4.02653184e+08,
							2.01326592e+08, 1.00663296e+08, 2.5165824e+07, 1.2582912e+07, 6.291456e+06, 3.145728e+06, 786432, 393216, 196608, 98304, 24576, 12288, 6144, 3072,
							768, 384, 192, 96, 24, 12, 6, 3,
						}),
						Counts: aws.Float64Slice([]float64{
							4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1,
						}),
					},
					entity:              entity,
					aggregationInterval: 0,
				},
			},
		},
		{
			name: "Exponential histogram with negative buckets",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				negBucketCounts := make([]uint64, 60)
				for i := range negBucketCounts {
					negBucketCounts[i] = uint64(i % 5)
				}
				histogramDP.Negative().BucketCounts().FromRaw(negBucketCounts)
				histogramDP.SetSum(-1000)
				histogramDP.SetMin(-9e+17)
				histogramDP.SetMax(-5)
				histogramDP.SetCount(uint64(3662))
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
							Maximum:     aws.Float64(-5),
							Minimum:     aws.Float64(-9e+17),
							SampleCount: aws.Float64(120),
							Sum:         aws.Float64(-1000),
						},
						Values: aws.Float64Slice([]float64{
							-3, -6, -12, -24, -96, -192, -384, -768, -3072, -6144, -12288, -24576, -98304, -196608, -393216, -786432,
							-3.145728e+06, -6.291456e+06, -1.2582912e+07, -2.5165824e+07, -1.00663296e+08, -2.01326592e+08, -4.02653184e+08, -8.05306368e+08, -3.221225472e+09,
							-6.442450944e+09, -1.2884901888e+10, -2.5769803776e+10, -1.03079215104e+11, -2.06158430208e+11, -4.12316860416e+11, -8.24633720832e+11, -3.298534883328e+12,
							-6.597069766656e+12, -1.3194139533312e+13, -2.6388279066624e+13, -1.05553116266496e+14, -2.11106232532992e+14, -4.22212465065984e+14, -8.44424930131968e+14,
							-3.377699720527872e+15, -6.755399441055744e+15, -1.3510798882111488e+16, -2.7021597764222976e+16, -1.080863910568919e+17, -2.161727821137838e+17,
							-4.323455642275676e+17, -8.646911284551352e+17,
						}),
						Counts: aws.Float64Slice([]float64{
							1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4,
						}),
					},
					entity:              entity,
					aggregationInterval: 0,
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
						Values: aws.Float64Slice([]float64{0}),
						Counts: aws.Float64Slice([]float64{5}),
					},
					entity:              entity,
					aggregationInterval: 0,
				},
			},
		},
		{
			name: "Exponential histogram with positive and zero buckets",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				posBucketCounts := make([]uint64, 120)
				for i := range posBucketCounts {
					posBucketCounts[i] = uint64(i + 1)
				}
				histogramDP.Positive().BucketCounts().FromRaw(posBucketCounts)
				histogramDP.SetZeroCount(2)
				histogramDP.SetSum(10000)
				histogramDP.SetMin(0)
				histogramDP.SetMax(9e+36)
				histogramDP.SetCount(uint64(7262))
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
							Maximum:     aws.Float64(9e+36),
							Minimum:     aws.Float64(0),
							SampleCount: aws.Float64(7262),
							Sum:         aws.Float64(10000),
						},
						Values: aws.Float64Slice([]float64{
							9.969209968386869e+35, 4.9846049841934345e+35, 2.4923024920967173e+35, 1.2461512460483586e+35, 6.230756230241793e+34,
							3.1153781151208966e+34, 1.5576890575604483e+34, 7.788445287802241e+33, 3.894222643901121e+33, 1.9471113219505604e+33,
							9.735556609752802e+32, 4.867778304876401e+32, 2.4338891524382005e+32, 1.2169445762191002e+32, 6.084722881095501e+31,
							3.0423614405477506e+31, 1.5211807202738753e+31, 7.605903601369376e+30, 3.802951800684688e+30, 1.901475900342344e+30,
							9.50737950171172e+29, 4.75368975085586e+29, 2.37684487542793e+29, 1.188422437713965e+29, 5.942112188569825e+28,
							2.9710560942849127e+28, 1.4855280471424563e+28, 7.427640235712282e+27, 3.713820117856141e+27, 1.8569100589280704e+27,
							9.284550294640352e+26, 4.642275147320176e+26, 2.321137573660088e+26, 1.160568786830044e+26, 5.80284393415022e+25,
							2.90142196707511e+25, 1.450710983537555e+25, 7.253554917687775e+24, 3.6267774588438875e+24, 1.8133887294219438e+24,
							9.066943647109719e+23, 4.5334718235548594e+23, 2.2667359117774297e+23, 1.1333679558887149e+23, 5.666839779443574e+22,
							2.833419889721787e+22, 1.4167099448608936e+22, 7.083549724304468e+21, 3.541774862152234e+21, 1.770887431076117e+21,
							8.854437155380585e+20, 4.4272185776902924e+20, 2.2136092888451462e+20, 1.1068046444225731e+20, 5.5340232221128655e+19,
							2.7670116110564327e+19, 1.3835058055282164e+19, 6.917529027641082e+18, 3.458764513820541e+18, 1.7293822569102705e+18,
							8.646911284551352e+17, 4.323455642275676e+17, 2.161727821137838e+17, 1.080863910568919e+17, 5.404319552844595e+16,
							2.7021597764222976e+16, 1.3510798882111488e+16, 6.755399441055744e+15, 3.377699720527872e+15, 1.688849860263936e+15,
							8.44424930131968e+14, 4.22212465065984e+14, 2.11106232532992e+14, 1.05553116266496e+14, 5.2776558133248e+13,
							2.6388279066624e+13, 1.3194139533312e+13, 6.597069766656e+12, 3.298534883328e+12, 1.649267441664e+12, 8.24633720832e+11,
							4.12316860416e+11, 2.06158430208e+11, 1.03079215104e+11, 5.1539607552e+10, 2.5769803776e+10, 1.2884901888e+10,
							6.442450944e+09, 3.221225472e+09, 1.610612736e+09, 8.05306368e+08, 4.02653184e+08, 2.01326592e+08, 1.00663296e+08,
							5.0331648e+07, 2.5165824e+07, 1.2582912e+07, 6.291456e+06, 3.145728e+06, 1.572864e+06, 786432, 393216, 196608, 98304,
							49152, 24576, 12288, 6144, 3072, 1536, 768, 384, 192, 96, 48, 24, 12, 6, 3, 1.5, 0,
						}),
						Counts: aws.Float64Slice([]float64{
							120, 119, 118, 117, 116, 115, 114, 113, 112, 111, 110, 109, 108, 107, 106, 105, 104, 103, 102, 101, 100, 99,
							98, 97, 96, 95, 94, 93, 92, 91, 90, 89, 88, 87, 86, 85, 84, 83, 82, 81, 80, 79, 78, 77, 76, 75, 74, 73, 72, 71, 70, 69, 68,
							67, 66, 65, 64, 63, 62, 61, 60, 59, 58, 57, 56, 55, 54, 53, 52, 51, 50, 49, 48, 47, 46, 45, 44, 43, 42, 41, 40, 39, 38, 37,
							36, 35, 34, 33, 32, 31, 30, 29, 28, 27, 26, 25, 24, 23, 22, 21, 20, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5,
							4, 3, 2, 1, 2,
						}),
					},
					entity:              entity,
					aggregationInterval: 0,
				},
			},
		},
		{
			name: "Exponential histogram with negative and zero buckets",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				negBucketCounts := make([]uint64, 120)
				for i := range negBucketCounts {
					negBucketCounts[i] = uint64(i + 1)
				}
				histogramDP.Negative().BucketCounts().FromRaw(negBucketCounts)
				histogramDP.SetZeroCount(2)
				histogramDP.SetSum(-10000)
				histogramDP.SetMin(-9e+36)
				histogramDP.SetMax(0)
				histogramDP.SetCount(uint64(7262))
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
							Minimum:     aws.Float64(-9e+36),
							SampleCount: aws.Float64(7262),
							Sum:         aws.Float64(-10000),
						},
						Values: aws.Float64Slice([]float64{
							0, -1.5, -3, -6, -12, -24, -48, -96, -192, -384, -768, -1536, -3072, -6144, -12288, -24576,
							-49152, -98304, -196608, -393216, -786432, -1.572864e+06, -3.145728e+06, -6.291456e+06, -1.2582912e+07,
							-2.5165824e+07, -5.0331648e+07, -1.00663296e+08, -2.01326592e+08, -4.02653184e+08, -8.05306368e+08,
							-1.610612736e+09, -3.221225472e+09, -6.442450944e+09, -1.2884901888e+10, -2.5769803776e+10,
							-5.1539607552e+10, -1.03079215104e+11, -2.06158430208e+11, -4.12316860416e+11, -8.24633720832e+11,
							-1.649267441664e+12, -3.298534883328e+12, -6.597069766656e+12, -1.3194139533312e+13, -2.6388279066624e+13,
							-5.2776558133248e+13, -1.05553116266496e+14, -2.11106232532992e+14, -4.22212465065984e+14, -8.44424930131968e+14,
							-1.688849860263936e+15, -3.377699720527872e+15, -6.755399441055744e+15, -1.3510798882111488e+16,
							-2.7021597764222976e+16, -5.404319552844595e+16, -1.080863910568919e+17, -2.161727821137838e+17,
							-4.323455642275676e+17, -8.646911284551352e+17, -1.7293822569102705e+18, -3.458764513820541e+18,
							-6.917529027641082e+18, -1.3835058055282164e+19, -2.7670116110564327e+19, -5.5340232221128655e+19,
							-1.1068046444225731e+20, -2.2136092888451462e+20, -4.4272185776902924e+20, -8.854437155380585e+20,
							-1.770887431076117e+21, -3.541774862152234e+21, -7.083549724304468e+21, -1.4167099448608936e+22,
							-2.833419889721787e+22, -5.666839779443574e+22, -1.1333679558887149e+23, -2.2667359117774297e+23,
							-4.5334718235548594e+23, -9.066943647109719e+23, -1.8133887294219438e+24, -3.6267774588438875e+24,
							-7.253554917687775e+24, -1.450710983537555e+25, -2.90142196707511e+25, -5.80284393415022e+25,
							-1.160568786830044e+26, -2.321137573660088e+26, -4.642275147320176e+26, -9.284550294640352e+26,
							-1.8569100589280704e+27, -3.713820117856141e+27, -7.427640235712282e+27, -1.4855280471424563e+28,
							-2.9710560942849127e+28, -5.942112188569825e+28, -1.188422437713965e+29, -2.37684487542793e+29,
							-4.75368975085586e+29, -9.50737950171172e+29, -1.901475900342344e+30, -3.802951800684688e+30,
							-7.605903601369376e+30, -1.5211807202738753e+31, -3.0423614405477506e+31, -6.084722881095501e+31,
							-1.2169445762191002e+32, -2.4338891524382005e+32, -4.867778304876401e+32, -9.735556609752802e+32,
							-1.9471113219505604e+33, -3.894222643901121e+33, -7.788445287802241e+33, -1.5576890575604483e+34,
							-3.1153781151208966e+34, -6.230756230241793e+34, -1.2461512460483586e+35, -2.4923024920967173e+35,
							-4.9846049841934345e+35, -9.969209968386869e+35,
						}),
						Counts: aws.Float64Slice([]float64{
							2, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24,
							25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46,
							47, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63, 64, 65, 66, 67, 68, 69, 70, 71, 72,
							73, 74, 75, 76, 77, 78, 79, 80, 81, 82, 83, 84, 85, 86, 87, 88, 89, 90, 91, 92, 93, 94, 95, 96, 97, 98, 99,
							100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111, 112, 113, 114, 115, 116, 117, 118, 119, 120,
						}),
					},
					entity:              entity,
					aggregationInterval: 0,
				},
			},
		},
		{
			name: "Exponential histogram with positive and negative buckets",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				posBucketCounts := make([]uint64, 60)
				for i := range posBucketCounts {
					posBucketCounts[i] = uint64(i + 1)
				}
				histogramDP.Positive().BucketCounts().FromRaw(posBucketCounts)
				negBucketCounts := make([]uint64, 60)
				for i := range negBucketCounts {
					negBucketCounts[i] = uint64(i + 1)
				}
				histogramDP.Negative().BucketCounts().FromRaw(negBucketCounts)
				histogramDP.SetSum(1000)
				histogramDP.SetMin(-9e+17)
				histogramDP.SetMax(9e+17)
				histogramDP.SetCount(uint64(3660))
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
							Maximum:     aws.Float64(9e17),
							Minimum:     aws.Float64(-9e17),
							SampleCount: aws.Float64(3660),
							Sum:         aws.Float64(1000),
						},
						Values: aws.Float64Slice([]float64{
							8.646911284551352e+17, 4.323455642275676e+17, 2.161727821137838e+17, 1.080863910568919e+17, 5.404319552844595e+16, 2.7021597764222976e+16,
							1.3510798882111488e+16, 6.755399441055744e+15, 3.377699720527872e+15, 1.688849860263936e+15, 8.44424930131968e+14, 4.22212465065984e+14,
							2.11106232532992e+14, 1.05553116266496e+14, 5.2776558133248e+13, 2.6388279066624e+13, 1.3194139533312e+13, 6.597069766656e+12, 3.298534883328e+12,
							1.649267441664e+12, 8.24633720832e+11, 4.12316860416e+11, 2.06158430208e+11, 1.03079215104e+11, 5.1539607552e+10, 2.5769803776e+10,
							1.2884901888e+10, 6.442450944e+09, 3.221225472e+09, 1.610612736e+09, 8.05306368e+08, 4.02653184e+08, 2.01326592e+08, 1.00663296e+08,
							5.0331648e+07, 2.5165824e+07, 1.2582912e+07, 6.291456e+06, 3.145728e+06, 1.572864e+06, 786432, 393216, 196608, 98304, 49152, 24576,
							12288, 6144, 3072, 1536, 768, 384, 192, 96, 48, 24, 12, 6, 3, 1.5, -1.5, -3, -6, -12, -24, -48, -96, -192, -384, -768, -1536, -3072,
							-6144, -12288, -24576, -49152, -98304, -196608, -393216, -786432, -1.572864e+06, -3.145728e+06, -6.291456e+06, -1.2582912e+07, -2.5165824e+07,
							-5.0331648e+07, -1.00663296e+08, -2.01326592e+08, -4.02653184e+08, -8.05306368e+08, -1.610612736e+09, -3.221225472e+09, -6.442450944e+09,
							-1.2884901888e+10, -2.5769803776e+10, -5.1539607552e+10, -1.03079215104e+11, -2.06158430208e+11, -4.12316860416e+11, -8.24633720832e+11,
							-1.649267441664e+12, -3.298534883328e+12, -6.597069766656e+12, -1.3194139533312e+13, -2.6388279066624e+13, -5.2776558133248e+13,
							-1.05553116266496e+14, -2.11106232532992e+14, -4.22212465065984e+14, -8.44424930131968e+14, -1.688849860263936e+15, -3.377699720527872e+15,
							-6.755399441055744e+15, -1.3510798882111488e+16, -2.7021597764222976e+16, -5.404319552844595e+16, -1.080863910568919e+17, -2.161727821137838e+17,
							-4.323455642275676e+17, -8.646911284551352e+17,
						}),
						Counts: aws.Float64Slice([]float64{
							60, 59, 58, 57, 56, 55, 54, 53, 52, 51, 50, 49, 48, 47, 46, 45, 44, 43, 42, 41, 40, 39, 38, 37, 36, 35, 34, 33, 32, 31, 30, 29, 28, 27, 26, 25, 24, 23, 22, 21, 20, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10, 9, 8, 7,
							6, 5, 4, 3, 2, 1, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33,
							34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60,
						}),
					},
					entity:              entity,
					aggregationInterval: 0,
				},
			},
		},
		{
			name: "Exponential histogram with positive, negative and zero buckets",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				posBucketCounts := make([]uint64, 60)
				for i := range posBucketCounts {
					posBucketCounts[i] = uint64(i % 5)
				}
				histogramDP.Positive().BucketCounts().FromRaw(posBucketCounts)
				histogramDP.SetZeroCount(2)
				negBucketCounts := make([]uint64, 60)
				for i := range negBucketCounts {
					negBucketCounts[i] = uint64(i % 5)
				}
				histogramDP.Negative().BucketCounts().FromRaw(negBucketCounts)
				histogramDP.SetSum(1000)
				histogramDP.SetMin(-9e+17)
				histogramDP.SetMax(9e+17)
				histogramDP.SetCount(uint64(3662))
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
							Maximum:     aws.Float64(9e+17),
							Minimum:     aws.Float64(-9e+17),
							SampleCount: aws.Float64(242),
							Sum:         aws.Float64(1000),
						},
						Values: aws.Float64Slice([]float64{
							8.646911284551352e+17, 4.323455642275676e+17, 2.161727821137838e+17, 1.080863910568919e+17, 2.7021597764222976e+16,
							1.3510798882111488e+16, 6.755399441055744e+15, 3.377699720527872e+15, 8.44424930131968e+14, 4.22212465065984e+14, 2.11106232532992e+14,
							1.05553116266496e+14, 2.6388279066624e+13, 1.3194139533312e+13, 6.597069766656e+12, 3.298534883328e+12, 8.24633720832e+11, 4.12316860416e+11,
							2.06158430208e+11, 1.03079215104e+11, 2.5769803776e+10, 1.2884901888e+10, 6.442450944e+09, 3.221225472e+09, 8.05306368e+08, 4.02653184e+08,
							2.01326592e+08, 1.00663296e+08, 2.5165824e+07, 1.2582912e+07, 6.291456e+06, 3.145728e+06, 786432, 393216, 196608, 98304, 24576, 12288, 6144, 3072,
							768, 384, 192, 96, 24, 12, 6, 3, 0, -3, -6, -12, -24, -96, -192, -384, -768, -3072, -6144, -12288, -24576, -98304, -196608, -393216, -786432,
							-3.145728e+06, -6.291456e+06, -1.2582912e+07, -2.5165824e+07, -1.00663296e+08, -2.01326592e+08, -4.02653184e+08, -8.05306368e+08, -3.221225472e+09,
							-6.442450944e+09, -1.2884901888e+10, -2.5769803776e+10, -1.03079215104e+11, -2.06158430208e+11, -4.12316860416e+11, -8.24633720832e+11, -3.298534883328e+12,
							-6.597069766656e+12, -1.3194139533312e+13, -2.6388279066624e+13, -1.05553116266496e+14, -2.11106232532992e+14, -4.22212465065984e+14, -8.44424930131968e+14,
							-3.377699720527872e+15, -6.755399441055744e+15, -1.3510798882111488e+16, -2.7021597764222976e+16, -1.080863910568919e+17, -2.161727821137838e+17,
							-4.323455642275676e+17, -8.646911284551352e+17,
						}),
						Counts: aws.Float64Slice([]float64{
							4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3,
							2, 1, 2, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4,
						}),
					},
					entity:              entity,
					aggregationInterval: 0,
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
		},
		{
			name: "Exponential histogram with with StorageResolution",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				posBucketCounts := make([]uint64, 60)
				for i := range posBucketCounts {
					posBucketCounts[i] = uint64(i % 5)
				}
				histogramDP.Positive().BucketCounts().FromRaw(posBucketCounts)
				histogramDP.SetZeroCount(2)
				negBucketCounts := make([]uint64, 60)
				for i := range negBucketCounts {
					negBucketCounts[i] = uint64(i % 5)
				}
				histogramDP.Negative().BucketCounts().FromRaw(negBucketCounts)
				histogramDP.SetSum(1000)
				histogramDP.SetMin(-9e+17)
				histogramDP.SetMax(9e+17)
				histogramDP.SetCount(uint64(3662))
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
							Maximum:     aws.Float64(9e+17),
							Minimum:     aws.Float64(-9e+17),
							SampleCount: aws.Float64(242),
							Sum:         aws.Float64(1000),
						},
						Values: aws.Float64Slice([]float64{
							8.646911284551352e+17, 4.323455642275676e+17, 2.161727821137838e+17, 1.080863910568919e+17, 2.7021597764222976e+16,
							1.3510798882111488e+16, 6.755399441055744e+15, 3.377699720527872e+15, 8.44424930131968e+14, 4.22212465065984e+14, 2.11106232532992e+14,
							1.05553116266496e+14, 2.6388279066624e+13, 1.3194139533312e+13, 6.597069766656e+12, 3.298534883328e+12, 8.24633720832e+11, 4.12316860416e+11,
							2.06158430208e+11, 1.03079215104e+11, 2.5769803776e+10, 1.2884901888e+10, 6.442450944e+09, 3.221225472e+09, 8.05306368e+08, 4.02653184e+08,
							2.01326592e+08, 1.00663296e+08, 2.5165824e+07, 1.2582912e+07, 6.291456e+06, 3.145728e+06, 786432, 393216, 196608, 98304, 24576, 12288, 6144, 3072,
							768, 384, 192, 96, 24, 12, 6, 3, 0, -3, -6, -12, -24, -96, -192, -384, -768, -3072, -6144, -12288, -24576, -98304, -196608, -393216, -786432,
							-3.145728e+06, -6.291456e+06, -1.2582912e+07, -2.5165824e+07, -1.00663296e+08, -2.01326592e+08, -4.02653184e+08, -8.05306368e+08, -3.221225472e+09,
							-6.442450944e+09, -1.2884901888e+10, -2.5769803776e+10, -1.03079215104e+11, -2.06158430208e+11, -4.12316860416e+11, -8.24633720832e+11, -3.298534883328e+12,
							-6.597069766656e+12, -1.3194139533312e+13, -2.6388279066624e+13, -1.05553116266496e+14, -2.11106232532992e+14, -4.22212465065984e+14, -8.44424930131968e+14,
							-3.377699720527872e+15, -6.755399441055744e+15, -1.3510798882111488e+16, -2.7021597764222976e+16, -1.080863910568919e+17, -2.161727821137838e+17,
							-4.323455642275676e+17, -8.646911284551352e+17,
						}),
						Counts: aws.Float64Slice([]float64{
							4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3,
							2, 1, 2, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4,
						}),
					},
					entity:              entity,
					aggregationInterval: 0,
				},
			},
		},
		{
			name: "Exponential histogram with AggregationInterval",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				posBucketCounts := make([]uint64, 60)
				for i := range posBucketCounts {
					posBucketCounts[i] = uint64(i % 5)
				}
				histogramDP.Positive().BucketCounts().FromRaw(posBucketCounts)
				histogramDP.SetZeroCount(2)
				negBucketCounts := make([]uint64, 60)
				for i := range negBucketCounts {
					negBucketCounts[i] = uint64(i % 5)
				}
				histogramDP.Negative().BucketCounts().FromRaw(negBucketCounts)
				histogramDP.SetSum(1000)
				histogramDP.SetMin(-9e+17)
				histogramDP.SetMax(9e+17)
				histogramDP.SetCount(uint64(3662))
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
						StorageResolution: aws.Int64(60),
						StatisticValues: &cloudwatch.StatisticSet{
							Maximum:     aws.Float64(9e+17),
							Minimum:     aws.Float64(-9e+17),
							SampleCount: aws.Float64(242),
							Sum:         aws.Float64(1000),
						},
						Values: aws.Float64Slice([]float64{
							8.646911284551352e+17, 4.323455642275676e+17, 2.161727821137838e+17, 1.080863910568919e+17, 2.7021597764222976e+16,
							1.3510798882111488e+16, 6.755399441055744e+15, 3.377699720527872e+15, 8.44424930131968e+14, 4.22212465065984e+14, 2.11106232532992e+14,
							1.05553116266496e+14, 2.6388279066624e+13, 1.3194139533312e+13, 6.597069766656e+12, 3.298534883328e+12, 8.24633720832e+11, 4.12316860416e+11,
							2.06158430208e+11, 1.03079215104e+11, 2.5769803776e+10, 1.2884901888e+10, 6.442450944e+09, 3.221225472e+09, 8.05306368e+08, 4.02653184e+08,
							2.01326592e+08, 1.00663296e+08, 2.5165824e+07, 1.2582912e+07, 6.291456e+06, 3.145728e+06, 786432, 393216, 196608, 98304, 24576, 12288, 6144, 3072,
							768, 384, 192, 96, 24, 12, 6, 3, 0, -3, -6, -12, -24, -96, -192, -384, -768, -3072, -6144, -12288, -24576, -98304, -196608, -393216, -786432,
							-3.145728e+06, -6.291456e+06, -1.2582912e+07, -2.5165824e+07, -1.00663296e+08, -2.01326592e+08, -4.02653184e+08, -8.05306368e+08, -3.221225472e+09,
							-6.442450944e+09, -1.2884901888e+10, -2.5769803776e+10, -1.03079215104e+11, -2.06158430208e+11, -4.12316860416e+11, -8.24633720832e+11, -3.298534883328e+12,
							-6.597069766656e+12, -1.3194139533312e+13, -2.6388279066624e+13, -1.05553116266496e+14, -2.11106232532992e+14, -4.22212465065984e+14, -8.44424930131968e+14,
							-3.377699720527872e+15, -6.755399441055744e+15, -1.3510798882111488e+16, -2.7021597764222976e+16, -1.080863910568919e+17, -2.161727821137838e+17,
							-4.323455642275676e+17, -8.646911284551352e+17,
						}),
						Counts: aws.Float64Slice([]float64{
							4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3,
							2, 1, 2, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4,
						}),
					},
					entity:              entity,
					aggregationInterval: 5 * time.Minute,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(_ *testing.T) {
			dps := ConvertOtelExponentialHistogramDataPoints(tc.histogramDPS, "foo", "none", 1, entity)

			assert.Equal(t, 1, len(dps))
			for i, expectedDP := range tc.expected {
				assert.Equal(t, expectedDP, dps[i], "datapoint mismatch at index %d", i)
			}
		})
	}
}

func TestConvertOtelMetrics_ExponentialHistogramWithSplit(t *testing.T) {
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
		name      string
		histogram pmetric.ExponentialHistogramDataPoint
		expected  []*aggregationDatum
	}{
		{
			name: "Exponential histogram positive buckets",
			histogram: func() pmetric.ExponentialHistogramDataPoint {
				histogramDP := pmetric.NewExponentialHistogramDataPoint()
				posBucketCounts := make([]uint64, 155)
				for i := range posBucketCounts {
					posBucketCounts[i] = uint64(i + 1)
				}
				histogramDP.Positive().BucketCounts().FromRaw(posBucketCounts)
				histogramDP.SetSum(4.1e46)
				histogramDP.SetMin(1)
				histogramDP.SetMax(4.0e46)
				histogramDP.SetCount(uint64(12090))
				histogramDP.SetScale(0)
				histogramDP.Attributes().PutStr("label1", "value1")
				histogramDP.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				return histogramDP
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
							Maximum:     aws.Float64(4.0e46), // first entry matches the maximum in the histogram metric
							Minimum:     aws.Float64(3.2e1),  // should be the start of bucket index 5
							SampleCount: aws.Float64(12075),  // sum of numbers 6 to 155
							Sum:         aws.Float64(4.1e46), // first entry matches the sum in the histogram metric
						},
						Values: aws.Float64Slice([]float64{
							// Largest bucket should be positive bucket index 154 (zero-origin)
							// Start: 2.28e46 (2^154)
							// Mid:   3.43e46 (start+end)/2
							// End:   4.57e46 (2^155)
							//
							// Smallest bucket should be bucket index 5 (150 positive buckets)
							// Start: 3.20e1 (2^5)
							// Mid:   4.80e1 (start+end)/2
							// End:   6.40e1 (2^6)
							3.4253944624943037e+46, 1.7126972312471519e+46, 8.563486156235759e+45, 4.2817430781178796e+45, 2.1408715390589398e+45, 1.0704357695294699e+45,
							5.3521788476473496e+44, 2.6760894238236748e+44, 1.3380447119118374e+44, 6.690223559559187e+43, 3.3451117797795935e+43, 1.6725558898897967e+43,
							8.362779449448984e+42, 4.181389724724492e+42, 2.090694862362246e+42, 1.045347431181123e+42, 5.226737155905615e+41, 2.6133685779528074e+41,
							1.3066842889764037e+41, 6.5334214448820185e+40, 3.2667107224410092e+40, 1.6333553612205046e+40, 8.166776806102523e+39, 4.0833884030512616e+39,
							2.0416942015256308e+39, 1.0208471007628154e+39, 5.104235503814077e+38, 2.5521177519070385e+38, 1.2760588759535192e+38, 6.380294379767596e+37,
							3.190147189883798e+37, 1.595073594941899e+37, 7.975367974709495e+36, 3.9876839873547476e+36, 1.9938419936773738e+36, 9.969209968386869e+35,
							4.9846049841934345e+35, 2.4923024920967173e+35, 1.2461512460483586e+35, 6.230756230241793e+34, 3.1153781151208966e+34, 1.5576890575604483e+34,
							7.788445287802241e+33, 3.894222643901121e+33, 1.9471113219505604e+33, 9.735556609752802e+32, 4.867778304876401e+32, 2.4338891524382005e+32,
							1.2169445762191002e+32, 6.084722881095501e+31, 3.0423614405477506e+31, 1.5211807202738753e+31, 7.605903601369376e+30, 3.802951800684688e+30,
							1.901475900342344e+30, 9.50737950171172e+29, 4.75368975085586e+29, 2.37684487542793e+29, 1.188422437713965e+29, 5.942112188569825e+28,
							2.9710560942849127e+28, 1.4855280471424563e+28, 7.427640235712282e+27, 3.713820117856141e+27, 1.8569100589280704e+27, 9.284550294640352e+26,
							4.642275147320176e+26, 2.321137573660088e+26, 1.160568786830044e+26, 5.80284393415022e+25, 2.90142196707511e+25, 1.450710983537555e+25,
							7.253554917687775e+24, 3.6267774588438875e+24, 1.8133887294219438e+24, 9.066943647109719e+23, 4.5334718235548594e+23, 2.2667359117774297e+23,
							1.1333679558887149e+23, 5.666839779443574e+22, 2.833419889721787e+22, 1.4167099448608936e+22, 7.083549724304468e+21, 3.541774862152234e+21,
							1.770887431076117e+21, 8.854437155380585e+20, 4.4272185776902924e+20, 2.2136092888451462e+20, 1.1068046444225731e+20, 5.5340232221128655e+19,
							2.7670116110564327e+19, 1.3835058055282164e+19, 6.917529027641082e+18, 3.458764513820541e+18, 1.7293822569102705e+18, 8.646911284551352e+17,
							4.323455642275676e+17, 2.161727821137838e+17, 1.080863910568919e+17, 5.404319552844595e+16, 2.7021597764222976e+16, 1.3510798882111488e+16,
							6.755399441055744e+15, 3.377699720527872e+15, 1.688849860263936e+15, 8.44424930131968e+14, 4.22212465065984e+14, 2.11106232532992e+14,
							1.05553116266496e+14, 5.2776558133248e+13, 2.6388279066624e+13, 1.3194139533312e+13, 6.597069766656e+12, 3.298534883328e+12, 1.649267441664e+12,
							8.24633720832e+11, 4.12316860416e+11, 2.06158430208e+11, 1.03079215104e+11, 5.1539607552e+10, 2.5769803776e+10, 1.2884901888e+10,
							6.442450944e+09, 3.221225472e+09, 1.610612736e+09, 8.05306368e+08, 4.02653184e+08, 2.01326592e+08, 1.00663296e+08, 5.0331648e+07, 2.5165824e+07,
							1.2582912e+07, 6.291456e+06, 3.145728e+06, 1.572864e+06, 786432, 393216, 196608, 98304, 49152, 24576, 12288, 6144, 3072, 1536, 768, 384, 192, 96,
							48,
						}),
						Counts: aws.Float64Slice([]float64{
							155, 154, 153, 152, 151, 150, 149, 148, 147, 146, 145, 144, 143, 142, 141, 140, 139, 138, 137, 136, 135, 134, 133, 132, 131, 130, 129, 128,
							127, 126, 125, 124, 123, 122, 121, 120, 119, 118, 117, 116, 115, 114, 113, 112, 111, 110, 109, 108, 107, 106, 105, 104, 103, 102, 101, 100,
							99, 98, 97, 96, 95, 94, 93, 92, 91, 90, 89, 88, 87, 86, 85, 84, 83, 82, 81, 80, 79, 78, 77, 76, 75, 74, 73, 72, 71, 70, 69, 68, 67, 66, 65,
							64, 63, 62, 61, 60, 59, 58, 57, 56, 55, 54, 53, 52, 51, 50, 49, 48, 47, 46, 45, 44, 43, 42, 41, 40, 39, 38, 37, 36, 35, 34, 33, 32, 31, 30,
							29, 28, 27, 26, 25, 24, 23, 22, 21, 20, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6,
						}),
					},
					entity:              entity,
					aggregationInterval: 0,
				},
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
							Maximum:     aws.Float64(3.2e1), // should be the start of bucket negative bucket index 5
							Minimum:     aws.Float64(1),     // last entry matches the minimum in the histogram metric
							SampleCount: aws.Float64(15),    // sum of numbers 1 to 5
							Sum:         aws.Float64(0),     // not the first entry, sum should be 0
						},
						Values: aws.Float64Slice([]float64{
							// Largest bucket should be bucket index 4 since last datum had up to bucket index 5
							// Start: 1.60e1 (2^4)
							// Mid:   2.40e1 (start+end)/2
							// End:   3.20e1 (2^5)
							//
							// Smallest bucket should be bucket index 0
							// Start: 1.0e0 (2^0)
							// Mid:   1.5e0 (start+end)/2
							// End:   2.0e0 (2^1)
							24, 12, 6, 3, 1.5,
						}),
						Counts: aws.Float64Slice([]float64{
							5, 4, 3, 2, 1,
						}),
					},
					entity:              entity,
					aggregationInterval: 0,
				},
			},
		},
		{
			name: "Exponential histogram positive and zero buckets",
			histogram: func() pmetric.ExponentialHistogramDataPoint {
				histogramDP := pmetric.NewExponentialHistogramDataPoint()
				posBucketCounts := make([]uint64, 160)
				for i := range posBucketCounts {
					posBucketCounts[i] = uint64(i + 1)
				}
				histogramDP.Positive().BucketCounts().FromRaw(posBucketCounts)
				histogramDP.SetZeroCount(5)
				histogramDP.SetSum(4.1e46)
				histogramDP.SetMin(0)
				histogramDP.SetMax(4.0e46)
				histogramDP.SetCount(uint64(12095))
				histogramDP.SetScale(0)
				histogramDP.Attributes().PutStr("label1", "value1")
				histogramDP.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				return histogramDP
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
							Maximum:     aws.Float64(4.0e46), // first entry matches the maximum in the histogram metric
							Minimum:     aws.Float64(3.2e1),  // should be the start of bucket index 5
							SampleCount: aws.Float64(12075),  // sum of numbers 6 to 155
							Sum:         aws.Float64(4.1e46), // first entry matches the sum in the histogram metric
						},
						Values: aws.Float64Slice([]float64{
							// Largest bucket should be positive bucket index 154 (zero-origin)
							// Start: 2.28e46 (2^154)
							// Mid:   3.43e46 (start+end)/2
							// End:   4.57e46 (2^155)
							//
							// Smallest bucket should be bucket index 5 (150 positive buckets)
							// Start: 3.20e1 (2^5)
							// Mid:   4.80e1 (start+end)/2
							// End:   6.40e1 (2^6)
							3.4253944624943037e+46, 1.7126972312471519e+46, 8.563486156235759e+45, 4.2817430781178796e+45, 2.1408715390589398e+45, 1.0704357695294699e+45,
							5.3521788476473496e+44, 2.6760894238236748e+44, 1.3380447119118374e+44, 6.690223559559187e+43, 3.3451117797795935e+43, 1.6725558898897967e+43,
							8.362779449448984e+42, 4.181389724724492e+42, 2.090694862362246e+42, 1.045347431181123e+42, 5.226737155905615e+41, 2.6133685779528074e+41,
							1.3066842889764037e+41, 6.5334214448820185e+40, 3.2667107224410092e+40, 1.6333553612205046e+40, 8.166776806102523e+39, 4.0833884030512616e+39,
							2.0416942015256308e+39, 1.0208471007628154e+39, 5.104235503814077e+38, 2.5521177519070385e+38, 1.2760588759535192e+38, 6.380294379767596e+37,
							3.190147189883798e+37, 1.595073594941899e+37, 7.975367974709495e+36, 3.9876839873547476e+36, 1.9938419936773738e+36, 9.969209968386869e+35,
							4.9846049841934345e+35, 2.4923024920967173e+35, 1.2461512460483586e+35, 6.230756230241793e+34, 3.1153781151208966e+34, 1.5576890575604483e+34,
							7.788445287802241e+33, 3.894222643901121e+33, 1.9471113219505604e+33, 9.735556609752802e+32, 4.867778304876401e+32, 2.4338891524382005e+32,
							1.2169445762191002e+32, 6.084722881095501e+31, 3.0423614405477506e+31, 1.5211807202738753e+31, 7.605903601369376e+30, 3.802951800684688e+30,
							1.901475900342344e+30, 9.50737950171172e+29, 4.75368975085586e+29, 2.37684487542793e+29, 1.188422437713965e+29, 5.942112188569825e+28,
							2.9710560942849127e+28, 1.4855280471424563e+28, 7.427640235712282e+27, 3.713820117856141e+27, 1.8569100589280704e+27, 9.284550294640352e+26,
							4.642275147320176e+26, 2.321137573660088e+26, 1.160568786830044e+26, 5.80284393415022e+25, 2.90142196707511e+25, 1.450710983537555e+25,
							7.253554917687775e+24, 3.6267774588438875e+24, 1.8133887294219438e+24, 9.066943647109719e+23, 4.5334718235548594e+23, 2.2667359117774297e+23,
							1.1333679558887149e+23, 5.666839779443574e+22, 2.833419889721787e+22, 1.4167099448608936e+22, 7.083549724304468e+21, 3.541774862152234e+21,
							1.770887431076117e+21, 8.854437155380585e+20, 4.4272185776902924e+20, 2.2136092888451462e+20, 1.1068046444225731e+20, 5.5340232221128655e+19,
							2.7670116110564327e+19, 1.3835058055282164e+19, 6.917529027641082e+18, 3.458764513820541e+18, 1.7293822569102705e+18, 8.646911284551352e+17,
							4.323455642275676e+17, 2.161727821137838e+17, 1.080863910568919e+17, 5.404319552844595e+16, 2.7021597764222976e+16, 1.3510798882111488e+16,
							6.755399441055744e+15, 3.377699720527872e+15, 1.688849860263936e+15, 8.44424930131968e+14, 4.22212465065984e+14, 2.11106232532992e+14,
							1.05553116266496e+14, 5.2776558133248e+13, 2.6388279066624e+13, 1.3194139533312e+13, 6.597069766656e+12, 3.298534883328e+12, 1.649267441664e+12,
							8.24633720832e+11, 4.12316860416e+11, 2.06158430208e+11, 1.03079215104e+11, 5.1539607552e+10, 2.5769803776e+10, 1.2884901888e+10,
							6.442450944e+09, 3.221225472e+09, 1.610612736e+09, 8.05306368e+08, 4.02653184e+08, 2.01326592e+08, 1.00663296e+08, 5.0331648e+07, 2.5165824e+07,
							1.2582912e+07, 6.291456e+06, 3.145728e+06, 1.572864e+06, 786432, 393216, 196608, 98304, 49152, 24576, 12288, 6144, 3072, 1536, 768, 384, 192, 96,
							48,
						}),
						Counts: aws.Float64Slice([]float64{
							155, 154, 153, 152, 151, 150, 149, 148, 147, 146, 145, 144, 143, 142, 141, 140, 139, 138, 137, 136, 135, 134, 133, 132, 131, 130, 129, 128,
							127, 126, 125, 124, 123, 122, 121, 120, 119, 118, 117, 116, 115, 114, 113, 112, 111, 110, 109, 108, 107, 106, 105, 104, 103, 102, 101, 100,
							99, 98, 97, 96, 95, 94, 93, 92, 91, 90, 89, 88, 87, 86, 85, 84, 83, 82, 81, 80, 79, 78, 77, 76, 75, 74, 73, 72, 71, 70, 69, 68, 67, 66, 65,
							64, 63, 62, 61, 60, 59, 58, 57, 56, 55, 54, 53, 52, 51, 50, 49, 48, 47, 46, 45, 44, 43, 42, 41, 40, 39, 38, 37, 36, 35, 34, 33, 32, 31, 30,
							29, 28, 27, 26, 25, 24, 23, 22, 21, 20, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6,
						}),
					},
					entity:              entity,
					aggregationInterval: 0,
				},
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
							Maximum:     aws.Float64(3.2e1), // should be the start of bucket negative bucket index 5
							Minimum:     aws.Float64(0),     // last entry matches the minimum in the histogram metric
							SampleCount: aws.Float64(20),    // sum of numbers 1 to 5 + 5
							Sum:         aws.Float64(0),     // not the first entry, sum should be 0
						},
						Values: aws.Float64Slice([]float64{
							// Largest bucket should be bucket index 4 since last datum had up to bucket index 5
							// Start: 1.60e1 (2^4)
							// Mid:   2.40e1 (start+end)/2
							// End:   3.20e1 (2^5)
							//
							// Smallest bucket should be bucket index 0
							// Start: 1.0e0 (2^0)
							// Mid:   1.5e0 (start+end)/2
							// End:   2.0e0 (2^1)
							24, 12, 6, 3, 1.5, 0,
						}),
						Counts: aws.Float64Slice([]float64{
							5, 5, 4, 3, 2, 1,
						}),
					},
					entity:              entity,
					aggregationInterval: 0,
				},
			},
		},
		{
			name: "Exponential histogram negative buckets",
			histogram: func() pmetric.ExponentialHistogramDataPoint {
				histogramDP := pmetric.NewExponentialHistogramDataPoint()
				negBucketCounts := make([]uint64, 155)
				for i := range negBucketCounts {
					negBucketCounts[i] = uint64(i + 1)
				}
				histogramDP.Negative().BucketCounts().FromRaw(negBucketCounts)
				histogramDP.SetSum(-4.1e46)
				histogramDP.SetMin(-4.0e46)
				histogramDP.SetMax(-1)
				histogramDP.SetCount(uint64(12090))
				histogramDP.SetScale(0)
				histogramDP.Attributes().PutStr("label1", "value1")
				histogramDP.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				return histogramDP
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
							Maximum:     aws.Float64(-1),                    // first entry matches the maximum in the histogram metric
							Minimum:     aws.Float64(-1.42724769270596e+45), // should be the end of negative bucket index 149
							SampleCount: aws.Float64(11325),                 // sum of numbers 1 to 155
							Sum:         aws.Float64(-4.1e46),               // first entry matches the sum set in the histogram metric
						},
						Values: aws.Float64Slice([]float64{
							// Largest bucket should be negative bucket index 0 (zero-origin)
							// Start: -1.0e0 (2^0)
							// Mid:   -1.5e0 (start+end)/2
							// End:   -2.0e0 (2^1)
							//
							// Largest bucket should be negative bucket index 149 (150 positive buckets)
							// Start: -7.13e44 (2^149)
							// Mid:   -1.07e45 (start+end)/2
							// End:   -1.43e45 (2^150)
							-1.5, -3, -6, -12, -24, -48, -96, -192, -384, -768, -1536, -3072, -6144, -12288, -24576, -49152, -98304, -196608, -393216, -786432,
							-1.572864e+06, -3.145728e+06, -6.291456e+06, -1.2582912e+07, -2.5165824e+07, -5.0331648e+07, -1.00663296e+08, -2.01326592e+08, -4.02653184e+08,
							-8.05306368e+08, -1.610612736e+09, -3.221225472e+09, -6.442450944e+09, -1.2884901888e+10, -2.5769803776e+10, -5.1539607552e+10,
							-1.03079215104e+11, -2.06158430208e+11, -4.12316860416e+11, -8.24633720832e+11, -1.649267441664e+12, -3.298534883328e+12, -6.597069766656e+12,
							-1.3194139533312e+13, -2.6388279066624e+13, -5.2776558133248e+13, -1.05553116266496e+14, -2.11106232532992e+14, -4.22212465065984e+14,
							-8.44424930131968e+14, -1.688849860263936e+15, -3.377699720527872e+15, -6.755399441055744e+15, -1.3510798882111488e+16, -2.7021597764222976e+16,
							-5.404319552844595e+16, -1.080863910568919e+17, -2.161727821137838e+17, -4.323455642275676e+17, -8.646911284551352e+17, -1.7293822569102705e+18,
							-3.458764513820541e+18, -6.917529027641082e+18, -1.3835058055282164e+19, -2.7670116110564327e+19, -5.5340232221128655e+19,
							-1.1068046444225731e+20, -2.2136092888451462e+20, -4.4272185776902924e+20, -8.854437155380585e+20, -1.770887431076117e+21, -3.541774862152234e+21,
							-7.083549724304468e+21, -1.4167099448608936e+22, -2.833419889721787e+22, -5.666839779443574e+22, -1.1333679558887149e+23, -2.2667359117774297e+23,
							-4.5334718235548594e+23, -9.066943647109719e+23, -1.8133887294219438e+24, -3.6267774588438875e+24, -7.253554917687775e+24, -1.450710983537555e+25,
							-2.90142196707511e+25, -5.80284393415022e+25, -1.160568786830044e+26, -2.321137573660088e+26, -4.642275147320176e+26, -9.284550294640352e+26,
							-1.8569100589280704e+27, -3.713820117856141e+27, -7.427640235712282e+27, -1.4855280471424563e+28, -2.9710560942849127e+28, -5.942112188569825e+28,
							-1.188422437713965e+29, -2.37684487542793e+29, -4.75368975085586e+29, -9.50737950171172e+29, -1.901475900342344e+30, -3.802951800684688e+30,
							-7.605903601369376e+30, -1.5211807202738753e+31, -3.0423614405477506e+31, -6.084722881095501e+31, -1.2169445762191002e+32,
							-2.4338891524382005e+32, -4.867778304876401e+32, -9.735556609752802e+32, -1.9471113219505604e+33, -3.894222643901121e+33, -7.788445287802241e+33,
							-1.5576890575604483e+34, -3.1153781151208966e+34, -6.230756230241793e+34, -1.2461512460483586e+35, -2.4923024920967173e+35,
							-4.9846049841934345e+35, -9.969209968386869e+35, -1.9938419936773738e+36, -3.9876839873547476e+36, -7.975367974709495e+36, -1.595073594941899e+37,
							-3.190147189883798e+37, -6.380294379767596e+37, -1.2760588759535192e+38, -2.5521177519070385e+38, -5.104235503814077e+38, -1.0208471007628154e+39,
							-2.0416942015256308e+39, -4.0833884030512616e+39, -8.166776806102523e+39, -1.6333553612205046e+40, -3.2667107224410092e+40,
							-6.5334214448820185e+40, -1.3066842889764037e+41, -2.6133685779528074e+41, -5.226737155905615e+41, -1.045347431181123e+42, -2.090694862362246e+42,
							-4.181389724724492e+42, -8.362779449448984e+42, -1.6725558898897967e+43, -3.3451117797795935e+43, -6.690223559559187e+43, -1.3380447119118374e+44,
							-2.6760894238236748e+44, -5.3521788476473496e+44, -1.0704357695294699e+45,
						}),
						Counts: aws.Float64Slice([]float64{
							1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39,
							40, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63, 64, 65, 66, 67, 68, 69, 70, 71, 72, 73, 74, 75, 76,
							77, 78, 79, 80, 81, 82, 83, 84, 85, 86, 87, 88, 89, 90, 91, 92, 93, 94, 95, 96, 97, 98, 99, 100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110,
							111, 112, 113, 114, 115, 116, 117, 118, 119, 120, 121, 122, 123, 124, 125, 126, 127, 128, 129, 130, 131, 132, 133, 134, 135, 136, 137, 138, 139,
							140, 141, 142, 143, 144, 145, 146, 147, 148, 149, 150,
						}),
					},
					entity:              entity,
					aggregationInterval: 0,
				},
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
							Maximum:     aws.Float64(-1.42724769270596e+45), // should be the start of negative bucket index 150
							Minimum:     aws.Float64(-4.0e46),               // last entry matches the minimum set in the histogram metric
							SampleCount: aws.Float64(765),                   // sum of numbers 151 to 155
							Sum:         aws.Float64(0),                     // not the first entry, sum should be 0
						},
						Values: aws.Float64Slice([]float64{
							// Largest bucket should be negative bucket index 150 since last datum had up to bucket index 150
							// Start: -1.43e45 (2^150)
							// Mid:   -2.14e45 (start+end)/2
							// End:   -2.85e45 (2^151)
							//
							// Smallest bucket should be bucket index 154
							// Start: -2.28e46 (2^154)
							// Mid:   -3.43e46 (start+end)/2
							// End:   -4.57e46 (2^155)
							-2.1408715390589398e+45, -4.2817430781178796e+45, -8.563486156235759e+45, -1.7126972312471519e+46, -3.4253944624943037e+46,
						}),
						Counts: aws.Float64Slice([]float64{
							151, 152, 153, 154, 155,
						}),
					},
					entity:              entity,
					aggregationInterval: 0,
				},
			},
		},
		{
			name: "Exponential histogram positive, negative, and zero buckets",
			histogram: func() pmetric.ExponentialHistogramDataPoint {
				histogramDP := pmetric.NewExponentialHistogramDataPoint()
				posBucketCounts := make([]uint64, 120)
				for i := range posBucketCounts {
					posBucketCounts[i] = uint64(i + 1)
				}
				histogramDP.Positive().BucketCounts().FromRaw(posBucketCounts)
				histogramDP.SetZeroCount(10)
				negBucketCounts := make([]uint64, 120)
				for i := range negBucketCounts {
					negBucketCounts[i] = uint64(i + 1)
				}
				histogramDP.Negative().BucketCounts().FromRaw(negBucketCounts)
				histogramDP.SetSum(1000)
				histogramDP.SetMin(-1.1e36)
				histogramDP.SetMax(1.1e36)
				histogramDP.SetCount(uint64(3660))
				histogramDP.SetScale(0)
				histogramDP.Attributes().PutStr("label1", "value1")
				histogramDP.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				return histogramDP
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
							Maximum:     aws.Float64(1.1e36),          // first entry matches the maximum in the histogram metric
							Minimum:     aws.Float64(-5.36870912e+08), // should be the end of negative bucket index 29
							SampleCount: aws.Float64(7705),            // sum of number 120 to 29
							Sum:         aws.Float64(1000),            // first entry matches the sum set in the histogram metric
						},
						Values: aws.Float64Slice([]float64{
							// Largest bucket should be positive bucket index 119 (zero-origin)
							// Start: 6.65e35 (2^119)
							// Mid:   9.97e35 (start+end)/2
							// End:   1.33e36 (2^120)
							//
							// Smallest bucket should be negative bucket index 28 (120 positive buckets + 1 zero-value bucket + 29 negative buckets)
							// Start: -2.67e8 (2^28)
							// Mid:   -4.03e8 (start+end)/2
							// End:   -5.37e8 (2^29)
							9.969209968386869e+35, 4.9846049841934345e+35, 2.4923024920967173e+35, 1.2461512460483586e+35, 6.230756230241793e+34,
							3.1153781151208966e+34, 1.5576890575604483e+34, 7.788445287802241e+33, 3.894222643901121e+33, 1.9471113219505604e+33,
							9.735556609752802e+32, 4.867778304876401e+32, 2.4338891524382005e+32, 1.2169445762191002e+32, 6.084722881095501e+31,
							3.0423614405477506e+31, 1.5211807202738753e+31, 7.605903601369376e+30, 3.802951800684688e+30, 1.901475900342344e+30,
							9.50737950171172e+29, 4.75368975085586e+29, 2.37684487542793e+29, 1.188422437713965e+29, 5.942112188569825e+28, 2.9710560942849127e+28,
							1.4855280471424563e+28, 7.427640235712282e+27, 3.713820117856141e+27, 1.8569100589280704e+27, 9.284550294640352e+26,
							4.642275147320176e+26, 2.321137573660088e+26, 1.160568786830044e+26, 5.80284393415022e+25, 2.90142196707511e+25, 1.450710983537555e+25,
							7.253554917687775e+24, 3.6267774588438875e+24, 1.8133887294219438e+24, 9.066943647109719e+23, 4.5334718235548594e+23,
							2.2667359117774297e+23, 1.1333679558887149e+23, 5.666839779443574e+22, 2.833419889721787e+22, 1.4167099448608936e+22,
							7.083549724304468e+21, 3.541774862152234e+21, 1.770887431076117e+21, 8.854437155380585e+20, 4.4272185776902924e+20,
							2.2136092888451462e+20, 1.1068046444225731e+20, 5.5340232221128655e+19, 2.7670116110564327e+19, 1.3835058055282164e+19,
							6.917529027641082e+18, 3.458764513820541e+18, 1.7293822569102705e+18, 8.646911284551352e+17, 4.323455642275676e+17,
							2.161727821137838e+17, 1.080863910568919e+17, 5.404319552844595e+16, 2.7021597764222976e+16, 1.3510798882111488e+16,
							6.755399441055744e+15, 3.377699720527872e+15, 1.688849860263936e+15, 8.44424930131968e+14, 4.22212465065984e+14, 2.11106232532992e+14,
							1.05553116266496e+14, 5.2776558133248e+13, 2.6388279066624e+13, 1.3194139533312e+13, 6.597069766656e+12, 3.298534883328e+12,
							1.649267441664e+12, 8.24633720832e+11, 4.12316860416e+11, 2.06158430208e+11, 1.03079215104e+11, 5.1539607552e+10, 2.5769803776e+10,
							1.2884901888e+10, 6.442450944e+09, 3.221225472e+09, 1.610612736e+09, 8.05306368e+08, 4.02653184e+08, 2.01326592e+08, 1.00663296e+08,
							5.0331648e+07, 2.5165824e+07, 1.2582912e+07, 6.291456e+06, 3.145728e+06, 1.572864e+06, 786432, 393216, 196608, 98304, 49152, 24576,
							12288, 6144, 3072, 1536, 768, 384, 192, 96, 48, 24, 12, 6, 3, 1.5, 0, -1.5, -3, -6, -12, -24, -48, -96, -192, -384, -768, -1536,
							-3072, -6144, -12288, -24576, -49152, -98304, -196608, -393216, -786432, -1.572864e+06, -3.145728e+06, -6.291456e+06, -1.2582912e+07,
							-2.5165824e+07, -5.0331648e+07, -1.00663296e+08, -2.01326592e+08, -4.02653184e+08,
						}),
						Counts: aws.Float64Slice([]float64{
							120, 119, 118, 117, 116, 115, 114, 113, 112, 111, 110, 109, 108, 107, 106, 105, 104, 103, 102, 101, 100, 99, 98, 97, 96, 95, 94, 93, 92,
							91, 90, 89, 88, 87, 86, 85, 84, 83, 82, 81, 80, 79, 78, 77, 76, 75, 74, 73, 72, 71, 70, 69, 68, 67, 66, 65, 64, 63, 62, 61, 60, 59, 58,
							57, 56, 55, 54, 53, 52, 51, 50, 49, 48, 47, 46, 45, 44, 43, 42, 41, 40, 39, 38, 37, 36, 35, 34, 33, 32, 31, 30, 29, 28, 27, 26, 25, 24,
							23, 22, 21, 20, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 10, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14,
							15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29,
						}),
					},
					entity:              entity,
					aggregationInterval: 0,
				},
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
							Maximum:     aws.Float64(-5.36870912e+08), // should be the start of bucket negative bucket index 28
							Minimum:     aws.Float64(-1.1e+36),        // last entry matches the minimum set in the histogram metric
							SampleCount: aws.Float64(6825),            // sum of numbers 30 to 120
							Sum:         aws.Float64(0),               // not the first entry, sum should be 0
						},
						Values: aws.Float64Slice([]float64{
							// Largest bucket should be negative bucket index 29 since last datum had up to bucket index 28
							// Start: -5.37e8 (2^29)
							// Mid:   -8.05e8 (start+end)/2
							// End:   -1.07e9 (2^30)
							//
							// Smallest bucket should be negative bucket index 119
							// Start: -6.65e35 (2^119)
							// Mid:   -9.97e35 (start+end)/2
							// End:   -1.33e36 (2^120)
							-8.05306368e+08, -1.610612736e+09, -3.221225472e+09, -6.442450944e+09, -1.2884901888e+10, -2.5769803776e+10, -5.1539607552e+10,
							-1.03079215104e+11, -2.06158430208e+11, -4.12316860416e+11, -8.24633720832e+11, -1.649267441664e+12, -3.298534883328e+12,
							-6.597069766656e+12, -1.3194139533312e+13, -2.6388279066624e+13, -5.2776558133248e+13, -1.05553116266496e+14, -2.11106232532992e+14,
							-4.22212465065984e+14, -8.44424930131968e+14, -1.688849860263936e+15, -3.377699720527872e+15, -6.755399441055744e+15,
							-1.3510798882111488e+16, -2.7021597764222976e+16, -5.404319552844595e+16, -1.080863910568919e+17, -2.161727821137838e+17,
							-4.323455642275676e+17, -8.646911284551352e+17, -1.7293822569102705e+18, -3.458764513820541e+18, -6.917529027641082e+18,
							-1.3835058055282164e+19, -2.7670116110564327e+19, -5.5340232221128655e+19, -1.1068046444225731e+20, -2.2136092888451462e+20,
							-4.4272185776902924e+20, -8.854437155380585e+20, -1.770887431076117e+21, -3.541774862152234e+21, -7.083549724304468e+21,
							-1.4167099448608936e+22, -2.833419889721787e+22, -5.666839779443574e+22, -1.1333679558887149e+23, -2.2667359117774297e+23,
							-4.5334718235548594e+23, -9.066943647109719e+23, -1.8133887294219438e+24, -3.6267774588438875e+24, -7.253554917687775e+24,
							-1.450710983537555e+25, -2.90142196707511e+25, -5.80284393415022e+25, -1.160568786830044e+26, -2.321137573660088e+26,
							-4.642275147320176e+26, -9.284550294640352e+26, -1.8569100589280704e+27, -3.713820117856141e+27, -7.427640235712282e+27,
							-1.4855280471424563e+28, -2.9710560942849127e+28, -5.942112188569825e+28, -1.188422437713965e+29, -2.37684487542793e+29,
							-4.75368975085586e+29, -9.50737950171172e+29, -1.901475900342344e+30, -3.802951800684688e+30, -7.605903601369376e+30,
							-1.5211807202738753e+31, -3.0423614405477506e+31, -6.084722881095501e+31, -1.2169445762191002e+32, -2.4338891524382005e+32,
							-4.867778304876401e+32, -9.735556609752802e+32, -1.9471113219505604e+33, -3.894222643901121e+33, -7.788445287802241e+33,
							-1.5576890575604483e+34, -3.1153781151208966e+34, -6.230756230241793e+34, -1.2461512460483586e+35, -2.4923024920967173e+35,
							-4.9846049841934345e+35, -9.969209968386869e+35,
						}),
						Counts: aws.Float64Slice([]float64{
							30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63,
							64, 65, 66, 67, 68, 69, 70, 71, 72, 73, 74, 75, 76, 77, 78, 79, 80, 81, 82, 83, 84, 85, 86, 87, 88, 89, 90, 91, 92, 93, 94, 95, 96, 97,
							98, 99, 100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111, 112, 113, 114, 115, 116, 117, 118, 119, 120,
						}),
					},
					entity:              entity,
					aggregationInterval: 0,
				},
			},
		},
		// {
		// 	name: "Exponential histogram with exact 200 buckets, including positive, negative buckets",
		// 	histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
		// 		histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
		// 		histogramDP := histogramDPS.AppendEmpty()
		// 		posBucketCounts := make([]uint64, 100)
		// 		for i := range posBucketCounts {
		// 			posBucketCounts[i] = uint64(i + 1)
		// 		}
		// 		histogramDP.Positive().BucketCounts().FromRaw(posBucketCounts)
		// 		negBucketCounts := make([]uint64, 100)
		// 		for i := range negBucketCounts {
		// 			negBucketCounts[i] = uint64(i + 1)
		// 		}
		// 		histogramDP.Negative().BucketCounts().FromRaw(negBucketCounts)
		// 		histogramDP.SetSum(100000)
		// 		histogramDP.SetMin(-9e+36)
		// 		histogramDP.SetMax(9e+36)
		// 		histogramDP.SetCount(uint64(3662))
		// 		histogramDP.Attributes().PutStr("label1", "value1")
		// 		return histogramDPS
		// 	}(),
		// 	expectedDatapoints: []dataPoint{
		// 		{
		// 			name: "foo",
		// 			value: &cWMetricHistogram{
		// 				Values: []float64{
		// 					9.50737950171172e+29, 4.75368975085586e+29, 2.37684487542793e+29, 1.188422437713965e+29, 5.942112188569825e+28,
		// 					2.9710560942849127e+28, 1.4855280471424563e+28, 7.427640235712282e+27, 3.713820117856141e+27, 1.8569100589280704e+27,
		// 					9.284550294640352e+26, 4.642275147320176e+26, 2.321137573660088e+26, 1.160568786830044e+26, 5.80284393415022e+25,
		// 					2.90142196707511e+25, 1.450710983537555e+25, 7.253554917687775e+24, 3.6267774588438875e+24, 1.8133887294219438e+24,
		// 					9.066943647109719e+23, 4.5334718235548594e+23, 2.2667359117774297e+23, 1.1333679558887149e+23, 5.666839779443574e+22,
		// 					2.833419889721787e+22, 1.4167099448608936e+22, 7.083549724304468e+21, 3.541774862152234e+21, 1.770887431076117e+21,
		// 					8.854437155380585e+20, 4.4272185776902924e+20, 2.2136092888451462e+20, 1.1068046444225731e+20, 5.5340232221128655e+19,
		// 					2.7670116110564327e+19, 1.3835058055282164e+19, 6.917529027641082e+18, 3.458764513820541e+18, 1.7293822569102705e+18,
		// 					8.646911284551352e+17, 4.323455642275676e+17, 2.161727821137838e+17, 1.080863910568919e+17, 5.404319552844595e+16,
		// 					2.7021597764222976e+16, 1.3510798882111488e+16, 6.755399441055744e+15, 3.377699720527872e+15, 1.688849860263936e+15,
		// 					8.44424930131968e+14, 4.22212465065984e+14, 2.11106232532992e+14, 1.05553116266496e+14, 5.2776558133248e+13,
		// 					2.6388279066624e+13, 1.3194139533312e+13, 6.597069766656e+12, 3.298534883328e+12, 1.649267441664e+12, 8.24633720832e+11,
		// 					4.12316860416e+11, 2.06158430208e+11, 1.03079215104e+11, 5.1539607552e+10, 2.5769803776e+10, 1.2884901888e+10, 6.442450944e+09,
		// 					3.221225472e+09, 1.610612736e+09, 8.05306368e+08, 4.02653184e+08, 2.01326592e+08, 1.00663296e+08, 5.0331648e+07,
		// 					2.5165824e+07, 1.2582912e+07, 6.291456e+06, 3.145728e+06, 1.572864e+06, 786432, 393216, 196608, 98304, 49152, 24576, 12288,
		// 					6144, 3072, 1536, 768, 384, 192, 96, 48, 24, 12, 6, 3, 1.5,
		// 				},
		// 				Counts: []float64{
		// 					100, 99, 98, 97, 96, 95, 94,
		// 					93, 92, 91, 90, 89, 88, 87, 86, 85, 84, 83, 82, 81, 80, 79, 78, 77, 76, 75, 74, 73, 72, 71, 70, 69, 68, 67, 66, 65, 64, 63, 62, 61,
		// 					60, 59, 58, 57, 56, 55, 54, 53, 52, 51, 50, 49, 48, 47, 46, 45, 44, 43, 42, 41, 40, 39, 38, 37, 36, 35, 34, 33, 32, 31, 30, 29, 28,
		// 					27, 26, 25, 24, 23, 22, 21, 20, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1,
		// 				},
		// 				Sum: 100000, Count: 5050, Min: 1, Max: 9e+36,
		// 			},
		// 			labels: map[string]string{"label1": "value1"},
		// 		},
		// 		{
		// 			name: "foo",
		// 			value: &cWMetricHistogram{
		// 				Values: []float64{
		// 					-1.5, -3, -6, -12, -24, -48, -96, -192, -384, -768, -1536, -3072, -6144, -12288, -24576, -49152, -98304, -196608, -393216,
		// 					-786432, -1.572864e+06, -3.145728e+06, -6.291456e+06, -1.2582912e+07, -2.5165824e+07, -5.0331648e+07, -1.00663296e+08,
		// 					-2.01326592e+08, -4.02653184e+08, -8.05306368e+08, -1.610612736e+09, -3.221225472e+09, -6.442450944e+09, -1.2884901888e+10,
		// 					-2.5769803776e+10, -5.1539607552e+10, -1.03079215104e+11, -2.06158430208e+11, -4.12316860416e+11, -8.24633720832e+11,
		// 					-1.649267441664e+12, -3.298534883328e+12, -6.597069766656e+12, -1.3194139533312e+13, -2.6388279066624e+13, -5.2776558133248e+13,
		// 					-1.05553116266496e+14, -2.11106232532992e+14, -4.22212465065984e+14, -8.44424930131968e+14, -1.688849860263936e+15,
		// 					-3.377699720527872e+15, -6.755399441055744e+15, -1.3510798882111488e+16, -2.7021597764222976e+16,
		// 					-5.404319552844595e+16, -1.080863910568919e+17, -2.161727821137838e+17, -4.323455642275676e+17, -8.646911284551352e+17,
		// 					-1.7293822569102705e+18, -3.458764513820541e+18, -6.917529027641082e+18, -1.3835058055282164e+19, -2.7670116110564327e+19,
		// 					-5.5340232221128655e+19, -1.1068046444225731e+20, -2.2136092888451462e+20, -4.4272185776902924e+20, -8.854437155380585e+20,
		// 					-1.770887431076117e+21, -3.541774862152234e+21, -7.083549724304468e+21, -1.4167099448608936e+22, -2.833419889721787e+22,
		// 					-5.666839779443574e+22, -1.1333679558887149e+23, -2.2667359117774297e+23, -4.5334718235548594e+23, -9.066943647109719e+23,
		// 					-1.8133887294219438e+24, -3.6267774588438875e+24, -7.253554917687775e+24, -1.450710983537555e+25, -2.90142196707511e+25,
		// 					-5.80284393415022e+25, -1.160568786830044e+26, -2.321137573660088e+26, -4.642275147320176e+26, -9.284550294640352e+26,
		// 					-1.8569100589280704e+27, -3.713820117856141e+27, -7.427640235712282e+27, -1.4855280471424563e+28, -2.9710560942849127e+28,
		// 					-5.942112188569825e+28, -1.188422437713965e+29, -2.37684487542793e+29, -4.75368975085586e+29, -9.50737950171172e+29,
		// 				},
		// 				Counts: []float64{
		// 					1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35,
		// 					36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63, 64, 65, 66, 67, 68,
		// 					69, 70, 71, 72, 73, 74, 75, 76, 77, 78, 79, 80, 81, 82, 83, 84, 85, 86, 87, 88, 89, 90, 91, 92, 93, 94, 95, 96, 97, 98, 99, 100,
		// 				},
		// 				Sum: 0, Count: 5050, Min: -9e+36, Max: -1,
		// 			},
		// 			labels: map[string]string{"label1": "value1"},
		// 		},
		// 	},
		// },
		// {
		// 	name: "Exponential histogram with more than 200 buckets, including positive, negative and zero buckets",
		// 	histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
		// 		histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
		// 		histogramDP := histogramDPS.AppendEmpty()
		// 		posBucketCounts := make([]uint64, 120)
		// 		for i := range posBucketCounts {
		// 			posBucketCounts[i] = uint64(i + 1)
		// 		}
		// 		histogramDP.Positive().BucketCounts().FromRaw(posBucketCounts)
		// 		histogramDP.SetZeroCount(2)
		// 		negBucketCounts := make([]uint64, 120)
		// 		for i := range negBucketCounts {
		// 			negBucketCounts[i] = uint64(i + 1)
		// 		}
		// 		histogramDP.Negative().BucketCounts().FromRaw(negBucketCounts)
		// 		histogramDP.SetSum(100000)
		// 		histogramDP.SetMin(-9e+36)
		// 		histogramDP.SetMax(9e+36)
		// 		histogramDP.SetCount(uint64(3662))
		// 		histogramDP.Attributes().PutStr("label1", "value1")
		// 		return histogramDPS
		// 	}(),
		// 	expectedDatapoints: []dataPoint{
		// 		{
		// 			name: "foo",
		// 			value: &cWMetricHistogram{
		// 				Values: []float64{
		// 					9.969209968386869e+35, 4.9846049841934345e+35, 2.4923024920967173e+35, 1.2461512460483586e+35, 6.230756230241793e+34,
		// 					3.1153781151208966e+34, 1.5576890575604483e+34, 7.788445287802241e+33, 3.894222643901121e+33, 1.9471113219505604e+33,
		// 					9.735556609752802e+32, 4.867778304876401e+32, 2.4338891524382005e+32, 1.2169445762191002e+32, 6.084722881095501e+31,
		// 					3.0423614405477506e+31, 1.5211807202738753e+31, 7.605903601369376e+30, 3.802951800684688e+30, 1.901475900342344e+30,
		// 					9.50737950171172e+29, 4.75368975085586e+29, 2.37684487542793e+29, 1.188422437713965e+29, 5.942112188569825e+28,
		// 					2.9710560942849127e+28, 1.4855280471424563e+28, 7.427640235712282e+27, 3.713820117856141e+27, 1.8569100589280704e+27,
		// 					9.284550294640352e+26, 4.642275147320176e+26, 2.321137573660088e+26, 1.160568786830044e+26, 5.80284393415022e+25,
		// 					2.90142196707511e+25, 1.450710983537555e+25, 7.253554917687775e+24, 3.6267774588438875e+24, 1.8133887294219438e+24,
		// 					9.066943647109719e+23, 4.5334718235548594e+23, 2.2667359117774297e+23, 1.1333679558887149e+23, 5.666839779443574e+22,
		// 					2.833419889721787e+22, 1.4167099448608936e+22, 7.083549724304468e+21, 3.541774862152234e+21, 1.770887431076117e+21,
		// 					8.854437155380585e+20, 4.4272185776902924e+20, 2.2136092888451462e+20, 1.1068046444225731e+20, 5.5340232221128655e+19,
		// 					2.7670116110564327e+19, 1.3835058055282164e+19, 6.917529027641082e+18, 3.458764513820541e+18, 1.7293822569102705e+18,
		// 					8.646911284551352e+17, 4.323455642275676e+17, 2.161727821137838e+17, 1.080863910568919e+17, 5.404319552844595e+16,
		// 					2.7021597764222976e+16, 1.3510798882111488e+16, 6.755399441055744e+15, 3.377699720527872e+15, 1.688849860263936e+15,
		// 					8.44424930131968e+14, 4.22212465065984e+14, 2.11106232532992e+14, 1.05553116266496e+14, 5.2776558133248e+13,
		// 					2.6388279066624e+13, 1.3194139533312e+13, 6.597069766656e+12, 3.298534883328e+12, 1.649267441664e+12, 8.24633720832e+11,
		// 					4.12316860416e+11, 2.06158430208e+11, 1.03079215104e+11, 5.1539607552e+10, 2.5769803776e+10, 1.2884901888e+10,
		// 					6.442450944e+09, 3.221225472e+09, 1.610612736e+09, 8.05306368e+08, 4.02653184e+08, 2.01326592e+08, 1.00663296e+08, 5.0331648e+07,
		// 					2.5165824e+07, 1.2582912e+07, 6.291456e+06, 3.145728e+06, 1.572864e+06,
		// 				},
		// 				Counts: []float64{
		// 					120, 119, 118, 117, 116, 115, 114, 113, 112, 111, 110, 109, 108, 107, 106, 105, 104, 103, 102, 101, 100, 99, 98, 97, 96, 95, 94,
		// 					93, 92, 91, 90, 89, 88, 87, 86, 85, 84, 83, 82, 81, 80, 79, 78, 77, 76, 75, 74, 73, 72, 71, 70, 69, 68, 67, 66, 65, 64, 63, 62, 61,
		// 					60, 59, 58, 57, 56, 55, 54, 53, 52, 51, 50, 49, 48, 47, 46, 45, 44, 43, 42, 41, 40, 39, 38, 37, 36, 35, 34, 33, 32, 31, 30, 29, 28,
		// 					27, 26, 25, 24, 23, 22, 21,
		// 				},
		// 				Sum: 100000, Count: 7050, Min: 1048576, Max: 9e+36,
		// 			},
		// 			labels: map[string]string{"label1": "value1"},
		// 		},
		// 		{
		// 			name: "foo",
		// 			value: &cWMetricHistogram{
		// 				Values: []float64{
		// 					786432, 393216, 196608, 98304, 49152, 24576, 12288, 6144, 3072, 1536, 768, 384, 192, 96, 48, 24,
		// 					12, 6, 3, 1.5, 0, -1.5, -3, -6, -12, -24, -48, -96, -192, -384, -768, -1536,
		// 					-3072, -6144, -12288, -24576, -49152, -98304, -196608, -393216, -786432, -1.572864e+06, -3.145728e+06, -6.291456e+06,
		// 					-1.2582912e+07, -2.5165824e+07, -5.0331648e+07, -1.00663296e+08, -2.01326592e+08, -4.02653184e+08, -8.05306368e+08,
		// 					-1.610612736e+09, -3.221225472e+09, -6.442450944e+09, -1.2884901888e+10, -2.5769803776e+10, -5.1539607552e+10,
		// 					-1.03079215104e+11, -2.06158430208e+11, -4.12316860416e+11, -8.24633720832e+11, -1.649267441664e+12,
		// 					-3.298534883328e+12, -6.597069766656e+12, -1.3194139533312e+13, -2.6388279066624e+13, -5.2776558133248e+13,
		// 					-1.05553116266496e+14, -2.11106232532992e+14, -4.22212465065984e+14, -8.44424930131968e+14,
		// 					-1.688849860263936e+15, -3.377699720527872e+15, -6.755399441055744e+15, -1.3510798882111488e+16,
		// 					-2.7021597764222976e+16, -5.404319552844595e+16, -1.080863910568919e+17, -2.161727821137838e+17,
		// 					-4.323455642275676e+17, -8.646911284551352e+17, -1.7293822569102705e+18, -3.458764513820541e+18,
		// 					-6.917529027641082e+18, -1.3835058055282164e+19, -2.7670116110564327e+19, -5.5340232221128655e+19,
		// 					-1.1068046444225731e+20, -2.2136092888451462e+20, -4.4272185776902924e+20, -8.854437155380585e+20,
		// 					-1.770887431076117e+21, -3.541774862152234e+21, -7.083549724304468e+21, -1.4167099448608936e+22,
		// 					-2.833419889721787e+22, -5.666839779443574e+22, -1.1333679558887149e+23, -2.2667359117774297e+23,
		// 					-4.5334718235548594e+23,
		// 				},
		// 				Counts: []float64{
		// 					20, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 2, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10,
		// 					11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36,
		// 					37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62,
		// 					63, 64, 65, 66, 67, 68, 69, 70, 71, 72, 73, 74, 75, 76, 77, 78, 79,
		// 				},
		// 				Sum: 0, Count: 3372, Min: -6.044629098073146e+23, Max: 1048576,
		// 			},
		// 			labels: map[string]string{"label1": "value1"},
		// 		},
		// 		{
		// 			name: "foo",
		// 			value: &cWMetricHistogram{
		// 				Values: []float64{
		// 					-9.066943647109719e+23, -1.8133887294219438e+24, -3.6267774588438875e+24, -7.253554917687775e+24, -1.450710983537555e+25,
		// 					-2.90142196707511e+25, -5.80284393415022e+25, -1.160568786830044e+26, -2.321137573660088e+26, -4.642275147320176e+26,
		// 					-9.284550294640352e+26, -1.8569100589280704e+27, -3.713820117856141e+27, -7.427640235712282e+27, -1.4855280471424563e+28,
		// 					-2.9710560942849127e+28, -5.942112188569825e+28, -1.188422437713965e+29, -2.37684487542793e+29, -4.75368975085586e+29,
		// 					-9.50737950171172e+29, -1.901475900342344e+30, -3.802951800684688e+30, -7.605903601369376e+30, -1.5211807202738753e+31,
		// 					-3.0423614405477506e+31, -6.084722881095501e+31, -1.2169445762191002e+32, -2.4338891524382005e+32, -4.867778304876401e+32,
		// 					-9.735556609752802e+32, -1.9471113219505604e+33, -3.894222643901121e+33, -7.788445287802241e+33, -1.5576890575604483e+34,
		// 					-3.1153781151208966e+34, -6.230756230241793e+34, -1.2461512460483586e+35, -2.4923024920967173e+35, -4.9846049841934345e+35,
		// 					-9.969209968386869e+35,
		// 				},
		// 				Counts: []float64{
		// 					80, 81, 82, 83, 84, 85, 86, 87, 88, 89, 90, 91, 92, 93, 94, 95, 96, 97, 98, 99, 100, 101, 102, 103, 104, 105, 106, 107, 108, 109,
		// 					110, 111, 112, 113, 114, 115, 116, 117, 118, 119, 120,
		// 				},
		// 				Sum: 0, Count: 4100, Min: -9e+36, Max: -6.044629098073146e+23,
		// 			},
		// 			labels: map[string]string{"label1": "value1"},
		// 		},
		// 	},
		// },
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(_ *testing.T) {
			dps := ConvertOtelExponentialHistogramDataPoint(tc.histogram, "foo", "none", 1, entity)

			//fmt.Println(dps)
			assert.Equal(t, len(tc.expected), len(dps))

			// special rules with StatisticValues
			if len(dps) > 0 {
				first := dps[0]
				last := dps[len(dps)-1]

				assert.Equal(t, tc.histogram.Max(), *first.StatisticValues.Maximum, "first entry maximum should match the maximum in the histogram metric")
				assert.Equal(t, tc.histogram.Sum(), *first.StatisticValues.Sum, "first entry sum should match the sum in the histogram metric")
				assert.Equal(t, tc.histogram.Min(), *last.StatisticValues.Minimum, "last entry minimum should match the minimum in the histogram metric")

				for i := 1; i < len(dps); i++ {
					assert.Equal(t, float64(0), *dps[i].StatisticValues.Sum, "only first entry should have a non-zero sum")
				}
			}

			for i, expectedDP := range tc.expected {
				assert.Equal(t, len(expectedDP.Values), len(dps[i].Values))
				assert.Equal(t, len(expectedDP.Counts), len(dps[i].Counts))
				assert.Equal(t, expectedDP, dps[i], "datapoint mismatch at index %d", i)
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
