// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus_scraper

import (
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/pkg/textparse"
	"net/url"

	"github.com/prometheus/prometheus/scrape"
	"github.com/stretchr/testify/assert"
	"testing"
)

type mockMetricMetadataStore struct {
	MetricList []string
	Type       textparse.MetricType
	Help       string
	Unit       string
}

func (mStore *mockMetricMetadataStore) GetMetadata(metric string) (scrape.MetricMetadata, bool) {
	found := false
	for _, m := range mStore.MetricList {
		if m == metric {
			found = true
			break
		}
	}

	if !found {
		return scrape.MetricMetadata{}, false
	}

	return scrape.MetricMetadata{
		Metric: metric,
		Type:   mStore.Type,
		Help:   mStore.Help,
		Unit:   mStore.Unit,
	}, true
}

// dummy function to satisfy the interface
func (mStore *mockMetricMetadataStore) ListMetadata() []scrape.MetricMetadata {
	return make([]scrape.MetricMetadata, 0, 0)
}

// dummy function to satisfy the interface
func (mStore *mockMetricMetadataStore) SizeMetadata() (s int) {
	return 0
}

// dummy function to satisfy the interface
func (mStore *mockMetricMetadataStore) LengthMetadata() int {
	return 0
}

type mockScrapeManager struct{}

func (ms *mockScrapeManager) TargetsAll() map[string][]*scrape.Target {
	targetMap := make(map[string][]*scrape.Target)

	//add a target
	params := url.Values{
		"abc": []string{"foo", "bar", "baz"},
		"xyz": []string{"www"},
	}
	labels1 := labels.FromMap(map[string]string{
		model.JobLabel:      "job1",
		model.InstanceLabel: "instance1",
	})
	target1 := scrape.NewTarget(labels1, labels1, params)
	mStore1 := &mockMetricMetadataStore{
		MetricList: []string{"m1", "m2", "m4"},
		Type:       textparse.MetricTypeCounter,
		Help:       "",
		Unit:       "",
	}
	target1.SetMetadataStore(mStore1)
	targetMap["job1"] = []*scrape.Target{target1}

	//add a target whose job name has been replaced (e.g. job2 -> job2_replaced)
	params2 := url.Values{
		"abc": []string{"foo", "bar", "foobar"},
		"xyz": []string{"ooo"},
	}
	labels2 := labels.FromMap(map[string]string{
		model.JobLabel:      "job2_replaced",
		model.InstanceLabel: "instance2",
	})
	target2 := scrape.NewTarget(labels2, labels2, params2)
	mStore2 := &mockMetricMetadataStore{
		MetricList: []string{"m1", "m2"},
		Type:       textparse.MetricTypeGauge,
		Help:       "",
		Unit:       "",
	}
	target2.SetMetadataStore(mStore2)
	targetMap["job2"] = []*scrape.Target{target2}

	return targetMap
}

func TestMetadataServiceImpl_GetWithUnknownJobnameInstance(t *testing.T) {
	metricsTypeHandler := NewMetricsTypeHandler()
	metricsTypeHandler.SetScrapeManager(&mockScrapeManager{})
	mCache, err := metricsTypeHandler.ms.Get("job_unknown", "instance_unknown")
	assert.Equal(t, mCache, nil)
	assert.EqualError(t, err, "unable to find a target group with job=job_unknown")

	mCache, err = metricsTypeHandler.ms.Get("job1", "instance_unknown")
	assert.Equal(t, mCache, nil)
	assert.EqualError(t, err, "unable to find a target with job=job1, and instance=instance_unknown")

	mCache, err = metricsTypeHandler.ms.Get("job2_replaced", "instance_unknown")
	assert.Equal(t, mCache, nil)
	assert.EqualError(t, err, "unable to find a target with job=job2_replaced, and instance=instance_unknown")
}

