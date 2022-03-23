// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus_scraper

import (
	"log"
	"github.com/prometheus/prometheus/pkg/textparse"
)

const (
	MaxDropMetricsLogged = 1000
)

type MetricsFilter struct {
	maxDropMetricsLogged int
	droppedMetrics       map[string]string
	hitMaxLimit          bool
}

// Filter out and Log the unsupported metric types
func (mf *MetricsFilter) Filter(pmb PrometheusMetricBatch,dropUnsupportedMetrics bool) (result PrometheusMetricBatch) {
	for _, pm := range pmb {
		if !pm.isGauge() && !pm.isCounter() && !pm.isSummary() {
			if dropUnsupportedMetrics {
				if mf.droppedMetrics == nil {
					mf.droppedMetrics = make(map[string]string, mf.maxDropMetricsLogged)
					log.Println("I! Drop Prometheus metrics with unsupported types. Only Gauge, Counter and Summary are supported.")
					log.Printf("I! Please enable CWAgent debug mode to view the first %d dropped metrics \n", mf.maxDropMetricsLogged)
				}

				if !mf.hitMaxLimit && (len(mf.droppedMetrics) < mf.maxDropMetricsLogged) {
					if _, ok := mf.droppedMetrics[pm.metricName]; !ok {
						log.Printf("D! [%d/%d] Unsupported Prometheus metric: %s with type: %s",
							len(mf.droppedMetrics)+1,
							mf.maxDropMetricsLogged, pm.metricName,
							pm.metricType)
						mf.droppedMetrics[pm.metricName] = pm.metricType
						if len(mf.droppedMetrics) == mf.maxDropMetricsLogged {
							mf.hitMaxLimit = true
							mf.droppedMetrics = make(map[string]string)
						}
					}
				}
				
				continue
			} else {
				pm.metricType = textparse.MetricTypeUnknown
			}
		}
		
		result = append(result, pm)
	}
	return
}

func NewMetricsFilter() *MetricsFilter {
	return &MetricsFilter{maxDropMetricsLogged: MaxDropMetricsLogged}
}
