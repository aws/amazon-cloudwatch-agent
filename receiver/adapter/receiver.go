// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package adapter

import (
	"context"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/models"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/receiver/adapter/accumulator"
)

// AdaptedReceiver uses an OTel Scrape Controller to scrape metrics and has three phases:
// Start: Start the accumulator to initialize the logger and resources metrics
// Scrape: Gather metrics using the accumulator (e.g CPU https://github.com/influxdata/telegraf/blob/6e924fcd5cc2ce79a024b7275d865d7a19c455ed/plugins/inputs/cpu/cpu.go)
// Shutdown: Stop the scraper and flush the remaining metrics before shutting down the scraper
type AdaptedReceiver struct {
	logger      *zap.Logger
	input       *models.RunningInput
	ctx         context.Context
	consumer    consumer.Metrics
	accumulator accumulator.OtelAccumulator
}

func newAdaptedReceiver(input *models.RunningInput, ctx context.Context, consumer consumer.Metrics, logger *zap.Logger) *AdaptedReceiver {
	return &AdaptedReceiver{
		input:    input,
		ctx:      ctx,
		consumer: consumer,
		logger:   logger,
	}
}

func (r *AdaptedReceiver) start(_ context.Context, _ component.Host) error {
	r.logger.Debug("Starting adapter", zap.String("receiver", r.input.Config.Name))

	// TODO: Add Set Precision based on agent precision and agent interval
	// https://github.com/influxdata/telegraf/blob/3b3584b40b7c9ea10ae9cb02137fc072da202704/agent/agent.go#L316-L317

	r.accumulator = accumulator.NewAccumulator(r.input, r.ctx, r.consumer, r.logger)

	// Service Input differs from a regular plugin in that it operates a background service while Telegraf/CWAgent is running
	// https://github.com/influxdata/telegraf/blob/d67f75e55765d364ad0aabe99382656cb5b51014/docs/INPUTS.md#service-input-plugins
	if serviceInput, ok := r.input.Input.(telegraf.ServiceInput); ok {
		if err := serviceInput.Start(r.accumulator); err != nil {
			r.accumulator.AddError(err)
			return err
		}
	}

	return nil
}

func (r *AdaptedReceiver) scrape(_ context.Context) (pmetric.Metrics, error) {
	r.logger.Debug("Begin scraping metrics with adapter", zap.String("receiver", r.input.Config.Name))

	// Depending on the type of input, Gather may conditionally add metrics to the accumulator. For most service inputs,
	// the background process is the one sending the metrics further along the pipeline but there are cases where the
	// background process can buffer the metrics and calling Gather is what flushes the buffer. An example of this is
	// our statsd plugin: https://github.com/aws/amazon-cloudwatch-agent/blob/2e468dfd96cf9084ab76c2420262e1bbe1eca483/plugins/inputs/statsd/statsd.go
	if err := r.input.Input.Gather(r.accumulator); err != nil {
		r.accumulator.AddError(err)
		return pmetric.Metrics{}, err
	}

	return r.accumulator.GetOtelMetrics(), nil
}

func (r *AdaptedReceiver) shutdown(_ context.Context) error {
	r.logger.Debug("Shutdown adapter", zap.String("receiver", r.input.Config.Name))
	if serviceInput, ok := r.input.Input.(telegraf.ServiceInput); ok {
		serviceInput.Stop()
	}

	return nil
}
