// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package accumulator

import (
	"errors"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/internal/util"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/metric"
	"github.com/influxdata/telegraf/models"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"time"
)

// OtelAccumulator implements the telegraf.Accumulator interface, but works as an OTel plugin by passing the metrics
// onward to the next consumer
type OtelAccumulator interface {
	// Accumulator Interface https://github.com/influxdata/telegraf/blob/381dc2272390cd9de1ce2b047a953f8337b55647/accumulator.go
	telegraf.Accumulator

	// GetOtelMetrics return the final OTEL metric that were gathered by scrape controller for each plugin
	GetOtelMetrics() pmetric.Metrics
}

/*
otelAccumulator struct
@input       Telegraf input plugins
@logger      Zap Logger
@precision   Round the timestamp during collection
@metrics     Otel Metrics which stacks multiple metrics through AddCounter, AddGauge, etc before resetting
*/
type otelAccumulator struct {
	input     *models.RunningInput
	logger    *zap.Logger
	precision time.Duration
	metrics   pmetric.Metrics
}

func NewAccumulator(input *models.RunningInput, logger *zap.Logger) OtelAccumulator {
	return &otelAccumulator{
		input:     input,
		logger:    logger,
		precision: time.Nanosecond,
		metrics:   pmetric.NewMetrics(),
	}
}

func (o *otelAccumulator) AddGauge(measurement string, fields map[string]interface{}, tags map[string]string, t ...time.Time) {
	o.addMetric(measurement, tags, fields, telegraf.Gauge, t...)
}

func (o *otelAccumulator) AddCounter(measurement string, fields map[string]interface{}, tags map[string]string, t ...time.Time) {
	o.addMetric(measurement, tags, fields, telegraf.Counter, t...)
}

// AddSummary is only being used by OpenTelemetry and Prometheus. https://github.com/influxdata/telegraf/search?q=AddSummary
// However, we already have a Prometheus Receiver which uses AddFields so there is actually no use case for AddSummary.
func (o *otelAccumulator) AddSummary(measurement string, fields map[string]interface{}, tags map[string]string, t ...time.Time) {
	o.logger.Error("CloudWatchAgent's adapter does not support Telegraf Summary.")
}

// AddHistogram is only being used by OpenTelemetry and Prometheus. https://github.com/influxdata/telegraf/search?q=AddHistogram
// Therefore, same no use case as AddSummary
func (o *otelAccumulator) AddHistogram(measurement string, fields map[string]interface{}, tags map[string]string, t ...time.Time) {
	o.logger.Error("CloudWatchAgent's adapter does not support Telegraf Histogram.")
}

func (o *otelAccumulator) AddFields(measurement string, fields map[string]interface{}, tags map[string]string, t ...time.Time) {
	o.addMetric(measurement, tags, fields, telegraf.Untyped, t...)
}

func (o otelAccumulator) AddMetric(m telegraf.Metric) {
	m.SetTime(m.Time().Round(o.precision))
	o.convertToOtelMetricsAndAddMetric(m)
}

func (o *otelAccumulator) SetPrecision(precision time.Duration) {
	o.precision = precision
}

func (o *otelAccumulator) AddError(err error) {
	if err == nil {
		return
	}

	o.logger.Error("Error with adapter", zap.Error(err))
}

func (o *otelAccumulator) WithTracking(maxTracked int) telegraf.TrackingAccumulator {
	panic("implement me")
}

// addMetric implements from addFields https://github.com/influxdata/telegraf/blob/381dc2272390cd9de1ce2b047a953f8337b55647/agent/accumulator.go#L86-L97
// which will filter the subset metrics and modify metadata on the metrics (e.g name)
func (o *otelAccumulator) addMetric(
	measurement string,
	tags map[string]string,
	fields map[string]interface{},
	metricType telegraf.ValueType,
	t ...time.Time,
) {
	m := metric.New(measurement, tags, fields, o.getTime(t), metricType)
	o.convertToOtelMetricsAndAddMetric(m)
}

