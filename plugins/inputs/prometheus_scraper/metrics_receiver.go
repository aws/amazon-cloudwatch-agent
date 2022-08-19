// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus_scraper

import (
	"context"
	"errors"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/value"
	"log"
	"math"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/textparse"
	"github.com/prometheus/prometheus/storage"
)

type PrometheusMetricBatch []*PrometheusMetric

type PrometheusMetric struct {
	tags        map[string]string
	metricName  string
	metricValue float64
	metricType  string
	timeInMS    int64 // Unix time in milli-seconds
}

func (pm *PrometheusMetric) isValueValid() bool {
	//treat NaN and +/-Inf values as invalid as emf log doesn't support them
	return !value.IsStaleNaN(pm.metricValue) && !math.IsNaN(pm.metricValue) && !math.IsInf(pm.metricValue, 0)
}

// metricsReceiver implement interface Appender for prometheus scarper to append metrics
type metricsReceiver struct {
	pmbCh chan<- PrometheusMetricBatch
}

type metricAppender struct {
	ctx      context.Context
	receiver *metricsReceiver
	batch    PrometheusMetricBatch
	mc       MetadataCache
}

func (mr *metricsReceiver) Appender(ctx context.Context) storage.Appender {
	return &metricAppender{ctx: ctx, receiver: mr, batch: PrometheusMetricBatch{}}
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

	ma.batch = append(ma.batch, pm)
	return 0, ma.BuildPrometheusMetric(ls, t, v) //return 0 to indicate caching is not supported
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

func (ma *metricAppender) BuildPrometheusMetric(ls labels.Labels, t int64, v float64) (err error) {
	metadataCache, err := getMetadataCache(ma.ctx)
	if err != nil {
		return err
	}

	metricName := ls.Get(model.MetricNameLabel)
	metricTags := ls.WithoutLabels(model.MetricNameLabel).Map()
	metricMetadata := metadataForMetric(metricName, metadataCache)
	metricType := string(metricMetadata.Type)
	metricTags[prometheusMetricTypeKey] = metricType

	if metricType == textparse.MetricTypeUnknown && !isInternalMetric(metricName) {
		return errors.Errorf("Unknown metric type for metric %s", metricName)
	}

	pm := &PrometheusMetric{
		metricName:  metricName,
		metricType:  metricType,
		metricValue: v,
		timeInMS:    t,
		tags:        metricTags,
	}

	ma.batch = append(ma.batch, pm)
	return nil
}
