// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus_scraper

import (
	"context"
	"errors"
	"fmt"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/textparse"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/storage"
	"log"
)

const prometheusMetricTypeKey = "prom_metric_type"

// metricsReceiver implement interface Appender for prometheus scarper to append metrics
type metricsReceiver struct {
	pmbCh chan<- PrometheusMetricBatch
}

type metricAppender struct {
	ctx        context.Context
	receiver   *metricsReceiver
	batch      PrometheusMetricBatch
	mc         MetadataCache
	isNewBatch bool
}

func (mr *metricsReceiver) Appender(ctx context.Context) storage.Appender {
	return &metricAppender{ctx: ctx, receiver: mr, batch: PrometheusMetricBatch{}, isNewBatch: true}
}

func (mr *metricsReceiver) feed(batch PrometheusMetricBatch) error {
	select {
	case mr.pmbCh <- batch:
	default:
		log.Println("W! metric batch drop due to channel full")
	}
	return nil
}

func (ma *metricAppender) Append(ref storage.SeriesRef, ls labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	select {
	case <-ma.ctx.Done():
		return 0, errors.New("Abort appending metrics to batch")
	default:
	}

	return 0, ma.AppendMetricToBatch(ls, t, v) //return 0 to indicate caching is not supported
}

func (ma *metricAppender) Commit() error {
	return ma.receiver.feed(ma.batch)
}

func (ma *metricAppender) Rollback() error {
	// wipe the batch
	ma.batch = PrometheusMetricBatch{}
	return nil
}

func (ma *metricAppender) AppendExemplar(ref storage.SeriesRef, ls labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	ma.Append(ref, ls, e.Ts, e.Value)
	return 0, nil
}

func (ma *metricAppender) AppendMetricToBatch(ls labels.Labels, metricCreateTime int64, metricValue float64) (err error) {
	// Each new scrape will create a context hold metadataStore. Therefore, the same context will be used
	// by all metrics in the same batch. So we only need to fetch the metadataStore once
	if ma.isNewBatch {
		metadataCache, err := getMetadataCache(ma.ctx)
		if err != nil {
			return err
		}
		ma.isNewBatch = false
		ma.mc = metadataCache
	}

	pm, err := ma.BuildPrometheusMetric(ls, metricCreateTime, metricValue)
	if err != nil {
		return err
	}
	
	// The internal metrics sometimes will return with type unknown and we would not consider it as valid metric (only support Gauge, Counter, Summary)
	// https://github.com/khanhntd/amazon-cloudwatch-agent/blob/master/plugins/inputs/prometheus_scraper/metrics_filter.go#L21-L48
	
	if pm == nil {
		return nil
	}
	
	ma.batch = append(ma.batch, pm)
	return nil
}

func (ma *metricAppender) BuildPrometheusMetric(ls labels.Labels, metricCreateTime int64, metricValue float64) (*PrometheusMetric, error) {
	metricName := ls.Get(model.MetricNameLabel)

	if metricName == "" {
		return nil, errors.New("metric name of the times-series is missing")
	}

	var metricTags map[string]string
	var metricMetadata *scrape.MetricMetadata

	if metricNameBeforeRelabel := ls.Get(savedScrapeNameLabel); metricNameBeforeRelabel != "" {
		metricTags = ls.WithoutLabels(model.MetricNameLabel, savedScrapeNameLabel).Map()
		metricMetadata = metadataForMetric(metricNameBeforeRelabel, ma.mc)
	} else {
		metricTags = ls.WithoutLabels(model.MetricNameLabel).Map()
		metricMetadata = metadataForMetric(metricName, ma.mc)
	}

	if metricMetadata.Type == textparse.MetricTypeUnknown {
		if isInternalMetric(metricName) {
			return nil, nil
		}
		return nil, fmt.Errorf("unknown metric type for metric %s", metricName)
	}

	metricType := string(metricMetadata.Type)
	metricTags[prometheusMetricTypeKey] = metricType

	return &PrometheusMetric{
		metricName:  metricName,
		metricType:  metricType,
		metricValue: metricValue,
		timeInMS:    metricCreateTime,
		tags:        metricTags,
	}, nil
}
