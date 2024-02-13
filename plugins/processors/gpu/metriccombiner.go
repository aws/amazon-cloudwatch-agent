// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package gpu

import (
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type metricCombiner struct {
	logger *zap.Logger
	rule   metricMutationRule
}

func NewMetricCombiner(logger *zap.Logger, rule metricMutationRule) *metricCombiner {
	return &metricCombiner{
		logger: logger,
		rule:   rule,
	}
}

// basic idea/code is from metricsgenerationprocessor [BETA] https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/processor/metricsgenerationprocessor/README.md
func (mm *metricCombiner) Process(ms pmetric.Metrics) error {
	rms := ms.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		nameToMetricMap := mm.getNameToMetricMap(rm)

		from2Val := float64(0)
		from1, ok := nameToMetricMap[mm.rule.sources[0]]
		if !ok {
			mm.logger.Debug("Missing first metric", zap.String("metric_name", mm.rule.sources[0]))
			continue
		}
		from2, ok := nameToMetricMap[mm.rule.sources[1]]
		if !ok {
			mm.logger.Debug("Missing second metric", zap.String("metric_name", mm.rule.sources[1]))
			continue
		}
		from2Val = mm.getMetricValue(from2)
		mm.generateMetrics(rm, mm.rule.target, from1.Name(), from1.Unit(), from2Val)
	}
	return nil
}

func (mm *metricCombiner) getNameToMetricMap(rm pmetric.ResourceMetrics) map[string]pmetric.Metric {
	ilms := rm.ScopeMetrics()
	metricMap := make(map[string]pmetric.Metric)

	for i := 0; i < ilms.Len(); i++ {
		ilm := ilms.At(i)
		metricSlice := ilm.Metrics()
		for j := 0; j < metricSlice.Len(); j++ {
			metric := metricSlice.At(j)
			metricMap[metric.Name()] = metric
		}
	}
	return metricMap
}

func (mm *metricCombiner) getMetricValue(metric pmetric.Metric) float64 {
	if metric.Type() == pmetric.MetricTypeGauge {
		dataPoints := metric.Gauge().DataPoints()
		if dataPoints.Len() > 0 {
			switch dataPoints.At(0).ValueType() {
			case pmetric.NumberDataPointValueTypeDouble:
				return dataPoints.At(0).DoubleValue()
			case pmetric.NumberDataPointValueTypeInt:
				return float64(dataPoints.At(0).IntValue())
			}
		}
		return 0
	}
	return 0
}

// generateMetrics creates a new metric based on the given rule and add it to the Resource Metric.
// The value for newly calculated metrics is always a floting point number and the dataType is set
// as MetricTypeDoubleGauge.
func (mm *metricCombiner) generateMetrics(rm pmetric.ResourceMetrics, newName string, f1name string, unit string, f2val float64) {
	ilms := rm.ScopeMetrics()
	for i := 0; i < ilms.Len(); i++ {
		ilm := ilms.At(i)
		metricSlice := ilm.Metrics()
		for j := 0; j < metricSlice.Len(); j++ {
			metric := metricSlice.At(j)
			if metric.Name() == f1name {
				newMetric := ilm.Metrics().AppendEmpty()
				newMetric.SetName(newName)
				newMetric.SetUnit(unit)
				newMetric.SetEmptyGauge()
				mm.addDoubleGaugeDataPoints(metric, newMetric, f2val)
			}
		}
	}
}

func (mm *metricCombiner) addDoubleGaugeDataPoints(from pmetric.Metric, to pmetric.Metric, m2val float64) {
	dataPoints := from.Gauge().DataPoints()
	for i := 0; i < dataPoints.Len(); i++ {
		from := dataPoints.At(i)
		var val float64
		switch from.ValueType() {
		case pmetric.NumberDataPointValueTypeDouble:
			val = from.DoubleValue()
		case pmetric.NumberDataPointValueTypeInt:
			val = float64(from.IntValue())
		}

		newDp := to.Gauge().DataPoints().AppendEmpty()
		from.CopyTo(newDp)
		value := val + m2val
		newDp.SetDoubleValue(value)
	}
}
