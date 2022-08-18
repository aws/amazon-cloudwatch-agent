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
	metricName := ""

	labelMap := make(map[string]string, len(ls))
	for _, l := range ls {
		if l.Name == model.MetricNameLabel {
			metricName = l.Value
			continue
		}
		labelMap[l.Name] = l.Value
	}

	if metricName == "" {
		// The error should never happen, print log here for debugging
		log.Println("E! receive invalid prometheus metric, metricName is missing")
		return 0, errors.New("metricName of the times-series is missing")
	}

	pm := &PrometheusMetric{
		metricName:  metricName,
		metricValue: v,
		timeInMS:    t,
	}

	pm.tags = labelMap
	ma.batch = append(ma.batch, pm)
	return 0, nil //return 0 to indicate caching is not supported
}

func (ma *metricAppender) Commit() error {
	return ma.receiver.feed(ma.batch)
}

func (ma *metricAppender) Rollback() error {
	// wipe the batch
	ma.batch = PrometheusMetricBatch{}
	return nil
}

func (ma *metricAppender) AppendExemplar(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	ma.Append(ref, l, e.Ts, e.Value)
	return 0, nil
}
