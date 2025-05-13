// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"errors"
	"fmt"
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
	fmt.Println("\n=== START: Handle PrometheusMetricBatch ===")
	fmt.Printf("Batch size: %d\n", len(pmb))

	if len(pmb) == 0 {
		fmt.Println("Skip empty batch")
		return nil
	}

	// Print initial batch details
	fmt.Println("\nInitial Batch Metrics:")
	for i, pm := range pmb {
		fmt.Printf("\nMetric #%d:\n", i+1)
		fmt.Printf("  Name: %s\n", pm.metricName)
		fmt.Printf("  Name Before Relabel: %s\n", pm.metricNameBeforeRelabel)
		fmt.Printf("  Job Before Relabel: %s\n", pm.jobBeforeRelabel)
		fmt.Printf("  Instance Before Relabel: %s\n", pm.instanceBeforeRelabel)
		fmt.Printf("  Value: %f\n", pm.metricValue)
		fmt.Printf("  Type: %s\n", pm.metricType)
		fmt.Printf("  Time: %d\n", pm.timeInMS)
		fmt.Printf("  Tags: %+v\n", pm.tags)
	}

	jobName, instanceId, err := getScrapeTargetInfo(pmb)
	if err != nil {
		fmt.Printf("ERROR: Failed to get Job Name and Instance ID: %v\n", err)
		return nil
	}
	fmt.Printf("\nScrape Target Info:\n  Job Name: %s\n  Instance ID: %s\n", jobName, instanceId)

	mc, err := mth.ms.Get(jobName, instanceId)
	if err != nil {
		fmt.Printf("ERROR: Failed to get metrics context for job %s, instance %s: %v\n",
			jobName, instanceId, err)
		return result
	}
	fmt.Println("Successfully got metrics context")

	fmt.Println("\nProcessing individual metrics:")
	for i, pm := range pmb {
		fmt.Printf("\nProcessing metric #%d: %s\n", i+1, pm.metricName)

		if pm.metricNameBeforeRelabel != pm.metricName {
			fmt.Printf("Metric name changed: %q -> %q\n",
				pm.metricNameBeforeRelabel, pm.metricName)
		}

		standardMetricName := normalizeMetricName(pm.metricNameBeforeRelabel, histogramSummarySuffixes)
		fmt.Printf("Normalized metric name (histogram/summary): %s\n", standardMetricName)

		mm, ok := mc.Metadata(standardMetricName)
		if !ok {
			fmt.Printf("Initial metadata lookup failed for %s\n", standardMetricName)
			if pm.metricName != standardMetricName {
				fmt.Printf("Attempting second lookup with original name: %s\n",
					pm.metricNameBeforeRelabel)
				mm, ok = mc.Metadata(pm.metricNameBeforeRelabel)
			} else {
				standardMetricName = normalizeMetricName(pm.metricNameBeforeRelabel, counterSuffixes)
				fmt.Printf("Attempting lookup with normalized counter name: %s\n",
					standardMetricName)
				mm, ok = mc.Metadata(standardMetricName)
			}
		}

		if ok {
			fmt.Printf("Found metadata. Setting type to: %s\n", mm.Type)
			pm.metricType = string(mm.Type)
			pm.tags[prometheusMetricTypeKey] = pm.metricType
		} else {
			if !isInternalMetric(pm.metricName) {
				fmt.Printf("WARNING: No metadata found for metric: %s\n", pm.metricName)
			}
		}

		if pm.metricType == "" && !isInternalMetric(pm.metricName) {
			fmt.Printf("ERROR: Empty metric type for non-internal metric: %s\n", pm.metricName)
			continue
		}

		result = append(result, pm)
		fmt.Printf("Successfully processed metric: %s\n", pm.metricName)
	}

	fmt.Printf("\nFinal result batch size: %d\n", len(result))
	fmt.Println("=== END: Handle PrometheusMetricBatch ===\n")

	return result
}