func TestMetadataServiceImpl_GetWithOriginalJobname(t *testing.T) {
	metricsTypeHandler := NewMetricsTypeHandler()
	metricsTypeHandler.SetScrapeManager(&mockScrapeManager{})
	mCache, err := metricsTypeHandler.ms.Get("job_unknown", "instance_unknown")
	assert.Equal(t, mCache, nil)
	assert.EqualError(t, err, "unable to find a target group with job=job_unknown")

	mCache, err = metricsTypeHandler.ms.Get("job1", "instance1")
	expectedMetricMetadata := scrape.MetricMetadata{
		Metric: "m1",
		Type:   textparse.MetricTypeCounter,
		Help:   "",
		Unit:   "",
	}
	metricMetadata, ok := mCache.Metadata("m1")
	assert.Equal(t, ok, true)
	assert.Equal(t, expectedMetricMetadata, metricMetadata)

	expectedMetricMetadata = scrape.MetricMetadata{
		Metric: "m2",
		Type:   textparse.MetricTypeCounter,
		Help:   "",
		Unit:   "",
	}
	metricMetadata, ok = mCache.Metadata("m2")
	assert.Equal(t, ok, true)
	assert.Equal(t, expectedMetricMetadata, metricMetadata)

	metricMetadata, ok = mCache.Metadata("m3")
	assert.Equal(t, ok, false)
}

func TestMetadataServiceImpl_GetWithReplacedJobname(t *testing.T) {
	metricsTypeHandler := NewMetricsTypeHandler()
	metricsTypeHandler.SetScrapeManager(&mockScrapeManager{})
	mCache, err := metricsTypeHandler.ms.Get("job2_replaced", "instance2")
	assert.Equal(t, err, nil)
	expectedMetricMetadata := scrape.MetricMetadata{
		Metric: "m1",
		Type:   textparse.MetricTypeGauge,
		Help:   "",
		Unit:   "",
	}
	metricMetadata, ok := mCache.Metadata("m1")
	assert.Equal(t, ok, true)
	assert.Equal(t, expectedMetricMetadata, metricMetadata)

	expectedMetricMetadata = scrape.MetricMetadata{
		Metric: "m2",
		Type:   textparse.MetricTypeGauge,
		Help:   "",
		Unit:   "",
	}
	metricMetadata, ok = mCache.Metadata("m2")
	assert.Equal(t, ok, true)
	assert.Equal(t, expectedMetricMetadata, metricMetadata)

	metricMetadata, ok = mCache.Metadata("m4")
	assert.Equal(t, ok, false)
}

func TestNewMetricsTypeHandler_HandleWithUnknownTarget(t *testing.T) {
	metricsTypeHandler := NewMetricsTypeHandler()
	metricsTypeHandler.SetScrapeManager(&mockScrapeManager{})
	pmb := make(PrometheusMetricBatch, 0)
	pmb = append(pmb,
		&PrometheusMetric{
			metricName: "m1",
			tags:       map[string]string{"job": "job_unknown", "instance": "instance_unknown"},
		},
		&PrometheusMetric{
			metricName: "m2",
			tags:       map[string]string{"job": "job_unknown", "instance": "instance_unknown"},
		})

	result := metricsTypeHandler.Handle(pmb)
	assert.Equal(t, 0, len(result))
}

func TestNewMetricsTypeHandler_HandleWithNormalTarget(t *testing.T) {
	metricsTypeHandler := NewMetricsTypeHandler()
	metricsTypeHandler.SetScrapeManager(&mockScrapeManager{})
	pmb := make(PrometheusMetricBatch, 0)
	pmb = append(pmb,
		&PrometheusMetric{
			metricName: "m3",
			tags:       map[string]string{"job": "job1", "instance": "instance1"},
		},
		&PrometheusMetric{
			metricName: "m1",
			tags:       map[string]string{"job": "job1", "instance": "instance1"},
		},
		&PrometheusMetric{
			metricName: "m2",
			tags:       map[string]string{"job": "job1", "instance": "instance1"},
		})

	result := metricsTypeHandler.Handle(pmb)
	assert.Equal(t, 2, len(result))
	expectedMetric1 := PrometheusMetric{
		metricName: "m1",
		metricType: textparse.MetricTypeCounter,
		tags:       map[string]string{"job": "job1", "instance": "instance1", "prom_metric_type": textparse.MetricTypeCounter},
	}
	expectedMetric2 := PrometheusMetric{
		metricName: "m2",
		metricType: textparse.MetricTypeCounter,
		tags:       map[string]string{"job": "job1", "instance": "instance1", "prom_metric_type": textparse.MetricTypeCounter},
	}
	assert.Equal(t, *result[0], expectedMetric1)
	assert.Equal(t, *result[1], expectedMetric2)
}

