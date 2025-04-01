// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatch

import (
	"log"
	"math"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	cloudwatchutil "github.com/aws/amazon-cloudwatch-agent/internal/cloudwatch"
	"github.com/aws/amazon-cloudwatch-agent/metric/distribution"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsentity/entityattributes"
	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatch"
)

// ConvertOtelDimensions will returns a sorted list of dimensions.
func ConvertOtelDimensions(attributes pcommon.Map) []*cloudwatch.Dimension {
	// Loop through map, similar to EMF exporter createLabels().
	mTags := make(map[string]string, attributes.Len())
	attributes.Range(func(k string, v pcommon.Value) bool {
		// we don't want to export entity related attributes as dimensions, so we skip these
		if strings.HasPrefix(k, entityattributes.AWSEntityPrefix) {
			return true
		}
		mTags[k] = v.AsString()
		return true
	})
	return BuildDimensions(mTags)
}

// NumberDataPointValue converts to float64 since that is what AWS SDK will use.
func NumberDataPointValue(dp pmetric.NumberDataPoint) float64 {
	switch dp.ValueType() {
	case pmetric.NumberDataPointValueTypeDouble:
		return dp.DoubleValue()
	case pmetric.NumberDataPointValueTypeInt:
		return float64(dp.IntValue())
	}
	return 0
}

// checkHighResolution removes the special attribute.
// Return 1 if it was present and set to "true". Else return 60.
func checkHighResolution(attributes *pcommon.Map) int64 {
	var resolution int64 = 60
	v, ok := attributes.Get(highResolutionTagKey)
	if ok {
		if strings.EqualFold(v.AsString(), "true") {
			resolution = 1
		}
		attributes.Remove(highResolutionTagKey)
	}
	return resolution
}

// getAggregationInterval removes this special dimension and returns its value.
func getAggregationInterval(attributes *pcommon.Map) time.Duration {
	var interval time.Duration
	v, ok := attributes.Get(aggregationIntervalTagKey)
	if !ok {
		return interval
	}
	s := v.AsString()
	interval, err := time.ParseDuration(s)
	if err != nil {
		log.Printf("W! cannot parse aggregation interval, %s", s)
	}
	attributes.Remove(aggregationIntervalTagKey)
	return interval
}

// ConvertOtelNumberDataPoints converts each datapoint in the given slice to
// 1 or more MetricDatums and returns them.
func ConvertOtelNumberDataPoints(
	dataPoints pmetric.NumberDataPointSlice,
	name string,
	unit string,
	scale float64,
	entity cloudwatch.Entity,
) []*aggregationDatum {
	// Could make() with attrs.Len() * len(c.RollupDimensions).
	datums := make([]*aggregationDatum, 0, dataPoints.Len())
	for i := 0; i < dataPoints.Len(); i++ {
		dp := dataPoints.At(i)
		attrs := dp.Attributes()
		storageResolution := checkHighResolution(&attrs)
		aggregationInterval := getAggregationInterval(&attrs)
		dimensions := ConvertOtelDimensions(attrs)
		value := NumberDataPointValue(dp) * scale
		ad := aggregationDatum{
			MetricDatum: cloudwatch.MetricDatum{
				Dimensions:        dimensions,
				MetricName:        aws.String(name),
				Unit:              aws.String(unit),
				Timestamp:         aws.Time(dp.Timestamp().AsTime()),
				Value:             aws.Float64(value),
				StorageResolution: aws.Int64(storageResolution),
			},
			aggregationInterval: aggregationInterval,
			entity:              entity,
		}
		datums = append(datums, &ad)
	}
	return datums
}

// ConvertOtelHistogramDataPoints converts each datapoint in the given slice to
// Distribution.
func ConvertOtelHistogramDataPoints(
	dataPoints pmetric.HistogramDataPointSlice,
	name string,
	unit string,
	scale float64,
	entity cloudwatch.Entity,
) []*aggregationDatum {
	datums := make([]*aggregationDatum, 0, dataPoints.Len())
	for i := 0; i < dataPoints.Len(); i++ {
		dp := dataPoints.At(i)
		attrs := dp.Attributes()
		storageResolution := checkHighResolution(&attrs)
		aggregationInterval := getAggregationInterval(&attrs)
		dimensions := ConvertOtelDimensions(attrs)
		ad := aggregationDatum{
			MetricDatum: cloudwatch.MetricDatum{
				Dimensions:        dimensions,
				MetricName:        aws.String(name),
				Unit:              aws.String(unit),
				Timestamp:         aws.Time(dp.Timestamp().AsTime()),
				StorageResolution: aws.Int64(storageResolution),
			},
			aggregationInterval: aggregationInterval,
			entity:              entity,
		}
		// Assume function pointer is valid.
		ad.distribution = distribution.NewDistribution()
		ad.distribution.ConvertFromOtel(dp, unit)
		datums = append(datums, &ad)
	}
	return datums
}