// convertToOtelMetricsAndAddMetric converts Telegraf's Metric model to OTEL Stream Model
// and add the OTEl Metric to channel
func (o *otelAccumulator) convertToOtelMetricsAndAddMetric(m telegraf.Metric) {

	mMetric, err := o.modifyMetricandConvertToOtelValue(m)
	if err != nil {
		o.logger.Warn("Filter and convert failed",
			zap.String("name", m.Name()),
			zap.Any("tags", m.Tags()),
			zap.Any("fields", m.Fields()),
			zap.Any("type", m.Type()), zap.Error(err))
		return
	}

	oMetric, err := ConvertTelegrafToOtelMetrics(mMetric.Name(), mMetric.Fields(), mMetric.Tags(), mMetric.Type(), mMetric.Time())
	if err != nil {
		o.logger.Warn("Convert to Otel Metric failed",
			zap.Any("name", oMetric),
			zap.Any("tags", mMetric.Tags()),
			zap.Any("fields", mMetric.Fields()),
			zap.Any("type", mMetric.Type()),
			zap.Error(err))
		return
	}
	oMetric.ResourceMetrics().MoveAndAppendTo(o.metrics.ResourceMetrics())
}

// GetOtelMetrics return the final OTEL metric that were gathered by scrape controller for each plugin
func (o *otelAccumulator) GetOtelMetrics() pmetric.Metrics {
	// MoveTo copy all relates properties and reset the current instance to its zero value. However,
	// the next time adding resources metrics, the Otel Metric will have nil value.
	// https://pkg.go.dev/go.opentelemetry.io/collector/pdata/pmetric#Metric.MoveTo
	finalMetrics := pmetric.NewMetrics()
	o.metrics.MoveTo(finalMetrics)
	return finalMetrics
}

// modifyMetricandConvertToOtelValue modifies metric by filtering metrics, add prefix for each field in metrics, etc
// and convert to value supported by OTEL (int64 and float64)
func (o *otelAccumulator) modifyMetricandConvertToOtelValue(m telegraf.Metric) (telegraf.Metric, error) {
	if len(m.Fields()) == 0 {
		return nil, errors.New("empty metrics before filterting metrics")
	}

	// MakeMetric modifies metrics (e.g filter metrics, add prefix for measurement) by customer config
	// https://github.com/influxdata/telegraf/blob/5479df2eb5e8401773d604a83590d789a158c735/models/running_input.go#L91-L114
	mMetric := o.input.MakeMetric(m)
	if mMetric == nil {
		return nil, errors.New("empty metrics after filterting metrics")
	}

	// Otel only supports numeric data. Therefore, filter unsupported data type and convert metrics value to corresponding value before
	// converting the data model
	// https://github.com/open-telemetry/opentelemetry-collector/blob/bdc3e22d28006b6c9496568bd8d8bcf0aa1e4950/pdata/pmetric/metrics.go#L106-L113
	for field, value := range mMetric.Fields() {
		// Convert all int,uint to int64 and float to float64 and bool to int
		// All other types are ignored
		otelValue := util.ToOtelValue(value)

		if otelValue == nil {
			mMetric.RemoveField(field)
		} else if value != otelValue {
			mMetric.AddField(field, otelValue)
		}
	}

	if len(mMetric.Fields()) == 0 {
		return nil, errors.New("empty metrics after final conversion")
	}

	return mMetric, nil
}

// Adapted from https://github.com/influxdata/telegraf/blob/b526945c64a56450b836656a6a2002b8bf748b78/agent/accumulator.go#L112
func (o *otelAccumulator) getTime(t []time.Time) time.Time {
	var timestamp time.Time
	if len(t) > 0 {
		timestamp = t[0]
	} else {
		timestamp = time.Now()
	}
	return timestamp.Round(o.precision)
}
