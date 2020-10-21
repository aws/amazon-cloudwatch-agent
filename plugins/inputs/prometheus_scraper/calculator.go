// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus_scraper

import (
	"strings"
)

type Calculator struct {
	deltaCalculator *DeltaCalculator
}

// Do calculation based on metric type
func (c *Calculator) Calculate(pmb PrometheusMetricBatch) (result PrometheusMetricBatch) {
	var gauges PrometheusMetricBatch
	var counters PrometheusMetricBatch
	var summaries PrometheusMetricBatch

	for _, pm := range pmb {
		if pm.isGauge() && !pm.isValueStale() {
			gauges = append(gauges, pm)
		} else if pm.isCounter() {
			if calculatedMetric := c.deltaCalculator.calculate(pm); calculatedMetric != nil {
				counters = append(counters, calculatedMetric)
			}
		} else if pm.isSummary() && !pm.isValueStale() {
			// calculate the delta for <basename>_count and <basename>_sum metrics as well
			if strings.HasSuffix(pm.metricName, histogramSummaryCountSuffix) ||
				strings.HasSuffix(pm.metricName, histogramSummarySumSuffix) {
				if calculatedMetric := c.deltaCalculator.calculate(pm); calculatedMetric != nil {
					summaries = append(summaries, calculatedMetric)
				}
			} else {
				summaries = append(summaries, pm)
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