// ConvertOtelHistogramDataPoints converts each datapoint in the given slice to
// Distribution.
func ConvertOtelExponentialHistogramDataPoints(
	dataPoints pmetric.ExponentialHistogramDataPointSlice,
	name string,
	unit string,
	scale float64,
	entity cloudwatch.Entity,
) []*aggregationDatum {
	datums := make([]*aggregationDatum, 0, dataPoints.Len())
	for i := 0; i < dataPoints.Len(); i++ {
		dp := dataPoints.At(i)
		datums = append(datums, ConvertOtelExponentialHistogramDataPoint(dp, name, unit, scale, entity)...)
	}
	return datums
}

func ConvertOtelExponentialHistogramDataPoint(
	metric pmetric.ExponentialHistogramDataPoint,
	name string,
	unit string,
	scale float64,
	entity cloudwatch.Entity,
) []*aggregationDatum {

	attrs := metric.Attributes()
	storageResolution := checkHighResolution(&attrs)
	aggregationInterval := getAggregationInterval(&attrs)
	dimensions := ConvertOtelDimensions(attrs)

	const splitThreshold = defaultMaxValuesPerDatum // TODO: source from export config c.config.MaxValuesPerDatum
	currentBucketIndex := 0
	currentPositiveIndex := metric.Positive().BucketCounts().Len() - 1
	currentZeroIndex := 0
	currentNegativeIndex := 0
	datapoints := []*aggregationDatum{}
	totalBucketLen := metric.Positive().BucketCounts().Len() + metric.Negative().BucketCounts().Len()
	if metric.ZeroCount() > 0 {
		totalBucketLen++
	}

	for currentBucketIndex < totalBucketLen {
		// Create a new dataPointSplit with a capacity of up to splitThreshold buckets
		capacity := splitThreshold
		if totalBucketLen-currentBucketIndex < splitThreshold {
			capacity = totalBucketLen - currentBucketIndex
		}

		sum := 0.0
		// Only assign `Sum` if this is the first split to make sure the total sum of the datapoints after aggregation is correct.
		if currentBucketIndex == 0 {
			sum = metric.Sum()
		}

		split := dataPointSplit{
			cWMetricHistogram: &cWMetricHistogram{
				Values: []float64{},
				Counts: []float64{},
				Max:    metric.Max(),
				Min:    metric.Min(),
				Count:  0,
				Sum:    sum,
			},
			length:   0,
			capacity: capacity,
		}

		// Set collect values from positive buckets and save into split.
		currentBucketIndex, currentPositiveIndex = collectDatapointsWithPositiveBuckets(&split, metric, currentBucketIndex, currentPositiveIndex)
		// Set collect values from zero buckets and save into split.
		currentBucketIndex, currentZeroIndex = collectDatapointsWithZeroBucket(&split, metric, currentBucketIndex, currentZeroIndex)
		// Set collect values from negative buckets and save into split.
		currentBucketIndex, currentNegativeIndex = collectDatapointsWithNegativeBuckets(&split, metric, currentBucketIndex, currentNegativeIndex)

		if split.length > 0 {
			// Add the current split to the datapoints list
			datapoints = append(datapoints, &aggregationDatum{
				MetricDatum: cloudwatch.MetricDatum{
					Dimensions:        dimensions,
					MetricName:        aws.String(name),
					Unit:              aws.String(unit),
					Timestamp:         aws.Time(metric.Timestamp().AsTime()),
					StorageResolution: aws.Int64(storageResolution),
					StatisticValues: &cloudwatch.StatisticSet{
						// Min and Max values are recalculated based on the bucket boundary within that specific split.
						Maximum: aws.Float64(split.cWMetricHistogram.Max),
						Minimum: aws.Float64(split.cWMetricHistogram.Min),
						// Count is accumulated based on the bucket counts within each split.
						SampleCount: aws.Float64(float64(split.cWMetricHistogram.Count)),
						// Sum is only assigned to the first split to ensure the total sum of the datapoints after aggregation is correct.
						Sum: aws.Float64(split.cWMetricHistogram.Sum),
					},
					Values: aws.Float64Slice(split.cWMetricHistogram.Values),
					Counts: aws.Float64Slice(split.cWMetricHistogram.Counts),
				},
				aggregationInterval: aggregationInterval,
				entity:              entity,
			})
		}
	}

	if len(datapoints) == 0 {
		return []*aggregationDatum{{
			MetricDatum: cloudwatch.MetricDatum{
				Dimensions:        dimensions,
				MetricName:        aws.String(name),
				Unit:              aws.String(unit),
				Timestamp:         aws.Time(metric.Timestamp().AsTime()),
				StorageResolution: aws.Int64(storageResolution),
				StatisticValues: &cloudwatch.StatisticSet{
					SampleCount: aws.Float64(float64(metric.Count())),
					Sum:         aws.Float64(metric.Sum()),
					Maximum:     aws.Float64(metric.Max()),
					Minimum:     aws.Float64(metric.Min()),
				},
			},
			aggregationInterval: aggregationInterval,
			entity:              entity,
		}}
	}

	// Override the min and max values of the first and last splits with the raw data of the metric.
	// The datapoint entries are collected in descending order. The first datapoint contains the largest values and therefore
	// the maximum of the first datapoint should be the same as the maximum of the metric. The last datapoint contains the
	// smallest values and therefore the minimum of the last datapoint should be the same as the minimum of the metric.
	datapoints[0].StatisticValues.SetMaximum(metric.Max())
	datapoints[len(datapoints)-1].StatisticValues.SetMinimum(metric.Min())

	return datapoints
}

