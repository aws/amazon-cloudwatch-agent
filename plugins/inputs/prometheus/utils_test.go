// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetMetricKeyForMerging(t *testing.T) {
	pm := &PrometheusMetric{
		tags:        map[string]string{"tagA": "tagA_v", "tagB": "tagB_v"},
		metricName:  "metric_name",
		metricValue: 0.1,
		metricType:  "counter",
		timeInMS:    100,
	}
	r := getMetricKeyForMerging(pm)
	assert.Equal(t, "tagA=tagA_v,tagB=tagB_v,", r)
}

func Test_getUniqMetricKey(t *testing.T) {
	pm := &PrometheusMetric{
		tags:        map[string]string{"tagA": "tagA_v", "tagB": "tagB_v"},
		metricName:  "metric_name",
		metricValue: 0.1,
		metricType:  "counter",
		timeInMS:    100,
	}
	r := getUniqMetricKey(pm)
	assert.Equal(t, "tagA=tagA_v,tagB=tagB_v,metricName=metric_name,", r)
}

func Test_mergeMetrics_merged(t *testing.T) {
	// merge based on tag lists
	type PrometheusMetricBatch []*PrometheusMetric
	pmb := []*PrometheusMetric{
		&PrometheusMetric{
			tags:        map[string]string{"tagA": "tagA_v", "tagB": "tagB_v"},
			metricName:  "metric_a",
			metricValue: 0.1,
			metricType:  "counter",
			timeInMS:    100,
		},
		&PrometheusMetric{
			tags:        map[string]string{"tagA": "tagA_v", "tagB": "tagB_v"},
			metricName:  "metric_b",
			metricValue: 0.2,
			metricType:  "counter",
			timeInMS:    100,
		},
	}
	mm := mergeMetrics(pmb)
	expected := []*metricMaterial{&metricMaterial{
		timeInMS: int64(100),
		tags:     map[string]string{"tagA": "tagA_v", "tagB": "tagB_v"},
		fields:   map[string]interface{}{"metric_a": 0.1, "metric_b": 0.2},
	},
	}
	assert.True(t, reflect.DeepEqual(expected, mm))
}

func Test_mergeMetrics_not_merged(t *testing.T) {
	// merge based on tag lists
	type PrometheusMetricBatch []*PrometheusMetric
	pmb := []*PrometheusMetric{
		&PrometheusMetric{
			tags:        map[string]string{"tagA": "tagA_v", "tagB": "tagB_v"},
			metricName:  "metric_b",
			metricValue: 0.2,
			metricType:  "counter",
			timeInMS:    100,
		},
		&PrometheusMetric{
			tags:        map[string]string{"tagA": "tagA_v", "tagC": "tagC_v"},
			metricName:  "metric_c",
			metricValue: 0.3,
			metricType:  "counter",
			timeInMS:    100,
		},
	}
	assert.Equal(t, 2, len(mergeMetrics(pmb)))
}

func TestIsInternalMetrics(t *testing.T) {
	testCases := map[string]struct {
		metricName             string
		expectedInternalMetric bool
	}{
		"ValidInternalMetric": {
			metricName:             "scrape_internal",
			expectedInternalMetric: true,
		},
		"InvalidInternalMetric": {
			metricName:             "internal",
			expectedInternalMetric: false,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(_ *testing.T) {
			isInternalMetric := isInternalMetric(testCase.metricName)
			assert.Equal(t, testCase.expectedInternalMetric, isInternalMetric)
		})
	}

}
