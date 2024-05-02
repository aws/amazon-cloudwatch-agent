// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"net/url"
	"testing"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/scrape"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockMetricMetadataStore struct {
	MetricList []string
	Type       model.MetricType
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
	return make([]scrape.MetricMetadata, 0)
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
		model.JobLabel:           "job1",
		model.InstanceLabel:      "instance1",
		savedScrapeInstanceLabel: "instance1",
	})
	target1 := scrape.NewTarget(labels1, labels1, params)
	mStore1 := &mockMetricMetadataStore{
		MetricList: []string{"m1", "m2", "m4"},
		Type:       model.MetricTypeCounter,
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
		model.JobLabel:           "job2_replaced",
		model.InstanceLabel:      "instance2",
		savedScrapeInstanceLabel: "instance2",
	})
	target2 := scrape.NewTarget(labels2, labels2, params2)
	mStore2 := &mockMetricMetadataStore{
		MetricList: []string{"m1", "m2"},
		Type:       model.MetricTypeGauge,
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
	// NOTE: since https://github.com/aws/amazon-cloudwatch-agent/issues/193
	// we no longer do the look up for relabeled job in metadataServiceImpl
	assert.EqualError(t, err, "unable to find a target group with job=job2_replaced")
}

func TestMetadataServiceImpl_GetWithOriginalJobname(t *testing.T) {
	metricsTypeHandler := NewMetricsTypeHandler()
	metricsTypeHandler.SetScrapeManager(&mockScrapeManager{})
	mCache, err := metricsTypeHandler.ms.Get("job_unknown", "instance_unknown")
	assert.Equal(t, mCache, nil)
	assert.EqualError(t, err, "unable to find a target group with job=job_unknown")

	mCache, err = metricsTypeHandler.ms.Get("job1", "instance1")
	require.NoError(t, err)
	expectedMetricMetadata := scrape.MetricMetadata{
		Metric: "m1",
		Type:   model.MetricTypeCounter,
		Help:   "",
		Unit:   "",
	}
	metricMetadata, ok := mCache.Metadata("m1")
	assert.Equal(t, ok, true)
	assert.Equal(t, expectedMetricMetadata, metricMetadata)

	expectedMetricMetadata = scrape.MetricMetadata{
		Metric: "m2",
		Type:   model.MetricTypeCounter,
		Help:   "",
		Unit:   "",
	}
	metricMetadata, ok = mCache.Metadata("m2")
	assert.Equal(t, ok, true)
	assert.Equal(t, expectedMetricMetadata, metricMetadata)

	_, ok = mCache.Metadata("m3")
	assert.Equal(t, ok, false)
}

func TestMetadataServiceImpl_GetWithReplacedJobname(t *testing.T) {
	t.Skip("will always use original job name since https://github.com/aws/amazon-cloudwatch-agent/issues/193")
}

func TestNewMetricsTypeHandler_HandleWithUnknownTarget(t *testing.T) {
	metricsTypeHandler := NewMetricsTypeHandler()
	metricsTypeHandler.SetScrapeManager(&mockScrapeManager{})
	pmb := make(PrometheusMetricBatch, 0)
	pmb = append(pmb,
		&PrometheusMetric{
			metricName:              "m1",
			metricNameBeforeRelabel: "m1",
			jobBeforeRelabel:        "job_unknown",
			instanceBeforeRelabel:   "instance_unknown",
			tags:                    map[string]string{"job": "job_unknown", "instance": "instance_unknown"},
		},
		&PrometheusMetric{
			metricName:              "m2",
			metricNameBeforeRelabel: "m2",
			jobBeforeRelabel:        "job_unknown",
			instanceBeforeRelabel:   "instance_unknown",
			tags:                    map[string]string{"job": "job_unknown", "instance": "instance_unknown"},
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
			metricName:              "m3",
			metricNameBeforeRelabel: "m3",
			jobBeforeRelabel:        "job1",
			instanceBeforeRelabel:   "instance1",
			tags:                    map[string]string{"job": "job1", "instance": "instance1"},
		},
		&PrometheusMetric{
			metricName:              "m1",
			metricNameBeforeRelabel: "m1",
			jobBeforeRelabel:        "job1",
			instanceBeforeRelabel:   "instance1",
			tags:                    map[string]string{"job": "job1", "instance": "instance1"},
		},
		&PrometheusMetric{
			metricName:              "m2",
			metricNameBeforeRelabel: "m2",
			jobBeforeRelabel:        "job1",
			instanceBeforeRelabel:   "instance1",
			tags:                    map[string]string{"job": "job1", "instance": "instance1"},
		})

	result := metricsTypeHandler.Handle(pmb)
	assert.Equal(t, 2, len(result))
	expectedMetric1 := PrometheusMetric{
		metricName:              "m1",
		metricNameBeforeRelabel: "m1",
		jobBeforeRelabel:        "job1",
		instanceBeforeRelabel:   "instance1",
		metricType:              string(model.MetricTypeCounter),
		tags:                    map[string]string{"job": "job1", "instance": "instance1", "prom_metric_type": string(model.MetricTypeCounter)},
	}
	expectedMetric2 := PrometheusMetric{
		metricName:              "m2",
		metricNameBeforeRelabel: "m2",
		jobBeforeRelabel:        "job1",
		instanceBeforeRelabel:   "instance1",
		metricType:              string(model.MetricTypeCounter),
		tags:                    map[string]string{"job": "job1", "instance": "instance1", "prom_metric_type": string(model.MetricTypeCounter)},
	}
	assert.Equal(t, *result[0], expectedMetric1)
	assert.Equal(t, *result[1], expectedMetric2)
}

