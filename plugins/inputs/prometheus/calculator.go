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
	log.Printf("Starting Calculate with batch size: %d", len(pmb))

	var gauges PrometheusMetricBatch
	var counters PrometheusMetricBatch
	var summaries PrometheusMetricBatch

	// Log initial batch details
	for i, pm := range pmb {
		log.Printf("Input Metric[%d]: Name: %s, NameBeforeRelabel: %s, Type: %s, Value: %f, Time: %d",
			i,
			pm.metricName,
			pm.metricNameBeforeRelabel,
			pm.metricType,
			pm.metricValue,
			pm.timeInMS)
		log.Printf("Tags for metric[%d]: %v", i, pm.tags)
	}

	for i, pm := range pmb {
		if pm.isGauge() {
			log.Printf("Processing Gauge metric[%d]: %s", i, pm.metricName)
			gauges = appendValidValue(gauges, pm)
			log.Printf("Gauge metrics count after append: %d", len(gauges))

		} else if pm.isCounter() {
			log.Printf("Processing Counter metric[%d]: %s", i, pm.metricName)
			if calculatedMetric := c.deltaCalculator.calculate(pm); calculatedMetric != nil {
				counters = append(counters, calculatedMetric)
				log.Printf("Counter calculated value: %f", calculatedMetric.metricValue)
			} else {
				log.Printf("Counter metric[%d] calculation returned nil", i)
			}
			log.Printf("Counter metrics count after append: %d", len(counters))

		} else if pm.isSummary() {
			log.Printf("Processing Summary metric[%d]: %s", i, pm.metricName)
			if strings.HasSuffix(pm.metricName, histogramSummaryCountSuffix) ||
				strings.HasSuffix(pm.metricName, histogramSummarySumSuffix) {
				log.Printf("Processing Summary count/sum metric: %s", pm.metricName)
				if calculatedMetric := c.deltaCalculator.calculate(pm); calculatedMetric != nil {
					summaries = append(summaries, calculatedMetric)
					log.Printf("Summary calculated value: %f", calculatedMetric.metricValue)
				} else {
					log.Printf("Summary metric[%d] calculation returned nil", i)
				}
			} else {
				log.Printf("Processing regular Summary metric: %s", pm.metricName)
				summaries = appendValidValue(summaries, pm)
			}
			log.Printf("Summary metrics count after append: %d", len(summaries))
		} else {
			log.Printf("Metric[%d] type not recognized: %s", i, pm.metricType)
		}
	}

	result = append(result, gauges...)
	result = append(result, counters...)
	result = append(result, summaries...)

	// Log final results
	log.Printf("Final metrics counts - Gauges: %d, Counters: %d, Summaries: %d, Total: %d",
		len(gauges),
		len(counters),
		len(summaries),
		len(result))

	// Log detailed final results
	log.Printf("=== Final Processed Metrics ===")
	for i, pm := range result {
		log.Printf("Result Metric[%d]: Name: %s, Type: %s, Value: %f, Time: %d",
			i,
			pm.metricName,
			pm.metricType,
			pm.metricValue,
			pm.timeInMS)
		log.Printf("Result Tags[%d]: %v", i, pm.tags)
	}

	return
}

func NewCalculator() *Calculator {
	return &Calculator{
		deltaCalculator: NewDeltaCalculator(),
	}
}
