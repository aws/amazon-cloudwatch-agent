// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus_scraper

import (
	"errors"
	"log"
	"strings"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/pkg/textparse"
	"github.com/prometheus/prometheus/scrape"
)

const (
	prometheusMetricTypeKey = "prom_metric_type"

	histogramSummaryCountSuffix = "_count"
	histogramSummarySumSuffix   = "_sum"
	histogramBucketSuffix       = "_bucket"
	counterSuffix               = "_total"
)

var (
	histogramSummarySuffixes = []string{histogramSummaryCountSuffix, histogramSummarySumSuffix, histogramBucketSuffix}
	counterSuffixes          = []string{counterSuffix}
)

// Get the metric name in the TYPE comments for Summary and Histogram
// e.g # TYPE nginx_ingress_controller_request_duration_seconds histogram
//     # TYPE nginx_ingress_controller_ingress_upstream_latency_seconds summary
func normalizeMetricName(name string, suffixes []string) string {
	for _, s := range suffixes {
		if strings.HasSuffix(name, s) && name != s {
			return strings.TrimSuffix(name, s)
		}
	}
	return name
}

func (pm *PrometheusMetric) isCounter() bool {
	return pm.metricType == textparse.MetricTypeCounter
}

func (pm *PrometheusMetric) isGauge() bool {
	return pm.metricType == textparse.MetricTypeGauge
}

func (pm *PrometheusMetric) isHistogram() bool {
	return pm.metricType == textparse.MetricTypeHistogram
}

func (pm *PrometheusMetric) isSummary() bool {
	return pm.metricType == textparse.MetricTypeSummary
}

// Adapter to prometheus scrape.Target
type metadataCache interface {
	Metadata(metricName string) (scrape.MetricMetadata, bool)
}

// adapter to get metadata from scrape.Target
type mCache struct {
	t *scrape.Target
}

func (m *mCache) Metadata(metricName string) (scrape.MetricMetadata, bool) {
	return m.t.Metadata(metricName)
}

// Adapter to ScrapeManager to retrieve the cache by job and instance
type metadataService interface {
	Get(job, instance string) (metadataCache, error)
}

type metadataServiceImpl struct {
	sm ScrapeManager
}

func (t *metadataServiceImpl) Get(job, instance string) (metadataCache, error) {
	targetGroupMap := t.sm.TargetsAll()
	targetGroup, ok := targetGroupMap[job]

	if !ok {
		//when the job is replaced in relabel_config, TargetsAll() still return the map with old job name as key
		//so we need to go over all the targets to find the matching job name
		targetGroup = nil
	checkJobLoop:
		for _, potentialTargetGroup := range targetGroupMap {
			for _, target := range potentialTargetGroup {
				if target.Labels().Get(model.JobLabel) == job {
					targetGroup = potentialTargetGroup
					break checkJobLoop
				}
			}
		}

		if targetGroup == nil {
			return nil, errors.New("unable to find a target group with job=" + job)
		}
	}

	// from the same targetGroup, instance is not going to be duplicated
	for _, target := range targetGroup {
		if target.Labels().Get(model.InstanceLabel) == instance {
			return &mCache{target}, nil
		}
	}
	return nil, errors.New("unable to find a target with job=" + job + ", and instance=" + instance)
}

type ScrapeManager interface {
	TargetsAll() map[string][]*scrape.Target
}

type metricsTypeHandler struct {
	ms metadataService
}

func NewMetricsTypeHandler() *metricsTypeHandler {
	return &metricsTypeHandler{}
}

func (mth *metricsTypeHandler) SetScrapeManager(scrapeManager ScrapeManager) {
	if scrapeManager != nil {
		mth.ms = &metadataServiceImpl{sm: scrapeManager}
	}
}

// Return JobName and Instance
func GetScrapeTargetInfo(pmb PrometheusMetricBatch) (string, string, error) {

	for _, pm := range pmb {
		job, ok := pm.tags[model.JobLabel]
		if !ok {
			continue
		}
		instance, ok := pm.tags[model.InstanceLabel]
		if !ok {
			continue
		}
		return job, instance, nil
	}
	return "", "", errors.New("No Job and Instance Label found.")
}

func isInternalMetric(metricName string) bool {
	//For each endpoint, Prometheus produces a set of internal metrics. See https://prometheus.io/docs/concepts/jobs_instances/
	if metricName == "up" || strings.HasPrefix(metricName, "scrape_") {
		return true
	}
	return false
}

// Decorate the Metrics with Metric Types
func (mth *metricsTypeHandler) Handle(pmb PrometheusMetricBatch) (result PrometheusMetricBatch) {
	// Filter out Summary, Histogram and untyped Metrics and adding logging
	jobName, instanceId, err := GetScrapeTargetInfo(pmb)
	if err != nil {
		log.Printf("E! Failed to get Job Name and Instance ID from Prometheus metrics. \n")
		return result
	}

	mc, err := mth.ms.Get(jobName, instanceId)
	if err != nil {
		log.Printf("E! metricsTypeHandler.mc.Get(jobName, instanceId) error. jobName: %v;  instanceId: %v \n", jobName, instanceId)
		// The Pod has been terminated when we are going to handle its Prometheus metrics in the channel
		// Drop the metrics directly
		return result
	}
	for _, pm := range pmb {
		// normalize the summary metric first, then if metric name == standardMetricName, it means it is not been normalized by summary
		// , then normalize the counter suffix if it failed to find metadata.
		standardMetricName := normalizeMetricName(pm.metricName, histogramSummarySuffixes)
		mm, ok := mc.Metadata(standardMetricName)
		if !ok {
			if pm.metricName != standardMetricName {
				// perform a 2nd lookup with the original metric name
				// It could happen if non histogram/summary ends with one of those _count/_sum suffixes
				mm, ok = mc.Metadata(pm.metricName)
			} else {
				// normalize the counter type suffixes, like "_total" suffix
				standardMetricName = normalizeMetricName(pm.metricName, counterSuffixes)
				mm, ok = mc.Metadata(standardMetricName)
			}
		}
		if ok {
			pm.metricType = string(mm.Type)
			pm.tags[prometheusMetricTypeKey] = pm.metricType
		} else {
			if !isInternalMetric(pm.metricName) {
				log.Printf("E! metricsHandler NO metaData for %v | %v | %v \n", pm.metricName, instanceId, jobName)
			}
		}

		if pm.metricType == "" && !isInternalMetric(pm.metricName) {
			log.Printf("E! metric_type ERROR: %v|%v|%v|%v  \n", pm.metricName, jobName, instanceId, pm.metricType)

			// skip the non-internal metrics with empty metric type due to cache not ready
			continue
		}
		result = append(result, pm)
	}

	return result
}
