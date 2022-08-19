// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus_scraper

import (
	"context"
	"errors"
	"github.com/prometheus/prometheus/scrape"
	"strings"
)

const (
	prometheusMetricTypeKey = "prom_metric_type"
	metricsSuffixCount      = "_count"
	metricsSuffixBucket     = "_bucket"
	metricsSuffixSum        = "_sum"
	metricSuffixTotal       = "_total"
)

var (
	trimmableSuffixes = []string{metricsSuffixCount, metricsSuffixBucket, metricsSuffixSum, metricSuffixTotal}
)

type MetadataCache interface {
	Metadata(metricName string) (scrape.MetricMetadata, bool)
}

type mCache struct {
	metadataStore scrape.MetricMetadataStore
}

func (m *mCache) Metadata(metricName string) (scrape.MetricMetadata, bool) {
	return m.metadataStore.GetMetadata(metricName)
}

func getMetadataCache(ctx context.Context) (MetadataCache, error) {
	target, ok := scrape.TargetFromContext(ctx)
	if !ok {
		return nil, errors.New("unable to find target in context")
	}
	metaStore, ok := scrape.MetricMetadataStoreFromContext(ctx)
	if !ok {
		return nil, errors.New("unable to find MetricMetadataStore in context")
	}

	return &mCache{
		target:   target,
		metadata: metaStore,
	}, nil
}

func metadataForMetric(metricName string, mc MetadataCache) (*scrape.MetricMetadata, string) {
	if metadata, ok := mc.Metadata(metricName); ok {
		return &metadata, metricName
	}
	// If we didn't find metadata with the original name,
	// try with suffixes trimmed, in-case it is a "merged" metric type.
	normalizedName := normalizeMetricName(metricName)
	if metadata, ok := mc.Metadata(normalizedName); ok {
		if metadata.Type == textparse.MetricTypeCounter {
			return &metadata, metricName
		}
		return &metadata, normalizedName
	}
	// Otherwise, the metric is unknown
	return &scrape.MetricMetadata{
		Metric: metricName,
		Type:   textparse.MetricTypeUnknown,
	}, metricName
}

func isInternalMetric(metricName string) bool {
	// For each endpoint, Prometheus produces a set of internal metrics. See https://prometheus.io/docs/concepts/jobs_instances/
	if metricName == "up" || strings.HasPrefix(metricName, "scrape_") {
		return true
	}
	return false
}

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
