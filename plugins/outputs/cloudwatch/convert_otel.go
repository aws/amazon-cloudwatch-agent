// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatch

import (
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// ConvertOtelDimensions will return a list of lists. Without dimension
// rollup there will be just 1 list containing 1 list of all the dimensions.
// With dimension rollup there will be 1 list containing many lists.
func (c *CloudWatch) ConvertOtelDimensions(
	attributes pcommon.Map,
) [][]*cloudwatch.Dimension {
	// Loop through map, similar to EMF exporter createLabels().
	mTags := make(map[string]string, attributes.Len())
	attributes.Range(func(k string, v pcommon.Value) bool {
		mTags[k] = v.AsString()
		return true
	})
	dimensions := BuildDimensions(mTags)
	return c.ProcessRollup(dimensions)
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

// checkHighResolution removes the special dimension/tag/attribute.
// Return true if it was present and set to "true".
// This may change depending on how receivers are implemented.
// Is there a better way to pass the CollectionInterval of each metric
// to ths exporter? For now, do it like it was done for Telegraf metrics.
func checkHighResolution(attributes pcommon.Map) bool {
	r := false
	v, ok := attributes.Get(highResolutionTagKey)
	if ok {
		if strings.EqualFold(v.AsString(), "true") {
			r = true
		}
		attributes.Remove(highResolutionTagKey)
	}
	return r
}

// ConvertOtelNumberDataPoints converts each datapoint in the given slice to
// 1 or more MetricDatums and returns them.
func (c *CloudWatch) ConvertOtelNumberDataPoints(
	dps pmetric.NumberDataPointSlice,
	name string,
	unit string,
) []*cloudwatch.MetricDatum {
	// Could make() with attrs.Len() * len(c.RollupDimensions).
	datums := make([]*cloudwatch.MetricDatum, 0, dps.Len())
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		attrs := dp.Attributes()
		isHighResolution := checkHighResolution(attrs)
		rolledDims := c.ConvertOtelDimensions(attrs)
		value := NumberDataPointValue(dp)
		// Each datapoint may become many datums due to dimension roll up.
		for _, dims := range rolledDims {
			// todo: IsDropping()
			md := cloudwatch.MetricDatum{
				Dimensions: dims,
				MetricName: aws.String(name),
				Unit:       aws.String(unit),
				Timestamp:  aws.Time(dp.Timestamp().AsTime()),
				Value:      aws.Float64(value),
			}
			if isHighResolution {
				md.SetStorageResolution(1)
			}
			datums = append(datums, &md)
		}
	}
	return datums
}

// ConvertOtelMetric creates a list of datums from the datapoints in the given
// metric and returns it. Only supports the metric DataTypes that we plan to use.
// Intentionally not caching previous values and converting cumulative to delta.
// Instead use cumulativetodeltaprocessor which supports monotonic cumulative sums.
func (c *CloudWatch) ConvertOtelMetric(m pmetric.Metric) []*cloudwatch.MetricDatum {
	n := m.Name()
	u, err := ConvertUnit(m.Unit())
	if err != nil {
		log.Printf("W! cloudwatch: metricname %q has %v", n, err)
	}
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		return c.ConvertOtelNumberDataPoints(m.Gauge().DataPoints(), n, u)
	case pmetric.MetricTypeSum:
		return c.ConvertOtelNumberDataPoints(m.Sum().DataPoints(), n, u)
	default:
		log.Printf("E! cloudwatch: Unsupported type, %s", m.Type())
	}
	return []*cloudwatch.MetricDatum{}
}

// ConvertOtelMetrics only uses dimensions/attributes on each "datapoint",
// not each "Resource".
// This is acceptable because ResourceToTelemetrySettings defaults to true.
func (c *CloudWatch) ConvertOtelMetrics(m pmetric.Metrics) []*cloudwatch.MetricDatum {
	datums := make([]*cloudwatch.MetricDatum, 0, m.DataPointCount())
	// Metrics -> ResourceMetrics -> ScopeMetrics -> Metrics -> DataPoints
	// ^^ "Metric" is in there twice... confusing...
	resourceMetrics := m.ResourceMetrics()
	for i := 0; i < resourceMetrics.Len(); i++ {
		scopeMetrics := resourceMetrics.At(i).ScopeMetrics()
		for j := 0; j < scopeMetrics.Len(); j++ {
			metrics := scopeMetrics.At(j).Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				newDatums := c.ConvertOtelMetric(metric)
				datums = append(datums, newDatums...)
			}
		}
	}

	return datums
}
