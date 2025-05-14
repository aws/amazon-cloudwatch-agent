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
	log.Println("\n=== START: Handle PrometheusMetricBatch ===")
	log.Printf("Batch size: %d\n", len(pmb))

	if len(pmb) == 0 {
		log.Println("Skip empty batch")
		return nil
	}

	// ... existing logging code ...

	jobName, instanceId, err := getScrapeTargetInfo(pmb)
	if err != nil {
		log.Printf("ERROR: Failed to get Job Name and Instance ID: %v\n", err)
		return nil
	}
	log.Printf("\nScrape Target Info:\n  Job Name: %s\n  Instance ID: %s\n", jobName, instanceId)

	mc, err := mth.ms.Get(jobName, instanceId)
	if err != nil {
		log.Printf("WARNING: Failed to get metrics context for job %s, instance %s: %v\n",
			jobName, instanceId, err)
		// Continue processing with fallback type detection
	}

	log.Println("\nProcessing individual metrics:")
	for i, pm := range pmb {
		log.Printf("\nProcessing metric #%d: %s\n", i+1, pm.metricName)

		if pm.metricNameBeforeRelabel != pm.metricName {
			log.Printf("Metric name changed: %q -> %q\n",
				pm.metricNameBeforeRelabel, pm.metricName)
		}

		// Try to get type from metadata first
		if mc != nil {
			standardMetricName := normalizeMetricName(pm.metricNameBeforeRelabel, histogramSummarySuffixes)
			log.Printf("Normalized metric name (histogram/summary): %s\n", standardMetricName)

			mm, ok := mc.Metadata(standardMetricName)
			if !ok {
				log.Printf("Initial metadata lookup failed for %s\n", standardMetricName)
				if pm.metricName != standardMetricName {
					mm, ok = mc.Metadata(pm.metricNameBeforeRelabel)
				} else {
					standardMetricName = normalizeMetricName(pm.metricNameBeforeRelabel, counterSuffixes)
					mm, ok = mc.Metadata(standardMetricName)
				}
			}

			if ok {
				log.Printf("Found metadata. Setting type to: %s\n", mm.Type)
				pm.metricType = string(mm.Type)
				pm.tags[prometheusMetricTypeKey] = pm.metricType
			}
		}

		// If no type was found from metadata, try to infer it from the metric name
		if pm.metricType == "" && !isInternalMetric(pm.metricName) {
			inferredType := inferMetricType(pm.metricName)
			if inferredType != "" {
				log.Printf("Inferred metric type from name: %s\n", inferredType)
				pm.metricType = inferredType
				pm.tags[prometheusMetricTypeKey] = inferredType
			} else {
				// Default to gauge if we can't determine the type
				log.Printf("Defaulting to gauge for metric: %s\n", pm.metricName)
				pm.metricType = string(v1.MetricTypeGauge)
				pm.tags[prometheusMetricTypeKey] = string(v1.MetricTypeGauge)
			}
		}

		result = append(result, pm)
		log.Printf("Successfully processed metric: %s with type: %s\n", pm.metricName, pm.metricType)
	}

	log.Printf("\nFinal result batch size: %d\n", len(result))
	log.Println("=== END: Handle PrometheusMetricBatch ===\n")

	return result
}

// Add this new function to infer metric type from name
func inferMetricType(metricName string) string {
	// Check for histogram suffixes
	if strings.HasSuffix(metricName, histogramBucketSuffix) ||
		(strings.HasSuffix(metricName, histogramSummaryCountSuffix) && strings.Contains(metricName, "bucket")) {
		return string(v1.MetricTypeHistogram)
	}

	// Check for summary suffixes
	if strings.HasSuffix(metricName, histogramSummaryCountSuffix) ||
		strings.HasSuffix(metricName, histogramSummarySumSuffix) {
		return string(v1.MetricTypeSummary)
	}

	// Check for counter suffix
	if strings.HasSuffix(metricName, counterSuffix) {
		return string(v1.MetricTypeCounter)
	}

	return ""
}
