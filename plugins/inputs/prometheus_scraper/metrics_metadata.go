// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus_scraper

import (
	"context"
	"errors"
	"github.com/prometheus/prometheus/model/textparse"
	"github.com/prometheus/prometheus/scrape"
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
