// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"log"
	"strings"
)

type Calculator struct {
	deltaCalculator *DeltaCalculator
}

func appendValidValue(pmb PrometheusMetricBatch, pm *PrometheusMetric) PrometheusMetricBatch {
	if !pm.isValueValid() {
		log.Printf("D! Drop metric with NaN or Inf value: %v", pm)
		return pmb
	}
	return append(pmb, pm)
}

// Do calculation based on metric type
func (c *Calculator) Calculate(pmb PrometheusMetricBatch) (result PrometheusMetricBatch) {
	var gauges PrometheusMetricBatch
	var counters PrometheusMetricBatch
	var summaries PrometheusMetricBatch

	for _, pm := range pmb {
		if pm.isGauge() {
			gauges = appendValidValue(gauges, pm)
		} else if pm.isCounter() {
			if calculatedMetric := c.deltaCalculator.calculate(pm); calculatedMetric != nil {
				counters = append(counters, calculatedMetric)
			}
		} else if pm.isSummary() {
			// calculate the delta for <basename>_count and <basename>_sum metrics as well
			if strings.HasSuffix(pm.metricName, histogramSummaryCountSuffix) ||
				strings.HasSuffix(pm.metricName, histogramSummarySumSuffix) {
				if calculatedMetric := c.deltaCalculator.calculate(pm); calculatedMetric != nil {
					summaries = append(summaries, calculatedMetric)
				}
			} else {
				summaries = appendValidValue(summaries, pm)
			}
		}
	}

	result = append(result, gauges...)
	result = append(result, counters...)
	result = append(result, summaries...)
	return
}

func NewCalculator() *Calculator {
	return &Calculator{
		deltaCalculator: NewDeltaCalculator(),
	}
}
