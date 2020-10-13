package prometheus_scraper

import (
	"errors"
	"log"

	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/pkg/value"
	"github.com/prometheus/prometheus/storage"
)

const (
	prometheusLabelKeyMetricNameKey = "__name__"
	prometheusLabelMeticTypeKey     = "__metric_type__"
	prometheusMeticTypeKey          = "prom_metric_type"
)

type PrometheusMetricBatch []*PrometheusMetric

type PrometheusMetric struct {
	tags        map[string]string
	metricName  string
	metricValue float64
	metricType  string
	timeInMS    int64 // Unix time in milli-seconds
}

func (pm *PrometheusMetric) isCounter() bool {
	return pm.metricType == "counter"
}

func (pm *PrometheusMetric) isGauge() bool {
	return pm.metricType == "gauge"
}

func (pm *PrometheusMetric) isHistogram() bool {
	return pm.metricType == "histogram"
}

func (pm *PrometheusMetric) isSummary() bool {
	return pm.metricType == "summary"
}

func (pm *PrometheusMetric) isValueStale() bool {
	return value.IsStaleNaN(pm.metricValue)
}

// metricsReceiver implement interface Appender for prometheus scarper to append metrics
type metricsReceiver struct {
	pmbCh chan<- PrometheusMetricBatch
}

type metricAppender struct {
	receiver *metricsReceiver
	batch    PrometheusMetricBatch
}

//TODO
// func (mr *metricsReceiver) Appender() (storage.Appender, error) {
// 	return &metricAppender{receiver: mr, batch: PrometheusMetricBatch{}}, nil
// }

func (mr *metricsReceiver) Appender() storage.Appender {
	return &metricAppender{receiver: mr, batch: PrometheusMetricBatch{}}
}

func (mr *metricsReceiver) feed(batch PrometheusMetricBatch) error {
	select {
	case mr.pmbCh <- batch:
	default:
		log.Println("W! metric batch drop due to channel full")
	}
	return nil
}

func (ma *metricAppender) Add(ls labels.Labels, t int64, v float64) (uint64, error) {
	metricName := ""
	metricType := ""

	labelMap := make(map[string]string, len(ls))
	for _, l := range ls {
		if l.Name == prometheusLabelMeticTypeKey {
			metricType = l.Value
			labelMap[prometheusMeticTypeKey] = metricType
			continue
		}
		if l.Name == prometheusLabelKeyMetricNameKey {
			metricName = l.Value
			continue
		}
		labelMap[l.Name] = l.Value
	}

	if metricName == "" {
		// The error should never happen, print log here for debugging
		log.Println("E! receive invalid prometheus metric, metricName is missing")
		return uint64(0), errors.New("metricName of the times-series is missing")
	}

	pm := &PrometheusMetric{
		metricName:  metricName,
		metricValue: v,
		metricType:  metricType,
		timeInMS:    t,
	}

	delete(labelMap, prometheusLabelKeyMetricNameKey)
	delete(labelMap, prometheusLabelMeticTypeKey)
	pm.tags = labelMap

	ma.batch = append(ma.batch, pm)
	return uint64(0), nil
}

// func (ma *metricAppender) AddFast(ls labels.Labels, _ uint64, t int64, v float64) error {
// 	_, err := ma.Add(ls, t, v)
// 	return err
// }

//a dummy function to satisfy the interface for storage.Appender
func (ma *metricAppender) AddFast(ref uint64, t int64, v float64) error {
	return nil
}

func (ma *metricAppender) Commit() error {
	return ma.receiver.feed(ma.batch)
}

func (ma *metricAppender) Rollback() error {
	// wipe the batch
	ma.batch = PrometheusMetricBatch{}
	return nil
}
