// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatch

import (
	"log"
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
		if isEntityAttribute(k) {
			return true
		}
		mTags[k] = v.AsString()
		return true
	})
	return BuildDimensions(mTags)
}

func isEntityAttribute(k string) bool {
	_, ok := entityattributes.KeyAttributeEntityToShortNameMap[k]
	_, ok2 := entityattributes.AttributeEntityToShortNameMap[k]
	return ok || ok2
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
	default:
		log.Printf("E! cloudwatch: Unsupported type, %s", m.Type())
	}
	return []*aggregationDatum{}
}

func ConvertOtelMetrics(m pmetric.Metrics) []*aggregationDatum {
	datums := make([]*aggregationDatum, 0, m.DataPointCount())
	for i := 0; i < m.ResourceMetrics().Len(); i++ {
		entity := fetchEntityFields(m.ResourceMetrics().At(i).Resource().Attributes())
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

func fetchEntityFields(resourceAttributes pcommon.Map) cloudwatch.Entity {
	keyAttributesMap := map[string]*string{}
	attributeMap := map[string]*string{}

	processEntityAttributes(entityattributes.KeyAttributeEntityToShortNameMap, keyAttributesMap, resourceAttributes)
	processEntityAttributes(entityattributes.AttributeEntityToShortNameMap, attributeMap, resourceAttributes)

	return cloudwatch.Entity{
		KeyAttributes: keyAttributesMap,
		Attributes:    attributeMap,
	}
}

// processEntityAttributes fetches the aws.entity fields and creates an entity to be sent at the PutMetricData call. It also
// removes the entity attributes so that it is not tagged as a dimension, and reduces the size of the PMD payload.
func processEntityAttributes(entityMap map[string]string, targetMap map[string]*string, mutableResourceAttributes pcommon.Map) {
	for entityField, shortName := range entityMap {
		if val, ok := mutableResourceAttributes.Get(entityField); ok {
			if strVal := val.Str(); strVal != "" {
				targetMap[shortName] = aws.String(strVal)
			}
			mutableResourceAttributes.Remove(entityField)
		}
	}
}
