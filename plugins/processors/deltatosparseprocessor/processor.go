// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package deltatosparseprocessor

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type deltaToSparseProcessor struct {
	includeMap map[string]struct{}
	*Config
	logger *zap.Logger
}

func newDeltaToSparseProcessor(config *Config, logger *zap.Logger) *deltaToSparseProcessor {
	d := &deltaToSparseProcessor{
		Config: config,
		logger: logger,
	}

	d.includeMap = map[string]struct{}{}
	for _, metricName := range config.Include {
		d.includeMap[metricName] = struct{}{}
	}
	return d
}

func (dtsp *deltaToSparseProcessor) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rs := rms.At(i)
		ilms := rs.ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ils := ilms.At(j)
			metrics := ils.Metrics()

			metricsLength := metrics.Len()
			for k := 0; k < metricsLength; k++ {
				m := metrics.At(k)
				if dtsp.shouldConvertMetric(m.Name()) &&
					m.Type() == pmetric.MetricTypeSum &&
					m.Sum().IsMonotonic() &&
					m.Sum().AggregationTemporality() == pmetric.AggregationTemporalityDelta {
					m.Sum().DataPoints().RemoveIf(func(dp pmetric.NumberDataPoint) bool {
						return dp.DoubleValue() == 0
					})
				}
			}
		}
	}
	return md, nil
}

func (dtsp *deltaToSparseProcessor) shouldConvertMetric(metricName string) bool {
	_, contains := dtsp.includeMap[metricName]
	return contains
}
