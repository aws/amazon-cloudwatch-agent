// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metric

import (
	"runtime"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
)

var serviceInputMeasurements = collections.NewSet[string](
	"prometheus",
)

// DataPoint is used to provide a common interface for OTEL metric data points.
type DataPoint[T any] interface {
	pmetric.NumberDataPoint | pmetric.HistogramDataPoint | pmetric.ExponentialHistogramDataPoint | pmetric.SummaryDataPoint
	MoveTo(dest T)
	Attributes() pcommon.Map
	StartTimestamp() pcommon.Timestamp
	SetStartTimestamp(pcommon.Timestamp)
	Timestamp() pcommon.Timestamp
	SetTimestamp(pcommon.Timestamp)
	CopyTo(dest T)
}

// DataPoints is used to provide a common interface for OTEL slice types.
type DataPoints[T DataPoint[T]] interface {
	Len() int
	At(i int) T
	EnsureCapacity(newCap int)
	AppendEmpty() T
	RemoveIf(f func(T) bool)
}

func RangeMetrics(md pmetric.Metrics, fn func(m pmetric.Metric)) {
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		ilms := rms.At(i).ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ms := ilms.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				fn(ms.At(k))
			}
		}
	}
}

func RangeDataPointAttributes(m pmetric.Metric, fn func(attrs pcommon.Map)) {
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		rangeDataPointAttributes[pmetric.NumberDataPoint](m.Gauge().DataPoints(), fn)
	case pmetric.MetricTypeSum:
		rangeDataPointAttributes[pmetric.NumberDataPoint](m.Sum().DataPoints(), fn)
	case pmetric.MetricTypeHistogram:
		rangeDataPointAttributes[pmetric.HistogramDataPoint](m.Histogram().DataPoints(), fn)
	case pmetric.MetricTypeExponentialHistogram:
		rangeDataPointAttributes[pmetric.ExponentialHistogramDataPoint](m.ExponentialHistogram().DataPoints(), fn)
	case pmetric.MetricTypeSummary:
		rangeDataPointAttributes[pmetric.SummaryDataPoint](m.Summary().DataPoints(), fn)
	}
}

func RangeDataPoints[T DataPoint[T]](dps DataPoints[T], fn func(dp T)) {
	for i := 0; i < dps.Len(); i++ {
		fn(dps.At(i))
	}
}

func rangeDataPointAttributes[T DataPoint[T]](dps DataPoints[T], fn func(attrs pcommon.Map)) {
	RangeDataPoints[T](dps, func(dp T) {
		fn(dp.Attributes())
	})
}

type Metrics struct {
	metrics pmetric.MetricSlice
}

func NewMetrics(metrics pmetric.MetricSlice) *Metrics {
	return &Metrics{
		metrics: metrics,
	}
}

func (metrics *Metrics) AddGaugeMetricDataPoint(
	name string,
	unit string,
	value interface{},
	timestamp pcommon.Timestamp,
	starttime pcommon.Timestamp,
	attributes map[string]string,
) {

	switch typedValue := value.(type) {
	case float64:
	case float32:
		value = float64(typedValue)
	case int64:
	case int:
		value = int64(typedValue)
	case int32:
		value = int64(typedValue)
	case bool:
		if typedValue {
			value = int64(1)
		} else {
			value = int64(0)
		}
	case byte:
		value = int64(typedValue)
	default:
		return
	}

	metric := metrics.metrics.AppendEmpty()
	metric.SetName(name)
	metric.SetUnit(unit)
	metric.SetEmptyGauge()
	dp := metric.Gauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(timestamp)
	dp.SetStartTimestamp(starttime)
	for key, val := range attributes {
		dp.Attributes().PutStr(key, val)
	}

	switch v := value.(type) {
	case float64:
		dp.SetDoubleValue(v)
	case int64:
		dp.SetIntValue(v)
	}
}

func DecorateMetricName(measurement, fieldKey string) string {
	if fieldKey == "" {
		return ""
	}
	// Statsd sets field name as default when the field is empty
	// https://github.com/aws/amazon-cloudwatch-agent/blob/6b3384ee44dcc07c1359b075eb9ea8e638126bc8/plugins/inputs/statsd/statsd.go#L492-L494
	if fieldKey == "value" {
		return measurement
	}

	// Honor metrics for service input (e.g prometheus)
	if serviceInputMeasurements.Contains(measurement) {
		return fieldKey
	}

	separator := "_"

	if runtime.GOOS == "windows" {
		separator = " "
	}

	return strings.Join([]string{measurement, fieldKey}, separator)
}
