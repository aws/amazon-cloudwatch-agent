// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package accumulator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/metric"
	"github.com/influxdata/telegraf/models"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/internal/util"
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
@input       Telegraf input plugin
@logger      Zap Logger
@precision   Round the timestamp during collection
@metrics     Otel Metrics which stacks multiple metrics through AddCounter, AddGauge, etc before resetting
*/
type otelAccumulator struct {
	input          *models.RunningInput
	isServiceInput bool
	ctx            context.Context
	consumer       consumer.Metrics
	logger         *zap.Logger
	precision      time.Duration
	metrics        pmetric.Metrics

	mutex sync.Mutex
}

func NewAccumulator(input *models.RunningInput, ctx context.Context, consumer consumer.Metrics, logger *zap.Logger) OtelAccumulator {
	_, isServiceInput := input.Input.(telegraf.ServiceInput)
	return &otelAccumulator{
		input:          input,
		isServiceInput: isServiceInput,
		ctx:            ctx,
		consumer:       consumer,
		logger:         logger,
		precision:      time.Nanosecond,
		metrics:        pmetric.NewMetrics(),
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

func (o *otelAccumulator) AddHistogram(measurement string, fields map[string]interface{}, tags map[string]string, t ...time.Time) {
	o.addMetric(measurement, tags, fields, telegraf.Histogram, t...)
}

func (o *otelAccumulator) AddFields(measurement string, fields map[string]interface{}, tags map[string]string, t ...time.Time) {
	o.addMetric(measurement, tags, fields, telegraf.Untyped, t...)
}

func (o *otelAccumulator) AddMetric(m telegraf.Metric) {
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
	mMetric, err := o.modifyMetricAndConvertToOtelValue(m)
	if err != nil {
		o.logger.Warn(
			"Conversion of metric values failed",
			zap.String("name", m.Name()),
			zap.Any("tags", m.Tags()),
			zap.Any("fields", m.Fields()),
			zap.Any("type", m.Type()),
			zap.Error(err),
		)
	}

	if mMetric == nil {
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

	// Gather and Start can add metrics concurrently. Therefore, a mutex ensures thread-safe access to the resource metrics
	o.mutex.Lock()
	defer o.mutex.Unlock()
	if o.isServiceInput {
		err := o.consumer.ConsumeMetrics(o.ctx, oMetric)
		if err != nil {
			o.AddError(err)
		}
	} else {
		oMetric.ResourceMetrics().MoveAndAppendTo(o.metrics.ResourceMetrics())
	}
}

// GetOtelMetrics return the final OTEL metric that were gathered by scrape controller for each plugin
func (o *otelAccumulator) GetOtelMetrics() pmetric.Metrics {
	finalMetrics := o.metrics
	o.metrics = pmetric.NewMetrics()
	return finalMetrics
}

// modifyMetricAndConvertToOtelValue modifies metric by filtering metrics, add prefix for each field in metrics, etc
// and convert to value supported by OTEL (int64 and float64).
// Distributions are not modified yet.
func (o *otelAccumulator) modifyMetricAndConvertToOtelValue(m telegraf.Metric) (telegraf.Metric, error) {
	if len(m.Fields()) == 0 {
		return nil, nil
	}

	// MakeMetric modifies metrics (e.g filter metrics, add prefix for measurement) by customer config
	// https://github.com/influxdata/telegraf/blob/5479df2eb5e8401773d604a83590d789a158c735/models/running_input.go#L91-L114
	mMetric := o.input.MakeMetric(m)
	if mMetric == nil {
		return nil, nil
	}

	if m.Type() == telegraf.Histogram {
		return mMetric, nil
	}
	// Otel only supports numeric data. Therefore, filter unsupported data type and convert metrics value to corresponding value before
	// converting the data model
	// https://github.com/open-telemetry/opentelemetry-collector/blob/bdc3e22d28006b6c9496568bd8d8bcf0aa1e4950/pdata/pmetric/metrics.go#L106-L113
	var errs error
	for field, value := range mMetric.Fields() {
		// Convert all int,uint to int64 and float to float64 and bool to int.
		otelValue, err := util.ToOtelValue(value)
		if err != nil {
			errs = multierr.Append(errs, fmt.Errorf("field (%q): %w", field, err))
		}

		if otelValue == nil {
			mMetric.RemoveField(field)
		} else if value != otelValue {
			mMetric.AddField(field, otelValue)
		}
	}

	if len(mMetric.Fields()) == 0 {
		return nil, fmt.Errorf("empty metrics after converting fields: %w", errs)
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

// TrackingAccumulator is an Accumulator that provides a signal when the
// metric has been fully processed. It drives to solve these two issues
// * https://github.com/influxdata/telegraf/issues/2905
// * https://github.com/influxdata/telegraf/issues/2919
// However, it will panic if the delivered message is reach to a certain threshold
// https://github.com/aws/telegraf/blob/066eb60aa48d74bf63dcd4e10b8f13db12b43c3b/agent/accumulator.go#L155-L159against
// which against CWA's goal (independent between input and output, etc)
// and can be solved by using OTEL Exporter persistent queue
// https://github.com/open-telemetry/opentelemetry-collector/tree/eebe590a465702b9f6b2a257ba3ab9735dd10152/exporter/exporterhelper#persistent-queue
func (o *otelAccumulator) WithTracking(maxTracked int) telegraf.TrackingAccumulator {
	o.logger.Error("CloudWatchAgent's adapter does not support tracking metrics.")
	return nil
}
