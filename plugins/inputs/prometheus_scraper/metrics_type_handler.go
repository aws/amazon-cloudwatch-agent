// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus_scraper

import (
	"context"
	"errors"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/textparse"
	"log"
	"strings"

	"github.com/prometheus/prometheus/scrape"
)

const (
	prometheusMetricTypeKey = "prom_metric_type"

	metricsSuffixCount  = "_count"
	metricsSuffixBucket = "_bucket"
	metricsSuffixSum    = "_sum"
	metricSuffixTotal   = "_total"
)

var (
	trimmableSuffixes = []string{metricsSuffixCount, metricsSuffixBucket, metricsSuffixSum, metricSuffixTotal}
)

// Get the metric name in the TYPE comments for Summary and Histogram
// e.g # TYPE nginx_ingress_controller_request_duration_seconds histogram
//     # TYPE nginx_ingress_controller_ingress_upstream_latency_seconds summary

func normalizeMetricName(name string) string {
	for _, s := range trimmableSuffixes {
		if strings.HasSuffix(name, s) && name != s {
			return strings.TrimSuffix(name, s)
		}
	}
	return name
}

func getMetadataForMetric(metricName string, mc metadataCache) *scrape.MetricMetadata {
	if metadata, ok := mc.Metadata(metricName); ok {
		return &metadata
	}

	normalizedMetricName := normalizeMetricName(metricName)
	if metadata, ok := mc.Metadata(normalizedMetricName); ok {
		return &metadata
	}

	return nil
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

// Adapter to prometheus scrape.Target
type metadataCache interface {
	Metadata(metricName string) (scrape.MetricMetadata, bool)
}

// adapter to get metadata from scrape.Target
type mCache struct {
	target   *scrape.Target
	metadata scrape.MetricMetadataStore
}

func (m *mCache) Metadata(metricName string) (scrape.MetricMetadata, bool) {
	return m.metadata.GetMetadata(metricName)
}

// Adapter to ScrapeManager to retrieve the cache by job and instance
type metadataService interface {
	Get(ctx context.Context) (metadataCache, error)
}

type metadataServiceImpl struct {
	sm ScrapeManager
}

// job and instance MUST be using value before relabel
func (t *metadataServiceImpl) Get(ctx context.Context) (metadataCache, error) {

	target, ok := scrape.TargetFromContext(ctx)
	if !ok {
		return nil, errors.New("Unable to find a target group with job=" + job)
	}

	metaStore, ok := scrape.MetricMetadataStoreFromContext(ctx)
	if !ok {
		return nil, errors.New("unable to find MetricMetadataStore in context")
	}

	return &mCache{target, metaStore}, nil
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
func getScrapeTargetInfo(pmb PrometheusMetricBatch) (string, string, error) {
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

	mc, err := mth.ms.Get(jobName)
	if err != nil {
		log.Printf("E! metricsTypeHandler.mc.Get(jobName, instanceId) error. jobName: %s  instanceId: %s: %v", jobName, instanceId, err)
		// The Pod has been terminated when we are going to handle its Prometheus metrics in the channel
		// Drop the metrics directly
		return result
	}
	for _, pm := range pmb {

		if metadata := getMetadataForMetric(pm.metricName, mc); metadata != nil {
			pm.metricType = string(metadata.Type)
			pm.tags[prometheusMetricTypeKey] = pm.metricType
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
