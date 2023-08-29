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
