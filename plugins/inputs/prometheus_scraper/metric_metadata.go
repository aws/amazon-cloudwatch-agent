// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus_scraper

import (
	"context"
	"errors"
	"github.com/prometheus/prometheus/model/textparse"
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
	metadata scrape.MetricMetadataStore
}

func (m *mCache) Metadata(metricName string) (scrape.MetricMetadata, bool) {
	return m.metadata.GetMetadata(metricName)
}

func getMetadataCache(ctx context.Context) (MetadataCache, error) {
	metaStore, ok := scrape.MetricMetadataStoreFromContext(ctx)
	if !ok {
		return nil, errors.New("Unable to find MetricMetadataStore in context")
	}

	return &mCache{
		metadata: metaStore,
	}, nil
}

func metadataForMetric(metricName string, mc MetadataCache) *scrape.MetricMetadata {
	if metadata, ok := mc.Metadata(metricName); ok {
		return &metadata
	}
	// If we didn't find metadata with the original name, try with suffix trimmed
	normalizedName := normalizeMetricName(metricName)
	if metadata, ok := mc.Metadata(normalizedName); ok {
		return &metadata
	}

	return &scrape.MetricMetadata{
		Metric: metricName,
		Type:   textparse.MetricTypeUnknown,
	}
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

func isInternalMetric(metricName string) bool {
	//For each endpoint, Prometheus produces a set of internal metrics. See https://prometheus.io/docs/concepts/jobs_instances/
	if metricName == "up" || strings.HasPrefix(metricName, "scrape_") {
		return true
	}
	return false
}
