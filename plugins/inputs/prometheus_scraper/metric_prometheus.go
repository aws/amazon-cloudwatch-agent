// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus_scraper

import (
	"github.com/prometheus/prometheus/model/textparse"
	"github.com/prometheus/prometheus/model/value"
	"math"
)

type PrometheusMetricBatch []*PrometheusMetric

type PrometheusMetric struct {
	tags        map[string]string
	metricName  string
	metricValue float64
	metricType  string
	timeInMS    int64 // Unix time in milli-seconds
}

func (pm *PrometheusMetric) isValueValid() bool {
	//treat NaN and +/-Inf values as invalid as emf log doesn't support them
	return !value.IsStaleNaN(pm.metricValue) && !math.IsNaN(pm.metricValue) && !math.IsInf(pm.metricValue, 0)
}

func (pm *PrometheusMetric) isCounter() bool {
	return pm.metricType == string(textparse.MetricTypeCounter)
}

func (pm *PrometheusMetric) isGauge() bool {
	return pm.metricType == string(textparse.MetricTypeGauge)
}

func (pm *PrometheusMetric) isHistogram() bool {
	return pm.metricType == string(textparse.MetricTypeHistogram)
}

func (pm *PrometheusMetric) isSummary() bool {
	return pm.metricType == string(textparse.MetricTypeSummary)
}
