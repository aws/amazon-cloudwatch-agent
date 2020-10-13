package prometheus_scraper

import (
	"testing"

	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/stretchr/testify/assert"
)

func Test_metricAppender_Add_BadMetricName(t *testing.T) {
	var ma metricAppender
	var ts int64 = 10
	var v float64 = 10.0

	ls := []labels.Label{
		{Name: "name_a", Value: "value_a"},
		{Name: "name_b", Value: "value_b"},
	}

	r, e := ma.Add(ls, ts, v)
	assert.Equal(t, uint64(0), r)
	assert.Equal(t, "metricName of the times-series is missing", e.Error())
}

//disable the test temporarily
// func Test_metricAppender_AddFast(t *testing.T) {
// 	mr := metricsReceiver{}
// 	ma, _ := mr.Appender()
// 	var ts int64 = 10
// 	var v float64 = 10.0
// 	ls := []labels.Label{
// 		{Name: "__name__", Value: "metric_name"},
// 		{Name: "__metric_type__", Value: "gauge"},
// 		{Name: "tag_a", Value: "a"},
// 	}

// 	e := ma.AddFast(ls, uint64(10), ts, v)
// 	assert.Nil(t, e)
// 	mac, _ := ma.(*metricAppender)
// 	assert.Equal(t, 1, len(mac.batch))

// 	expected := PrometheusMetric{
// 		metricName:  "metric_name",
// 		metricValue: v,
// 		metricType:  "gauge",
// 		timeInMS:    ts,
// 		tags:        map[string]string{"tag_a": "a", "prom_metric_type": "gauge"},
// 	}
// 	assert.Equal(t, expected, *mac.batch[0])
// }

func Test_metricAppender_isValueStale(t *testing.T) {
	nonStaleValue := PrometheusMetric{
		metricValue: 10.0,
	}
	assert.False(t, nonStaleValue.isValueStale())
}

//TODO enable the test
// func Test_metricAppender_Rollback(t *testing.T) {
// 	mr := metricsReceiver{}
// 	ma, _ := mr.Appender()
// 	var ts int64 = 10
// 	var v float64 = 10.0
// 	ls := []labels.Label{
// 		{Name: "__name__", Value: "metric_name"},
// 		{Name: "__metric_type__", Value: "gauge"},
// 		{Name: "tag_a", Value: "a"},
// 	}

// 	e := ma.AddFast(ls, uint64(10), ts, v)
// 	assert.Nil(t, e)
// 	mac, _ := ma.(*metricAppender)
// 	assert.Equal(t, 1, len(mac.batch))

// 	ma.Rollback()
// 	assert.Equal(t, 0, len(mac.batch))
// }

//TODO: enable the test
// func Test_metricAppender_Commit(t *testing.T) {
// 	mbCh := make(chan PrometheusMetricBatch, 3)
// 	mr := metricsReceiver{pmbCh: mbCh}
// 	ma, _ := mr.Appender()
// 	var ts int64 = 10
// 	var v float64 = 10.0
// 	ls := []labels.Label{
// 		{Name: "__name__", Value: "metric_name"},
// 		{Name: "__metric_type__", Value: "gauge"},
// 		{Name: "tag_a", Value: "a"},
// 	}

// 	e := ma.AddFast(ls, uint64(10), ts, v)
// 	assert.Nil(t, e)
// 	mac, _ := ma.(*metricAppender)
// 	assert.Equal(t, 1, len(mac.batch))
// 	err := ma.Commit()
// 	assert.Equal(t, nil, err)

// 	pmb := <-mbCh
// 	assert.Equal(t, 1, len(pmb))

// 	expected := PrometheusMetric{
// 		metricName:  "metric_name",
// 		metricValue: v,
// 		metricType:  "gauge",
// 		timeInMS:    ts,
// 		tags:        map[string]string{"tag_a": "a", "prom_metric_type": "gauge"},
// 	}
// 	assert.Equal(t, expected, *pmb[0])
// }