// For https://github.com/aws/amazon-cloudwatch-agent/issues/193
func TestNewMetricsTypeHandler_HandleWithReplacedJobname(t *testing.T) {
	metricsTypeHandler := NewMetricsTypeHandler()
	metricsTypeHandler.SetScrapeManager(&mockScrapeManager{})
	pmb := make(PrometheusMetricBatch, 0)
	pmb = append(pmb,
		&PrometheusMetric{
			metricName:              "m1",
			metricNameBeforeRelabel: "m1",
			jobBeforeRelabel:        "job2",
			instanceBeforeRelabel:   "instance2",
			tags:                    map[string]string{"job": "job2_replaced", "instance": "instance2_replaced"},
		},
		&PrometheusMetric{
			metricName:              "m3",
			metricNameBeforeRelabel: "m3",
			jobBeforeRelabel:        "job2",
			instanceBeforeRelabel:   "instance2",
			tags:                    map[string]string{"job": "job2_replaced", "instance": "instance2"},
		},
		&PrometheusMetric{
			metricName:              "m2",
			metricNameBeforeRelabel: "m2",
			jobBeforeRelabel:        "job2",
			instanceBeforeRelabel:   "instance2",
			tags:                    map[string]string{"job": "job2_replaced", "instance": "instance2"},
		})

	result := metricsTypeHandler.Handle(pmb)
	assert.Equal(t, 2, len(result))
	expectedMetric1 := PrometheusMetric{
		metricName:              "m1",
		metricNameBeforeRelabel: "m1",
		jobBeforeRelabel:        "job2",
		instanceBeforeRelabel:   "instance2",
		metricType:              string(model.MetricTypeGauge),
		tags:                    map[string]string{"job": "job2_replaced", "instance": "instance2_replaced", "prom_metric_type": string(model.MetricTypeGauge)},
	}
	expectedMetric2 := PrometheusMetric{
		metricName:              "m2",
		metricNameBeforeRelabel: "m2",
		jobBeforeRelabel:        "job2",
		instanceBeforeRelabel:   "instance2",
		metricType:              string(model.MetricTypeGauge),
		tags:                    map[string]string{"job": "job2_replaced", "instance": "instance2", "prom_metric_type": string(model.MetricTypeGauge)},
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
			metricName:              "m3_sum",
			metricNameBeforeRelabel: "m3_sum",
			jobBeforeRelabel:        "job1",
			instanceBeforeRelabel:   "instance1",
			tags:                    map[string]string{"job": "job1", "instance": "instance1"},
		},
		&PrometheusMetric{
			metricName:              "m1_sum",
			metricNameBeforeRelabel: "m1_sum",
			jobBeforeRelabel:        "job1",
			instanceBeforeRelabel:   "instance1",
			tags:                    map[string]string{"job": "job1", "instance": "instance1"},
		},
		&PrometheusMetric{
			metricName:              "m2_count",
			metricNameBeforeRelabel: "m2_count",
			jobBeforeRelabel:        "job1",
			instanceBeforeRelabel:   "instance1",
			tags:                    map[string]string{"job": "job1", "instance": "instance1"},
		},
		&PrometheusMetric{
			metricName:              "m4_total",
			metricNameBeforeRelabel: "m4_total",
			jobBeforeRelabel:        "job1",
			instanceBeforeRelabel:   "instance1",
			tags:                    map[string]string{"job": "job1", "instance": "instance1"},
		})

	result := metricsTypeHandler.Handle(pmb)
	assert.Equal(t, 3, len(result))
	expectedMetric1 := PrometheusMetric{
		metricName:              "m1_sum",
		metricNameBeforeRelabel: "m1_sum",
		jobBeforeRelabel:        "job1",
		instanceBeforeRelabel:   "instance1",
		metricType:              string(model.MetricTypeCounter),
		tags:                    map[string]string{"job": "job1", "instance": "instance1", "prom_metric_type": string(model.MetricTypeCounter)},
	}
	expectedMetric2 := PrometheusMetric{
		metricName:              "m2_count",
		metricNameBeforeRelabel: "m2_count",
		jobBeforeRelabel:        "job1",
		instanceBeforeRelabel:   "instance1",
		metricType:              string(model.MetricTypeCounter),
		tags:                    map[string]string{"job": "job1", "instance": "instance1", "prom_metric_type": string(model.MetricTypeCounter)},
	}
	expectedMetric4 := PrometheusMetric{
		metricName:              "m4_total",
		metricNameBeforeRelabel: "m4_total",
		jobBeforeRelabel:        "job1",
		instanceBeforeRelabel:   "instance1",
		metricType:              string(model.MetricTypeCounter),
		tags:                    map[string]string{"job": "job1", "instance": "instance1", "prom_metric_type": string(model.MetricTypeCounter)},
	}
	assert.Equal(t, *result[0], expectedMetric1)
	assert.Equal(t, *result[1], expectedMetric2)
	assert.Equal(t, *result[2], expectedMetric4)
}

