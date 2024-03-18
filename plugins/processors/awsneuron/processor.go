// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsneuron

import (
	"context"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsneuron/internal"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type awsneuronprocessor struct {
	*Config
	logger                 *zap.Logger
	cancelFunc             context.CancelFunc
	shutdownC              chan bool
	started                bool
	metricModifier         *internal.MetricModifier
	memoryMetricAggregator *internal.MemoryMetricAggregator
}

func newAwsNeuronProcessor(config *Config, logger *zap.Logger) *awsneuronprocessor {
	_, cancel := context.WithCancel(context.Background())
	d := &awsneuronprocessor{
		Config:                 config,
		logger:                 logger,
		cancelFunc:             cancel,
		metricModifier:         internal.NewMetricModifier(logger),
		memoryMetricAggregator: internal.NewMemoryMemoryAggregator(),
	}
	return d
}

func (d *awsneuronprocessor) processMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	if !d.started {
		return pmetric.NewMetrics(), nil
	}

	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rs := rms.At(i)
		ilms := rs.ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ils := ilms.At(j)
			metrics := ils.Metrics()

			newMetrics := pmetric.NewMetricSlice()
			for k := 0; k < metrics.Len(); k++ {
				m := metrics.At(k)
				d.memoryMetricAggregator.AggregateMemoryMetric(m)
				d.metricModifier.ModifyMetric(m).MoveAndAppendTo(newMetrics)
			}
			if d.memoryMetricAggregator.MemoryMetricsFound {
				aggregatedMemoryMetric := d.memoryMetricAggregator.FlushAggregatedMemoryMetric()
				d.metricModifier.ModifyMetric(aggregatedMemoryMetric).MoveAndAppendTo(newMetrics)
			}

			newMetrics.CopyTo(metrics)
		}
	}

	return md, nil
}

func (d *awsneuronprocessor) Shutdown(context.Context) error {
	close(d.shutdownC)
	d.cancelFunc()
	return nil
}

func (d *awsneuronprocessor) Start(ctx context.Context, _ component.Host) error {
	d.shutdownC = make(chan bool)
	d.started = true
	return nil
}
