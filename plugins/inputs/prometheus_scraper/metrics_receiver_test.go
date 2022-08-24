// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus_scraper

import (
	"context"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/textparse"
	"github.com/prometheus/prometheus/scrape"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

var testMetadata = map[string]scrape.MetricMetadata{
	"counter_test": {Metric: "counter_test", Type: textparse.MetricTypeCounter, Help: "", Unit: ""},
	"gauge_test":   {Metric: "gauge_test", Type: textparse.MetricTypeGauge, Help: "", Unit: ""},
	"hist_test":    {Metric: "hist_test", Type: textparse.MetricTypeHistogram, Help: "", Unit: ""},
}

type mockMetadataCache struct {
	data map[string]scrape.MetricMetadata
}

func newMockMetadataCache(data map[string]scrape.MetricMetadata) *mockMetadataCache {
	return &mockMetadataCache{data: data}
}

func (m *mockMetadataCache) Metadata(metricName string) (scrape.MetricMetadata, bool) {
	mm, ok := m.data[metricName]
	return mm, ok
}

func TestMetricAppender(t *testing.T) {
	assert := assert.New(t)

	t.Run("Valid metric name and type", func(t *testing.T) {
		labels := labels.Labels([]labels.Label{
			{Name: "instance", Value: "localhost:8080"},
			{Name: "job", Value: "test"},
			{Name: "__name__", Value: "counter_test"}},
		)

		mr := &metricsReceiver{pmbCh: make(chan PrometheusMetricBatch, 1)}
		ma := &metricAppender{ctx: context.Background(), receiver: mr, batch: PrometheusMetricBatch{}, isNewBatch: false, mc: newMockMetadataCache(testMetadata)}

		_, err := ma.Append(0, labels, time.Now().Unix()*1000, 1.0)
		assert.NoError(err)

		err = ma.Commit()
		assert.NoError(err)
		assert.Len(ma.batch, 1)
		assert.Len(mr.pmbCh, 1)
	})

	t.Run("invalid metric name in cache", func(t *testing.T) {
		labels := labels.Labels([]labels.Label{
			{Name: "instance", Value: "localhost:8080"},
			{Name: "job", Value: "test"},
			{Name: "__name__", Value: "foo_test"}},
		)
		mr := &metricsReceiver{pmbCh: make(chan PrometheusMetricBatch, 1)}
		ma := &metricAppender{ctx: context.Background(), receiver: mr, batch: PrometheusMetricBatch{}, isNewBatch: false, mc: newMockMetadataCache(testMetadata)}

		_, err := ma.Append(0, labels, time.Now().Unix()*1000, 1.0)
		assert.Error(err)

		err = ma.Rollback()
		assert.NoError(err)
		assert.Len(ma.batch, 0)
		assert.Len(mr.pmbCh, 0)
	})
}

func TestBuildPrometheusMetric(t *testing.T) {
	assert := assert.New(t)

	t.Run("valid metric", func(t *testing.T) {
		labels := labels.Labels([]labels.Label{
			{Name: "instance", Value: "localhost:8080"},
			{Name: "job", Value: "test"},
			{Name: "__name__", Value: "hist_test"}},
		)

		metricValue := 1.0
		metricCreateTime := time.Now().Unix() * 1000
		metricTags := labels.WithoutLabels(model.MetricNameLabel).Map()
		metricType := string(textparse.MetricTypeHistogram)
		metricTags[prometheusMetricTypeKey] = metricType

		mr := &metricsReceiver{pmbCh: make(chan PrometheusMetricBatch, 1)}
		ma := &metricAppender{ctx: context.Background(), receiver: mr, batch: PrometheusMetricBatch{}, isNewBatch: false, mc: newMockMetadataCache(testMetadata)}

		pm, err := ma.BuildPrometheusMetric(labels, metricCreateTime, metricValue)

		assert.NoError(err)
		assert.Equal(pm.metricName, labels.Get(model.MetricNameLabel))
		assert.Equal(pm.metricType, metricType)
		assert.Equal(pm.metricValue, metricValue)
		assert.Equal(pm.timeInMS, metricCreateTime)
		assert.Equal(pm.tags, metricTags)
	})

	t.Run("empty metric name", func(t *testing.T) {
		labels := labels.Labels([]labels.Label{
			{Name: "instance", Value: "localhost:8080"},
			{Name: "job", Value: "test"},
			{Name: "__name__", Value: ""}},
		)

		mr := &metricsReceiver{pmbCh: make(chan PrometheusMetricBatch, 1)}
		ma := &metricAppender{ctx: context.Background(), receiver: mr, batch: PrometheusMetricBatch{}, isNewBatch: false, mc: newMockMetadataCache(testMetadata)}

		metricValue := 1.0
		metricCreateTime := time.Now().Unix() * 1000

		_, err := ma.BuildPrometheusMetric(labels, metricCreateTime, metricValue)
		assert.EqualError(err, "metric name of the times-series is missing")
	})

	t.Run("non-internal metric with unknown metric type", func(t *testing.T) {
		labels := labels.Labels([]labels.Label{
			{Name: "instance", Value: "localhost:8080"},
			{Name: "job", Value: "test"},
			{Name: "__name__", Value: "foo_test"}},
		)

		mr := &metricsReceiver{pmbCh: make(chan PrometheusMetricBatch, 1)}
		ma := &metricAppender{ctx: context.Background(), receiver: mr, batch: PrometheusMetricBatch{}, isNewBatch: false, mc: newMockMetadataCache(testMetadata)}

		metricValue := 1.0
		metricCreateTime := time.Now().Unix() * 1000

		_, err := ma.BuildPrometheusMetric(labels, metricCreateTime, metricValue)
		assert.Contains(err.Error(), "unknown metric type for metric")
	})

	t.Run("internal metric with unknown metric type", func(t *testing.T) {
		labels := labels.Labels([]labels.Label{
			{Name: "instance", Value: "localhost:8080"},
			{Name: "job", Value: "test"},
			{Name: "__name__", Value: "up"}},
		)

		metricValue := 1.0
		metricCreateTime := time.Now().Unix() * 1000
		metricTags := labels.WithoutLabels(model.MetricNameLabel).Map()
		metricType := string(textparse.MetricTypeUnknown)
		metricTags[prometheusMetricTypeKey] = metricType
		
		mr := &metricsReceiver{pmbCh: make(chan PrometheusMetricBatch, 1)}
		ma := &metricAppender{ctx: context.Background(), receiver: mr, batch: PrometheusMetricBatch{}, isNewBatch: false, mc: newMockMetadataCache(testMetadata)}

		pm, err := ma.BuildPrometheusMetric(labels, metricCreateTime, metricValue)

		assert.NoError(err)
		assert.Nil(pm)
	})
}