// https://github.com/aws/amazon-cloudwatch-agent/issues/190
func TestNewMetricsTypeHandler_HandleRelabelName(t *testing.T) {
	metricsTypeHandler := NewMetricsTypeHandler()
	metricsTypeHandler.SetScrapeManager(&mockScrapeManager{})
	pmb := make(PrometheusMetricBatch, 0)
	pmb = append(pmb,
		&PrometheusMetric{
			metricName:              "m3_changed",
			metricNameBeforeRelabel: "m3",
			jobBeforeRelabel:        "job1",
			instanceBeforeRelabel:   "instance1",
			tags:                    map[string]string{"job": "job1", "instance": "instance1"},
		},
		&PrometheusMetric{
			metricName:              "m1",
			metricNameBeforeRelabel: "m1",
			jobBeforeRelabel:        "job1",
			instanceBeforeRelabel:   "instance1",
			tags:                    map[string]string{"job": "job1", "instance": "instance1"},
		},
		&PrometheusMetric{
			metricName:              "m2_changed",
			metricNameBeforeRelabel: "m2",
			jobBeforeRelabel:        "job1",
			instanceBeforeRelabel:   "instance1",
			tags:                    map[string]string{"job": "job1", "instance": "instance1"},
		})

	result := metricsTypeHandler.Handle(pmb)
	assert.Equal(t, 2, len(result))
	expectedMetric1 := PrometheusMetric{
		metricName:              "m1",
		metricNameBeforeRelabel: "m1",
		metricType:              string(model.MetricTypeCounter),
		jobBeforeRelabel:        "job1",
		instanceBeforeRelabel:   "instance1",
		// The saved label should be gone
		tags: map[string]string{"job": "job1", "instance": "instance1", "prom_metric_type": string(model.MetricTypeCounter)},
	}
	expectedMetric2 := PrometheusMetric{
		metricName:              "m2_changed",
		metricNameBeforeRelabel: "m2",
		jobBeforeRelabel:        "job1",
		instanceBeforeRelabel:   "instance1",
		metricType:              string(model.MetricTypeCounter),
		tags:                    map[string]string{"job": "job1", "instance": "instance1", "prom_metric_type": string(model.MetricTypeCounter)},
	}
	assert.Equal(t, expectedMetric1, *result[0])
	assert.Equal(t, expectedMetric2, *result[1])
}
