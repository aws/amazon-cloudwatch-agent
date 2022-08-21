// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus_scraper

import (
	"context"
	"testing"
	"time"
	"github.com/stretchr/testify/assert"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/textparse"
)

func Test_transaction_pdata(t *testing.T) {
	// discoveredLabels contain labels prior to any processing
	discoveredLabels := labels.New(
		labels.Label{
			Name:  model.AddressLabel,
			Value: "address:8080",
		},
		labels.Label{
			Name:  model.MetricNameLabel,
			Value: "foo",
		},
		labels.Label{
			Name:  model.SchemeLabel,
			Value: "http",
		},
	)
	// processedLabels contain label values after processing (e.g. relabeling)
	processedLabels := labels.New(
		labels.Label{
			Name:  model.InstanceLabel,
			Value: "localhost:8080",
		},
	)
	
	assert := assert.New(t)
	target := scrape.NewTarget(processedLabels, discoveredLabels, nil)
	scrapeCtx := scrape.ContextWithTarget(context.Background(), target)
	scrapeCtx = scrape.ContextWithMetricMetadataStore(scrapeCtx, metricMetadataStore{})

	goodLabels := labels.Labels([]labels.Label{
		{Name: "instance", Value: "localhost:8080"},
		{Name: "job", Value: "test"},
		{Name: "__name__", Value: "foo"}},
	)
	
	t.Run("Add One Good", func(t *testing.T) {
		mr := &metricsReceiver{pmbCh: make(chan PrometheusMetricBatch, 10000)}
		ma := &metricAppender{ctx: scrapeCtx, receiver: mr, batch: PrometheusMetricBatch{}, isNewBatch: true}
		_, err := ma.Append(0, goodLabels, time.Now().Unix()*1000, 1.0)
		assert.NoError(err)
	})

	
}

type metricMetadataStore struct{}

func (metricMetadataStore) ListMetadata() []scrape.MetricMetadata { return nil }
func (metricMetadataStore) GetMetadata(metric string) (scrape.MetricMetadata, bool) {
	return scrape.MetricMetadata{
			Metric: "go_threads",
			Type:   textparse.MetricTypeGauge,
			Help:   "Number of OS threads created",
			Unit:   "",
	}, false
}
func (metricMetadataStore) SizeMetadata() int   { return 0 }
func (metricMetadataStore) LengthMetadata() int { return 0 }