func collectDatapointsWithPositiveBuckets(split *dataPointSplit, metric pmetric.ExponentialHistogramDataPoint, currentBucketIndex int, currentPositiveIndex int) (int, int) {
	if !split.isNotFull() || currentPositiveIndex < 0 {
		return currentBucketIndex, currentPositiveIndex
	}

	scale := metric.Scale()
	base := math.Pow(2, math.Pow(2, float64(-scale)))
	positiveBuckets := metric.Positive()
	positiveOffset := positiveBuckets.Offset()
	positiveBucketCounts := positiveBuckets.BucketCounts()
	bucketBegin := 0.0
	bucketEnd := 0.0

	for split.isNotFull() && currentPositiveIndex >= 0 {
		index := currentPositiveIndex + int(positiveOffset)
		if bucketEnd == 0 {
			bucketEnd = math.Pow(base, float64(index+1))
		} else {
			bucketEnd = bucketBegin
		}
		bucketBegin = math.Pow(base, float64(index))
		metricVal := (bucketBegin + bucketEnd) / 2
		count := positiveBucketCounts.At(currentPositiveIndex)
		if count > 0 {
			split.appendMetricData(metricVal, count)

			// The value are append from high to low, set Max from the first bucket (highest value) and Min from the last bucket (lowest value)
			if split.length == 1 {
				split.setMax(bucketEnd)
			}
			if !split.isNotFull() {
				split.setMin(bucketBegin)
			}
		}
		currentBucketIndex++
		currentPositiveIndex--
	}

	return currentBucketIndex, currentPositiveIndex
}

func collectDatapointsWithZeroBucket(split *dataPointSplit, metric pmetric.ExponentialHistogramDataPoint, currentBucketIndex int, currentZeroIndex int) (int, int) {
	if metric.ZeroCount() > 0 && split.isNotFull() && currentZeroIndex == 0 {
		split.appendMetricData(0, metric.ZeroCount())

		// The value are append from high to low, set Max from the first bucket (highest value) and Min from the last bucket (lowest value)
		if split.length == 1 {
			split.setMax(0)
		}
		if !split.isNotFull() {
			split.setMin(0)
		}
		currentZeroIndex++
		currentBucketIndex++
	}

	return currentBucketIndex, currentZeroIndex
}

