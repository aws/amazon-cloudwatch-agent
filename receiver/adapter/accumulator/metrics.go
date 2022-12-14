// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package accumulator

import (
	"fmt"
	"log"
	"time"

	"github.com/influxdata/telegraf"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func ConvertTelegrafToOtelMetrics(measurement string, fields map[string]interface{}, tags map[string]string, tp telegraf.ValueType, t time.Time) (pmetric.Metrics, error) {
	// Instead of converting as tags as resource attributes, CWAgent will convert it to datapoint's attributes.
	// It would reduce memory consumption and hostmetricscraper does not add attributes to resource attributes.
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/99d2204f44d42db5eb7db2f7168a68304c9531c2/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/internal/metadata/generated_metrics_v2.go#L225-L249

	otelMetrics := pmetric.NewMetrics()
	switch tp {
	case telegraf.Counter:
		AddScopeMetricsIntoOtelMetrics(populateDataPointsForSum, otelMetrics, measurement, fields, tags, t)
	case telegraf.Gauge, telegraf.Untyped:
		AddScopeMetricsIntoOtelMetrics(populateDataPointsForGauge, otelMetrics, measurement, fields, tags, t)
	default:
		return pmetric.Metrics{}, fmt.Errorf("Unsupported Telegraf Metric type %v", tp)
	}

	return otelMetrics, nil
}

func NewOtelMetrics(measurement string) pmetric.Metrics {
	otelMetrics := pmetric.NewMetrics()
	otelMetrics.ResourceMetrics().AppendEmpty()

	return otelMetrics
}

type dataPointPopulator func(measurement string, metrics pmetric.MetricSlice, fields map[string]interface{}, tags map[string]string, timestamp pcommon.Timestamp)

// AddDataPointsIntoMetrics will use Telegraf's field (which holds  subset metrics from the main metrics)
// and convert to OTEL's datapoint
// Example:
//
//		Metric {                                                  -->  Metrics {
//		   Name: cpu                         				      -->    ResourceMetrics: [{
//		   TagList: [{key: mytag, value: myvalue}]			      -->       Resource: {
//		   FieldList: [										      -->         Attributes: [{key: mytag, value: myvalue}]
//		       {key: cpu_usage_user, value: 0.005},               -->       }
//		   ]                                                      -->       ScopeMetrics: [{
//		   Time: 1646946605										  -->         Metrics: [
//		   Type: Gauge                                            -->           {Name: cpu_usage_user
//		                                                          -->            DataType: Gauge
//		}														  -->            DataPoints: [{
//	                       									      -->              Attributes: [{key: mytag, value: myvalue}]
//	                       									      -->              Timestamp: 1646946605
//	                       									      --> 			   Type: Double
//	                        									  -->              Val: 0.005
//	                   											  -->            }]
//	                   											  -->         }]
//	                   											  -->       }]
//	                   											  -->    }]
//	                   											  --> }
func AddScopeMetricsIntoOtelMetrics(populateDataPoints dataPointPopulator, otelMetrics pmetric.Metrics, measurement string, fields map[string]interface{}, tags map[string]string, t time.Time) {
	rs := otelMetrics.ResourceMetrics().AppendEmpty()
	timestamp := pcommon.NewTimestampFromTime(t)
	metrics := rs.ScopeMetrics().AppendEmpty().Metrics()
	populateDataPoints(measurement, metrics, fields, tags, timestamp)
}

// Conversion from Influx Gauge to OTEL Gauge
// https://github.com/influxdata/influxdb-observability/blob/main/docs/metrics.md#gauge-metric
func populateDataPointsForGauge(measurement string, metrics pmetric.MetricSlice, fields map[string]interface{}, tags map[string]string, timestamp pcommon.Timestamp) {

	for field, value := range fields {
		metric := metrics.AppendEmpty()

		metric.SetName(getMetricName(measurement, field))

		populateNumberDataPoint(metric.SetEmptyGauge().DataPoints().AppendEmpty(), value, tags, timestamp)
	}
}

// Conversion from Influx Counter to OTEL Sum
// https://github.com/influxdata/influxdb-observability/blob/main/docs/metrics.md#sum-metric
func populateDataPointsForSum(measurement string, metrics pmetric.MetricSlice, fields map[string]interface{}, tags map[string]string, timestamp pcommon.Timestamp) {

	for field, value := range fields {
		metric := metrics.AppendEmpty()

		metric.SetName(getMetricName(measurement, field))

		// Sum is an  OTEL Stream Model which consists of:
		// * An Aggregation Temporality of delta or cumulative.
		// * Monotonic, to signal the time series data is increasing
		// For more information on OTEL Stream Model Sum, please following this document
		// https://opentelemetry.io/docs/reference/specification/metrics/datamodel/#sums
		sumMetric := metric.SetEmptySum()
		sumMetric.SetIsMonotonic(true)
		sumMetric.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		populateNumberDataPoint(sumMetric.DataPoints().AppendEmpty(), value, tags, timestamp)
	}
}

func populateNumberDataPoint(datapoint pmetric.NumberDataPoint, value interface{}, tags map[string]string, timestamp pcommon.Timestamp) {
	datapoint.SetTimestamp(timestamp)

	switch v := value.(type) {
	case int64:
		datapoint.SetIntValue(v)
	case float64:
		datapoint.SetDoubleValue(v)
	default:
		log.Fatalf("Invalid data type %v for NumberDataPoint ", v)
	}

	addTagsToAttributes(datapoint.Attributes(), tags)
}
