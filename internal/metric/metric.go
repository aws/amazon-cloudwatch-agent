// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metric

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

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
