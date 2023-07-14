// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metric

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestAddGaugeMetricDataPoint(t *testing.T) {
	timestamp := pcommon.NewTimestampFromTime(time.Now())
	attributes := map[string]string{
		"Attr": "attr",
	}
	metrics := NewMetrics(pmetric.NewMetricSlice())
	metrics.AddGaugeMetricDataPoint("float64", "", 1.23, timestamp, timestamp, attributes)
	metrics.AddGaugeMetricDataPoint("float32", "", float32(4.56), timestamp, timestamp, attributes)
	metrics.AddGaugeMetricDataPoint("int64", "", int64(123), timestamp, timestamp, attributes)
	metrics.AddGaugeMetricDataPoint("int32", "", int32(1234), timestamp, timestamp, attributes)
	metrics.AddGaugeMetricDataPoint("int", "", 456, timestamp, timestamp, attributes)
	metrics.AddGaugeMetricDataPoint("boolTrue", "", true, timestamp, timestamp, attributes)
	metrics.AddGaugeMetricDataPoint("boolFalse", "", false, timestamp, timestamp, attributes)
	metrics.AddGaugeMetricDataPoint("byte", "", byte(8), timestamp, timestamp, attributes)

	// unsupported, will get dropped
	metrics.AddGaugeMetricDataPoint("string", "none", "string", timestamp, timestamp, attributes)

	expected := pmetric.NewMetricSlice()

	metric := expected.AppendEmpty()
	metric.SetEmptyGauge()
	metric.SetName("float64")
	dp := metric.Gauge().DataPoints().AppendEmpty()
	dp.SetDoubleValue(1.23)
	dp.SetTimestamp(timestamp)
	dp.SetStartTimestamp(timestamp)
	dp.Attributes().PutStr("Attr", "attr")

	metric = expected.AppendEmpty()
	metric.SetEmptyGauge()
	metric.SetName("float32")
	dpf32 := metric.Gauge().DataPoints().AppendEmpty()
	dpf32.SetDoubleValue(float64(float32(4.56)))
	dpf32.SetTimestamp(timestamp)
	dpf32.SetStartTimestamp(timestamp)
	dpf32.Attributes().PutStr("Attr", "attr")

	metric = expected.AppendEmpty()
	metric.SetEmptyGauge()
	metric.SetName("int64")
	dpi64 := metric.Gauge().DataPoints().AppendEmpty()
	dpi64.SetIntValue(123)
	dpi64.SetTimestamp(timestamp)
	dpi64.SetStartTimestamp(timestamp)
	dpi64.Attributes().PutStr("Attr", "attr")

	metric = expected.AppendEmpty()
	metric.SetEmptyGauge()
	metric.SetName("int32")
	dpi32 := metric.Gauge().DataPoints().AppendEmpty()
	dpi32.SetIntValue(1234)
	dpi32.SetTimestamp(timestamp)
	dpi32.SetStartTimestamp(timestamp)
	dpi32.Attributes().PutStr("Attr", "attr")

	metric = expected.AppendEmpty()
	metric.SetEmptyGauge()
	metric.SetName("int")
	dpi := metric.Gauge().DataPoints().AppendEmpty()
	dpi.SetIntValue(456)
	dpi.SetTimestamp(timestamp)
	dpi.SetStartTimestamp(timestamp)
	dpi.Attributes().PutStr("Attr", "attr")

	metric = expected.AppendEmpty()
	metric.SetEmptyGauge()
	metric.SetName("boolTrue")
	dpb := metric.Gauge().DataPoints().AppendEmpty()
	dpb.SetIntValue(1)
	dpb.SetTimestamp(timestamp)
	dpb.SetStartTimestamp(timestamp)
	dpb.Attributes().PutStr("Attr", "attr")

	metric = expected.AppendEmpty()
	metric.SetEmptyGauge()
	metric.SetName("boolFalse")
	dpbf := metric.Gauge().DataPoints().AppendEmpty()
	dpbf.SetIntValue(0)
	dpbf.SetTimestamp(timestamp)
	dpbf.SetStartTimestamp(timestamp)
	dpbf.Attributes().PutStr("Attr", "attr")

	metric = expected.AppendEmpty()
	metric.SetEmptyGauge()
	metric.SetName("byte")
	dpbyte := metric.Gauge().DataPoints().AppendEmpty()
	dpbyte.SetIntValue(8)
	dpbyte.SetTimestamp(timestamp)
	dpbyte.SetStartTimestamp(timestamp)
	dpbyte.Attributes().PutStr("Attr", "attr")

	assert.Equal(t, expected, metrics.metrics)
}