func TestNewMetricsTypeHandler_HandleWithReplacedJobname(t *testing.T) {
	metricsTypeHandler := NewMetricsTypeHandler()
	metricsTypeHandler.SetScrapeManager(&mockScrapeManager{})
	pmb := make(PrometheusMetricBatch, 0)
	pmb = append(pmb,
		&PrometheusMetric{
			metricName: "m1",
			tags:       map[string]string{"job": "job2_replaced", "instance": "instance2"},
		},
		&PrometheusMetric{
			metricName: "m3",
			tags:       map[string]string{"job": "job2_replaced", "instance": "instance2"},
		},
		&PrometheusMetric{
			metricName: "m2",
			tags:       map[string]string{"job": "job2_replaced", "instance": "instance2"},
		})

	result := metricsTypeHandler.Handle(pmb)
	assert.Equal(t, 2, len(result))
	expectedMetric1 := PrometheusMetric{
		metricName: "m1",
		metricType: textparse.MetricTypeGauge,
		tags:       map[string]string{"job": "job2_replaced", "instance": "instance2", "prom_metric_type": textparse.MetricTypeGauge},
	}
	expectedMetric2 := PrometheusMetric{
		metricName: "m2",
		metricType: textparse.MetricTypeGauge,
		tags:       map[string]string{"job": "job2_replaced", "instance": "instance2", "prom_metric_type": textparse.MetricTypeGauge},
	}
	assert.Equal(t, *result[0], expectedMetric1)
	assert.Equal(t, *result[1], expectedMetric2)
}

func TestNewMetricsTypeHandler_HandleWithMetricSuffix(t *testing.T) {
	metricsTypeHandler := NewMetricsTypeHandler()
	metricsTypeHandler.SetScrapeManager(&mockScrapeManager{})
	pmb := make(PrometheusMetricBatch, 0)
	pmb = append(pmb,
		&PrometheusMetric{
			metricName: "m3_sum",
			tags:       map[string]string{"job": "job1", "instance": "instance1"},
		},
		&PrometheusMetric{
			metricName: "m1_sum",
			tags:       map[string]string{"job": "job1", "instance": "instance1"},
		},
		&PrometheusMetric{
			metricName: "m2_count",
			tags:       map[string]string{"job": "job1", "instance": "instance1"},
		},
		&PrometheusMetric{
			metricName: "m4_total",
			tags:       map[string]string{"job": "job1", "instance": "instance1"},
		})

	result := metricsTypeHandler.Handle(pmb)
	assert.Equal(t, 3, len(result))
	expectedMetric1 := PrometheusMetric{
		metricName: "m1_sum",
		metricType: textparse.MetricTypeCounter,
		tags:       map[string]string{"job": "job1", "instance": "instance1", "prom_metric_type": textparse.MetricTypeCounter},
	}
	expectedMetric2 := PrometheusMetric{
		metricName: "m2_count",
		metricType: textparse.MetricTypeCounter,
		tags:       map[string]string{"job": "job1", "instance": "instance1", "prom_metric_type": textparse.MetricTypeCounter},
	}
	expectedMetric4 := PrometheusMetric{
		metricName: "m4_total",
		metricType: textparse.MetricTypeCounter,
		tags:       map[string]string{"job": "job1", "instance": "instance1", "prom_metric_type": textparse.MetricTypeCounter},
	}
	assert.Equal(t, *result[0], expectedMetric1)
	assert.Equal(t, *result[1], expectedMetric2)
	assert.Equal(t, *result[2], expectedMetric4)
}
