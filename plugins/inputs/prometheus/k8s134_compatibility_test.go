// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"testing"
)

func TestApplyK8s134Compatibility(t *testing.T) {
	tests := []struct {
		name     string
		input    PrometheusMetricBatch
		expected PrometheusMetricBatch
	}{
		{
			name: "apiserver_resource_objects renamed to apiserver_storage_objects with group",
			input: PrometheusMetricBatch{
				&PrometheusMetric{
					metricName: "apiserver_resource_objects",
					tags: map[string]string{
						"resource": "pods",
						"group":    "v1",
					},
				},
			},
			expected: PrometheusMetricBatch{
				&PrometheusMetric{
					metricName: "apiserver_storage_objects",
					tags: map[string]string{
						"resource": "pods.v1",
					},
				},
			},
		},
		{
			name: "apiserver_resource_objects renamed to apiserver_storage_objects without group",
			input: PrometheusMetricBatch{
				&PrometheusMetric{
					metricName: "apiserver_resource_objects",
					tags: map[string]string{
						"resource": "nodes",
					},
				},
			},
			expected: PrometheusMetricBatch{
				&PrometheusMetric{
					metricName: "apiserver_storage_objects",
					tags: map[string]string{
						"resource": "nodes",
					},
				},
			},
		},
		{
			name: "etcd_request metric gets type label",
			input: PrometheusMetricBatch{
				&PrometheusMetric{
					metricName: "etcd_request_duration_seconds",
					tags: map[string]string{
						"resource": "pods",
						"group":    "v1",
					},
				},
			},
			expected: PrometheusMetricBatch{
				&PrometheusMetric{
					metricName: "etcd_request_duration_seconds",
					tags: map[string]string{
						"resource":        "pods",
						"group":           "v1",
						"resource_prefix": "pods.v1",
						"type":            "pods.v1",
					},
				},
			},
		},
		{
			name: "apiserver_watch metric gets kind label",
			input: PrometheusMetricBatch{
				&PrometheusMetric{
					metricName: "apiserver_watch_events_total",
					tags: map[string]string{
						"resource": "pods",
						"group":    "v1",
					},
				},
			},
			expected: PrometheusMetricBatch{
				&PrometheusMetric{
					metricName: "apiserver_watch_events_total",
					tags: map[string]string{
						"resource":        "pods",
						"group":           "v1",
						"resource_prefix": "pods.v1",
						"kind":            "pods",
					},
				},
			},
		},
		{
			name: "resource without group",
			input: PrometheusMetricBatch{
				&PrometheusMetric{
					metricName: "etcd_request_total",
					tags: map[string]string{
						"resource": "nodes",
					},
				},
			},
			expected: PrometheusMetricBatch{
				&PrometheusMetric{
					metricName: "etcd_request_total",
					tags: map[string]string{
						"resource":        "nodes",
						"resource_prefix": "nodes",
						"type":            "nodes",
					},
				},
			},
		},
		{
			name: "non-control-plane metric unchanged",
			input: PrometheusMetricBatch{
				&PrometheusMetric{
					metricName: "some_other_metric",
					tags: map[string]string{
						"label": "value",
					},
				},
			},
			expected: PrometheusMetricBatch{
				&PrometheusMetric{
					metricName: "some_other_metric",
					tags: map[string]string{
						"label": "value",
					},
				},
			},
		},
		{
			name: "empty group treated as missing",
			input: PrometheusMetricBatch{
				&PrometheusMetric{
					metricName: "etcd_request_errors",
					tags: map[string]string{
						"resource": "pods",
						"group":    "",
					},
				},
			},
			expected: PrometheusMetricBatch{
				&PrometheusMetric{
					metricName: "etcd_request_errors",
					tags: map[string]string{
						"resource":        "pods",
						"group":           "",
						"resource_prefix": "pods",
						"type":            "pods",
					},
				},
			},
		},
		{
			name: "metric without resource label unchanged",
			input: PrometheusMetricBatch{
				&PrometheusMetric{
					metricName: "etcd_request_duration_seconds",
					tags: map[string]string{
						"operation": "get",
					},
				},
			},
			expected: PrometheusMetricBatch{
				&PrometheusMetric{
					metricName: "etcd_request_duration_seconds",
					tags: map[string]string{
						"operation": "get",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := applyK8s134Compatibility(tt.input)
			
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d metrics, got %d", len(tt.expected), len(result))
				return
			}

			for i, metric := range result {
				expected := tt.expected[i]
				
				if metric.metricName != expected.metricName {
					t.Errorf("Expected metric name %s, got %s", expected.metricName, metric.metricName)
				}

				for key, expectedValue := range expected.tags {
					if actualValue, exists := metric.tags[key]; !exists {
						t.Errorf("Expected tag %s to exist", key)
					} else if actualValue != expectedValue {
						t.Errorf("Expected tag %s=%s, got %s=%s", key, expectedValue, key, actualValue)
					}
				}

				// Check that no unexpected tags exist (except for the ones we know should be removed)
				for key := range metric.tags {
					if key == "group" && metric.metricName == "apiserver_storage_objects" {
						t.Errorf("Group tag should be removed for apiserver_storage_objects")
					}
				}
			}
		})
	}
}
