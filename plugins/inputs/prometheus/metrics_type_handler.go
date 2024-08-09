// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"errors"
	"fmt"
	"log"
	"strings"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
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

// Get the metric name in the TYPE comments for Summary and Histogram. E.g:
// # TYPE nginx_ingress_controller_request_duration_seconds histogram
// # TYPE nginx_ingress_controller_ingress_upstream_latency_seconds summary
func normalizeMetricName(name string, suffixes []string) string {
	for _, s := range suffixes {
		if strings.HasSuffix(name, s) && name != s {
			return strings.TrimSuffix(name, s)
		}
	}
	return name
}

func (pm *PrometheusMetric) isCounter() bool {
	return pm.metricType == string(v1.MetricTypeCounter)
}

func (pm *PrometheusMetric) isGauge() bool {
	return pm.metricType == string(v1.MetricTypeGauge)
}

func (pm *PrometheusMetric) isHistogram() bool {
	return pm.metricType == string(v1.MetricTypeHistogram)
}

func (pm *PrometheusMetric) isSummary() bool {
	return pm.metricType == string(v1.MetricTypeSummary)
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
	return m.t.GetMetadata(metricName)
}

// Adapter to ScrapeManager to retrieve the cache by job and instance
type metadataService interface {
	Get(job, instance string) (metadataCache, error)
}

type metadataServiceImpl struct {
	sm ScrapeManager
}

// job and instance MUST be using value before relabel
func (t *metadataServiceImpl) Get(job, instance string) (metadataCache, error) {
	targetGroupMap := t.sm.TargetsAll()
	targetGroup, ok := targetGroupMap[job]

	if !ok {
		return nil, errors.New("unable to find a target group with job=" + job)
	}

	// from the same targetGroup, instance is not going to be duplicated
	for _, target := range targetGroup {
		if target.DiscoveredLabels().Get(savedScrapeInstanceLabel) == instance || target.DiscoveredLabels().Get(scrapeInstanceLabel) == instance {
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

// Return JobName and Instance based o metric label.
// job and instance are later used for getting metadata cache from scrape targets to determine metric type.
// All metrics in a batch are from same scrape target, we should only need first one.
// But loop all of them and returns error in cse our relabel hack is not working.
func getScrapeTargetInfo(pmb PrometheusMetricBatch) (job string, instance string, err error) {
	for _, pm := range pmb {
		job = pm.jobBeforeRelabel
		if job == "" {
			continue
		}
		instance = pm.instanceBeforeRelabel
		if instance == "" {
			continue
		}
		return job, instance, nil
	}
	return "", "", fmt.Errorf("job and/or instance not found from %d metrics job=%q instance=%q", len(pmb), job, instance)
}

// Decorate the Metrics with Metric Types.
// Filter out Summary, Histogram and untyped Metrics and adding logging.
func (mth *metricsTypeHandler) Handle(pmb PrometheusMetricBatch) (result PrometheusMetricBatch) {
	if len(pmb) == 0 {
		log.Printf("D! Skip empty batch")
		return nil
	}

	jobName, instanceId, err := getScrapeTargetInfo(pmb)
	if err != nil {
		log.Printf("E! Failed to get Job Name and Instance ID from scrape targetss %s", err)
		return nil
	}

	mc, err := mth.ms.Get(jobName, instanceId)
	if err != nil {
		log.Printf("E! metricsTypeHandler.mc.Get(jobName, instanceId) error. jobName: %s  instanceId: %s: %v", jobName, instanceId, err)
		// The Pod has been terminated when we are going to handle its Prometheus metrics in the channel
		// Drop the metrics directly
		return result
	}
	for _, pm := range pmb {
		// log for https://github.com/aws/amazon-cloudwatch-agent/issues/190
		if pm.metricNameBeforeRelabel != pm.metricName {
			log.Printf("D! metric name changed from %q to %q during relabel", pm.metricNameBeforeRelabel, pm.metricName)
		}
		// normalize the summary metric first, then if metric name == standardMetricName, it means it is not been normalized by summary
		// , then normalize the counter suffix if it failed to find metadata.
		standardMetricName := normalizeMetricName(pm.metricNameBeforeRelabel, histogramSummarySuffixes)
		mm, ok := mc.Metadata(standardMetricName)
		if !ok {
			if pm.metricName != standardMetricName {
				// perform a 2nd lookup with the original metric name
				// It could happen if non histogram/summary ends with one of those _count/_sum suffixes
				mm, ok = mc.Metadata(pm.metricNameBeforeRelabel)
			} else {
				// normalize the counter type suffixes, like "_total" suffix
				standardMetricName = normalizeMetricName(pm.metricNameBeforeRelabel, counterSuffixes)
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
