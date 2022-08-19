// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus_scraper

import (
	"github.com/prometheus/prometheus/model/textparse"
)

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
