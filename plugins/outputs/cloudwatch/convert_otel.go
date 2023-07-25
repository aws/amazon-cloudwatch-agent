// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatch

import (
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	cloudwatchutil "github.com/aws/amazon-cloudwatch-agent/internal/cloudwatch"
	"github.com/aws/amazon-cloudwatch-agent/metric/distribution"
)

// ConvertOtelDimensions will returns a sorted list of dimensions.
func ConvertOtelDimensions(attributes pcommon.Map) []*cloudwatch.Dimension {
	// Loop through map, similar to EMF exporter createLabels().
	mTags := make(map[string]string, attributes.Len())
	attributes.Range(func(k string, v pcommon.Value) bool {
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
		}
		// Assume function pointer is valid.
		ad.distribution = distribution.NewDistribution()
		ad.distribution.ConvertFromOtel(dp, unit)
		datums = append(datums, &ad)
	}
	return datums
}

// ConvertOtelMetric creates a list of datums from the datapoints in the given
// metric and returns it. Only supports the metric DataTypes that we plan to use.
// Intentionally not caching previous values and converting cumulative to delta.
// Instead use cumulativetodeltaprocessor which supports monotonic cumulative sums.
func ConvertOtelMetric(m pmetric.Metric) []*aggregationDatum {
	name := m.Name()
	unit, scale, err := cloudwatchutil.ToStandardUnit(m.Unit())
	if err != nil {
		log.Printf("W! cloudwatch: metricname %q has %v", name, err)
	}
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		return ConvertOtelNumberDataPoints(m.Gauge().DataPoints(), name, unit, scale)
	case pmetric.MetricTypeSum:
		return ConvertOtelNumberDataPoints(m.Sum().DataPoints(), name, unit, scale)
	case pmetric.MetricTypeHistogram:
		return ConvertOtelHistogramDataPoints(m.Histogram().DataPoints(), name, unit, scale)
	default:
		log.Printf("E! cloudwatch: Unsupported type, %s", m.Type())
	}
	return []*aggregationDatum{}
}

// ConvertOtelMetrics only uses dimensions/attributes on each "datapoint",
// not each "Resource".
// This is acceptable because ResourceToTelemetrySettings defaults to true.
func ConvertOtelMetrics(m pmetric.Metrics) []*aggregationDatum {
	datums := make([]*aggregationDatum, 0, m.DataPointCount())
	// Metrics -> ResourceMetrics -> ScopeMetrics -> MetricSlice -> DataPoints
	resourceMetrics := m.ResourceMetrics()
	for i := 0; i < resourceMetrics.Len(); i++ {
		scopeMetrics := resourceMetrics.At(i).ScopeMetrics()
		for j := 0; j < scopeMetrics.Len(); j++ {
			metrics := scopeMetrics.At(j).Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				newDatums := ConvertOtelMetric(metric)
				datums = append(datums, newDatums...)
			}
		}
	}
	return datums
}
