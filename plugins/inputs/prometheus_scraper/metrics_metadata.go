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
	metricsSuffixCount  = "_count"
	metricsSuffixBucket = "_bucket"
	metricsSuffixSum    = "_sum"
	metricSuffixTotal   = "_total"
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
	metadata, ok := scrape.MetricMetadataStoreFromContext(ctx)
	if !ok {
		return nil, errors.New("Unable to find MetricMetadataStore in context")
	}

	return &mCache{
		metadata: metadata,
	}, nil
}

func metadataForMetric(metricName string, mc MetadataCache) *scrape.MetricMetadata {
	// Two ways to get metric type through metadataStore:
	// * Use instance and job to get metadataStore. If customer relabel job or instance, it will fail
	// * Use Context that holds metadataStore which is created within each scrape loop https://github.com/prometheus/prometheus/blob/main/scrape/scrape.go#L1154-L1161
	// The former is being restricted by relabel job and relabel instance, but that does not apply to the latter
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
