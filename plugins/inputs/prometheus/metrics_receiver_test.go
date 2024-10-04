// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	kitlog "github.com/go-kit/log"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func Test_metricAppender_Add_BadMetricName(t *testing.T) {
	var ma metricAppender
	var ts int64 = 10
	var v = 10.0

	ls := []labels.Label{
		{Name: "name_a", Value: "value_a"},
		{Name: "name_b", Value: "value_b"},
	}

	r, err := ma.Append(0, ls, ts, v)
	assert.Equal(t, storage.SeriesRef(0), r)
	assert.Equal(t, "metricName of the times-series is missing", err.Error())
}

func Test_metricAppender_Add(t *testing.T) {
	mr := metricsReceiver{}
	ma := mr.Appender(nil)
	var ts int64 = 10
	var v = 10.0
	ls := []labels.Label{
		{Name: "__name__", Value: "metric_name"},
		{Name: "tag_a", Value: "a"},
	}

	ref, err := ma.Append(0, ls, ts, v)
	assert.Equal(t, ref, storage.SeriesRef(0))
	assert.Nil(t, err)
	mac, _ := ma.(*metricAppender)
	assert.Equal(t, 1, len(mac.batch))

	expected := PrometheusMetric{
		metricName:  "metric_name",
		metricValue: v,
		metricType:  "",
		timeInMS:    ts,
		tags:        map[string]string{"tag_a": "a"},
	}
	assert.Equal(t, expected, *mac.batch[0])
}

func Test_metricAppender_isValueStale(t *testing.T) {
	nonStaleValue := PrometheusMetric{
		metricValue: 10.0,
	}
	assert.True(t, nonStaleValue.isValueValid())
}

func Test_metricAppender_Rollback(t *testing.T) {
	mr := metricsReceiver{}
	ma := mr.Appender(nil)
	var ts int64 = 10
	var v = 10.0
	ls := []labels.Label{
		{Name: "__name__", Value: "metric_name"},
		{Name: "tag_a", Value: "a"},
	}

	ref, err := ma.Append(0, ls, ts, v)
	assert.Equal(t, ref, storage.SeriesRef(0))
	assert.Nil(t, err)
	mac, _ := ma.(*metricAppender)
	assert.Equal(t, 1, len(mac.batch))

	ma.Rollback()
	assert.Equal(t, 0, len(mac.batch))
}

func Test_metricAppender_Commit(t *testing.T) {
	mbCh := make(chan PrometheusMetricBatch, 3)
	mr := metricsReceiver{pmbCh: mbCh}
	ma := mr.Appender(nil)
	var ts int64 = 10
	var v = 10.0
	ls := []labels.Label{
		{Name: "__name__", Value: "metric_name"},
		{Name: "tag_a", Value: "a"},
	}

	ref, err := ma.Append(0, ls, ts, v)
	assert.Equal(t, ref, storage.SeriesRef(0))
	assert.Nil(t, err)
	mac, _ := ma.(*metricAppender)
	assert.Equal(t, 1, len(mac.batch))
	err = ma.Commit()
	assert.Equal(t, nil, err)

	pmb := <-mbCh
	assert.Equal(t, 1, len(pmb))

	expected := PrometheusMetric{
		metricName:  "metric_name",
		metricValue: v,
		metricType:  "",
		timeInMS:    ts,
		tags:        map[string]string{"tag_a": "a"},
	}
	assert.Equal(t, expected, *pmb[0])
}

func Test_loadConfigFromFile(t *testing.T) {
	os.Setenv("POD_NAME", "collector-1")
	configFile := filepath.Join("testdata", "target_allocator.yaml")
	logger := kitlog.NewLogfmtLogger(os.Stdout)
	var reloader = func(cfg *config.Config) error {
		logger.Log("reloaded")
		return nil
	}
	err := reloadConfig(configFile, logger, true, reloader)
	assert.NoError(t, err)
}

func Test_TA_Labels(t *testing.T) {
	mbCh := make(chan PrometheusMetricBatch, 3)
	mr := metricsReceiver{pmbCh: mbCh}
	ma := mr.Appender(nil)
	var ts int64 = 10
	var v = 10.0
	ls := []labels.Label{
		{Name: "__address__", Value: "192.168.20.37:8080"},
		{Name: "__meta_kubernetes_namespace", Value: "default"},
		{Name: "__meta_kubernetes_pod_container_id", Value: "containerd://b3a3dd8e4630a0c99fd65f4cffb8f2524394ec3e86dce2e94dced7dce082ea94"},
		{Name: "__meta_kubernetes_pod_container_image", Value: "registry.k8s.io/kube-state-metrics/kube-state-metrics:v2.12.0"},
		{Name: "__meta_kubernetes_pod_container_init", Value: "false"},
		{Name: "__meta_kubernetes_pod_container_name", Value: "kube-state-metrics"},
		{Name: "__meta_kubernetes_pod_container_port_name", Value: "http"},
		{Name: "__meta_kubernetes_pod_container_port_number", Value: "8080"},
		{Name: "__meta_kubernetes_pod_container_port_protocol", Value: "TCP"},
		{Name: "__meta_kubernetes_pod_controller_kind", Value: "ReplicaSet"},
		{Name: "__meta_kubernetes_pod_controller_name", Value: "example-kube-state-metrics-7446cc6b96"},
		{Name: "__meta_kubernetes_pod_host_ip", Value: "192.168.0.41"},
		{Name: "__meta_kubernetes_pod_ip", Value: "192.168.20.37"},
	}

	ref, err := ma.Append(0, ls, ts, v)
	assert.Equal(t, ref, storage.SeriesRef(0))
	assert.Nil(t, err)
	mac, _ := ma.(*metricAppender)
	assert.Equal(t, 1, len(mac.batch))

	expected := PrometheusMetric{
		metricName:  "metric_name",
		metricValue: v,
		metricType:  "",
		timeInMS:    ts,
		tags:        map[string]string{"tag_a": "a"},
	}
	assert.Equal(t, expected, *mac.batch[0])
}