func collectDatapointsWithNegativeBuckets(split *dataPointSplit, metric pmetric.ExponentialHistogramDataPoint, currentBucketIndex int, currentNegativeIndex int) (int, int) {
	// According to metrics spec, the value in histogram is expected to be non-negative.
	// https://opentelemetry.io/docs/specs/otel/metrics/api/#histogram
	// However, the negative support is defined in metrics data model.
	// https://opentelemetry.io/docs/specs/otel/metrics/data-model/#exponentialhistogram
	// The negative is also supported but only verified with unit test.
	if !split.isNotFull() || currentNegativeIndex >= metric.Negative().BucketCounts().Len() {
		return currentBucketIndex, currentNegativeIndex
	}

	scale := metric.Scale()
	base := math.Pow(2, math.Pow(2, float64(-scale)))
	negativeBuckets := metric.Negative()
	negativeOffset := negativeBuckets.Offset()
	negativeBucketCounts := negativeBuckets.BucketCounts()
	bucketBegin := 0.0
	bucketEnd := 0.0

	for split.isNotFull() && currentNegativeIndex < metric.Negative().BucketCounts().Len() {
		index := currentNegativeIndex + int(negativeOffset)
		if bucketEnd == 0 {
			bucketEnd = -math.Pow(base, float64(index))
		} else {
			bucketEnd = bucketBegin
		}
		bucketBegin = -math.Pow(base, float64(index+1))
		metricVal := (bucketBegin + bucketEnd) / 2
		count := negativeBucketCounts.At(currentNegativeIndex)
		if count > 0 {
			split.appendMetricData(metricVal, count)

			// The value are append from high to low, set Max from the first bucket (highest value) and Min from the last bucket (lowest value)
			if split.length == 1 {
				split.setMax(bucketEnd)
			}
			if !split.isNotFull() {
				split.setMin(bucketBegin)
			}
		}
		currentBucketIndex++
		currentNegativeIndex++
	}

	return currentBucketIndex, currentNegativeIndex
}

// The SampleCount of CloudWatch metrics will be calculated by the sum of the 'Counts' array.
// The 'Count' field should be same as the sum of the 'Counts' array and will be ignored in CloudWatch.
type cWMetricHistogram struct {
	Values []float64
	Counts []float64
	Max    float64
	Min    float64
	Count  uint64
	Sum    float64
}

type dataPointSplit struct {
	cWMetricHistogram *cWMetricHistogram
	length            int
	capacity          int
}

func (split *dataPointSplit) isNotFull() bool {
	return split.length < split.capacity
}

func (split *dataPointSplit) setMax(maxVal float64) {
	split.cWMetricHistogram.Max = maxVal
}

func (split *dataPointSplit) setMin(minVal float64) {
	split.cWMetricHistogram.Min = minVal
}

func (split *dataPointSplit) appendMetricData(metricVal float64, count uint64) {
	split.cWMetricHistogram.Values = append(split.cWMetricHistogram.Values, metricVal)
	split.cWMetricHistogram.Counts = append(split.cWMetricHistogram.Counts, float64(count))
	split.length++
	split.cWMetricHistogram.Count += count
}

// ConvertOtelMetric creates a list of datums from the datapoints in the given
// metric and returns it. Only supports the metric DataTypes that we plan to use.
// Intentionally not caching previous values and converting cumulative to delta.
// Instead use cumulativetodeltaprocessor which supports monotonic cumulative sums.
func ConvertOtelMetric(m pmetric.Metric, entity cloudwatch.Entity) []*aggregationDatum {
	name := m.Name()
	unit, scale, err := cloudwatchutil.ToStandardUnit(m.Unit())
	if err != nil {
		log.Printf("W! cloudwatch: metricname %q has %v", name, err)
	}
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		return ConvertOtelNumberDataPoints(m.Gauge().DataPoints(), name, unit, scale, entity)
	case pmetric.MetricTypeSum:
		return ConvertOtelNumberDataPoints(m.Sum().DataPoints(), name, unit, scale, entity)
	case pmetric.MetricTypeHistogram:
		return ConvertOtelHistogramDataPoints(m.Histogram().DataPoints(), name, unit, scale, entity)
	case pmetric.MetricTypeExponentialHistogram:
		return ConvertOtelExponentialHistogramDataPoints(m.ExponentialHistogram().DataPoints(), name, unit, scale, entity)
	default:
		log.Printf("E! cloudwatch: Unsupported type, %s", m.Type())
	}
	return []*aggregationDatum{}
}

func ConvertOtelMetrics(m pmetric.Metrics) []*aggregationDatum {
	datums := make([]*aggregationDatum, 0, m.DataPointCount())
	for i := 0; i < m.ResourceMetrics().Len(); i++ {
		entity := entityattributes.CreateCloudWatchEntityFromAttributes(m.ResourceMetrics().At(i).Resource().Attributes())
		scopeMetrics := m.ResourceMetrics().At(i).ScopeMetrics()
		for j := 0; j < scopeMetrics.Len(); j++ {
			metrics := scopeMetrics.At(j).Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				newDatums := ConvertOtelMetric(metric, entity)
				datums = append(datums, newDatums...)

			}
		}
	}
	return datums
}
