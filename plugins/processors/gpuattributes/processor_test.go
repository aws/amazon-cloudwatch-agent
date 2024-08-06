// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package gpuattributes

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func TestProcessMetricsForGPUMetrics(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	gp := newGpuAttributesProcessor(createDefaultConfig().(*Config), logger)
	ctx := context.Background()

	testcases := map[string]struct {
		resource      string
		metrics       pmetric.Metrics
		wantMetricCnt int
		want          []map[string]string
	}{
		"nonNode": {
			metrics: generateGPUMetrics("prefix", []map[string]string{
				{
					"ClusterName": "cluster",
				},
			}),
			wantMetricCnt: 1,
			want: []map[string]string{
				{
					"ClusterName": "cluster",
				},
			},
		},
		"nodeDropSimple": {
			metrics: generateGPUMetrics("node", []map[string]string{
				{
					"ClusterName": "cluster",
					"Drop":        "val",
				},
			}),
			wantMetricCnt: 1,
			want: []map[string]string{
				{
					"ClusterName": "cluster",
				},
			},
		},
		"nodeDropJson": {
			metrics: generateGPUMetrics("node", []map[string]string{
				{
					"ClusterName": "cluster",
					"kubernetes":  "{\"host\":\"test\"}",
				},
			}),
			wantMetricCnt: 1,
			want: []map[string]string{
				{
					"ClusterName": "cluster",
					"kubernetes":  "{\"host\":\"test\"}",
				},
			},
		},
		"nodeDropMixed": {
			metrics: generateGPUMetrics("node", []map[string]string{
				{
					"ClusterName": "cluster",
					"Drop":        "val",
					"kubernetes":  "{\"host\":\"test\",\"b\":\"2\"}",
				},
			}),
			wantMetricCnt: 1,
			want: []map[string]string{
				{
					"ClusterName": "cluster",
					"kubernetes":  "{\"host\":\"test\"}",
				},
			},
		},
		"dropPodWithoutPodName": {
			metrics: generateGPUMetrics("pod", []map[string]string{
				{
					"ClusterName": "cluster",
					"kubernetes":  "{\"host\":\"test\",\"b\":\"2\"}",
				},
			}),
			wantMetricCnt: 0,
			want:          []map[string]string{},
		},
		"keepPodWithPodName": {
			metrics: generateGPUMetrics("pod", []map[string]string{
				{
					"ClusterName": "cluster",
					"PodName":     "pod",
					"kubernetes":  "{\"host\":\"test\",\"b\":\"2\"}",
				},
			}),
			wantMetricCnt: 1,
			want: []map[string]string{
				{
					"ClusterName": "cluster",
					"PodName":     "pod",
					"kubernetes":  "{\"host\":\"test\"}",
				},
			},
		},
		"dropContainerWithoutPodName": {
			metrics: generateGPUMetrics("container", []map[string]string{
				{
					"ClusterName": "cluster",
					"kubernetes":  "{\"host\":\"test\",\"b\":\"2\"}",
				},
			}),
			wantMetricCnt: 0,
			want:          []map[string]string{},
		},
		"keepContainerWithPodName": {
			metrics: generateGPUMetrics("container", []map[string]string{
				{
					"ClusterName": "cluster",
					"PodName":     "pod",
					"kubernetes":  "{\"host\":\"test\",\"b\":\"2\"}",
				},
			}),
			wantMetricCnt: 1,
			want: []map[string]string{
				{
					"ClusterName": "cluster",
					"PodName":     "pod",
					"kubernetes":  "{\"host\":\"test\"}",
				},
			},
		},
		"dropSingleDatapointWithoutPodName": {
			metrics: generateGPUMetrics("container", []map[string]string{
				{
					"ClusterName": "cluster",
					"kubernetes":  "{\"host\":\"test\",\"b\":\"2\"}",
				},
				{
					"ClusterName": "cluster",
					"PodName":     "pod",
					"kubernetes":  "{\"host\":\"test\",\"b\":\"2\"}",
				},
			}),
			wantMetricCnt: 1,
			want: []map[string]string{
				{
					"ClusterName": "cluster",
					"PodName":     "pod",
					"kubernetes":  "{\"host\":\"test\"}",
				},
			},
		},
		"keepAllDatapoints": {
			metrics: generateGPUMetrics("container", []map[string]string{
				{
					"ClusterName": "cluster",
					"PodName":     "pod1",
					"kubernetes":  "{\"host\":\"test\",\"b\":\"2\"}",
				},
				{
					"ClusterName": "cluster",
					"PodName":     "pod2",
					"kubernetes":  "{\"host\":\"test\",\"b\":\"2\"}",
				},
			}),
			wantMetricCnt: 1,
			want: []map[string]string{
				{
					"ClusterName": "cluster",
					"PodName":     "pod1",
					"kubernetes":  "{\"host\":\"test\"}",
				},
				{
					"ClusterName": "cluster",
					"PodName":     "pod2",
					"kubernetes":  "{\"host\":\"test\"}",
				},
			},
		},
	}

	for tname, tc := range testcases {
		fmt.Printf("running %s\n", tname)
		ms, _ := gp.processMetrics(ctx, tc.metrics)
		assert.Equal(t, tc.wantMetricCnt, ms.MetricCount())
		if tc.wantMetricCnt > 0 {
			dps := ms.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints()
			assert.Equal(t, len(tc.want), dps.Len())
			for i, dim := range tc.want {
				attrs := dps.At(i).Attributes()
				assert.Equal(t, len(dim), attrs.Len())
				for k, v := range dim {
					got, ok := attrs.Get(k)
					assert.True(t, ok)
					assert.Equal(t, v, got.Str())
				}
			}
		}
	}
}

func TestProcessMetricsForNeuronMetrics(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	gp := newGpuAttributesProcessor(createDefaultConfig().(*Config), logger)
	ctx := context.Background()

	testcases := map[string]struct {
		resource      string
		metrics       pmetric.Metrics
		wantMetricCnt int
		want          []map[string]string
	}{
		"neuronMetricsProcessedWithNoPodCorrelation": {
			metrics: generateNeuronMetrics("neuron_execution_latency", []map[string]string{
				{
					"ClusterName": "cluster",
					"Drop":        "val",
					"percentile":  "p50",
					"kubernetes":  "{\"host\":\"test\",\"drop\":\"2\",\"labels\":\"label\"}",
				},
			}),
			wantMetricCnt: 2,
			want: []map[string]string{
				{
					"ClusterName": "cluster",
					"Drop":        "val",
					"percentile":  "p50",
					"kubernetes":  "{\"host\":\"test\",\"drop\":\"2\",\"labels\":\"label\"}",
				},
				{
					"ClusterName": "cluster",
					"Type":        "NodeAWSNeuron",
					"kubernetes":  "{\"host\":\"test\",\"labels\":\"label\"}",
				},
			},
		},
		"neuronMetricsProcessedWithPodCorrelation": {
			metrics: generateNeuronMetrics("neuroncore_memory_usage_constants", []map[string]string{
				{
					"ClusterName":   "cluster",
					"Drop":          "val",
					"runtime_tag":   "10",
					"NeuronCore":    "0",
					"NeuronDevice":  "0",
					"PodName":       "testPod",
					"ContainerName": "testContainer",
					"kubernetes":    "{\"host\":\"test\",\"drop\":\"2\",\"labels\":\"label\"}",
				},
			}),
			wantMetricCnt: 7,
			want: []map[string]string{
				{
					"ClusterName":   "cluster",
					"Drop":          "val",
					"runtime_tag":   "10",
					"NeuronCore":    "core0",
					"NeuronDevice":  "device0",
					"PodName":       "testPod",
					"ContainerName": "testContainer",
					"kubernetes":    "{\"host\":\"test\",\"drop\":\"2\",\"labels\":\"label\"}",
				},
				{
					"ClusterName":   "cluster",
					"runtime_tag":   "10",
					"NeuronCore":    "core0",
					"NeuronDevice":  "device0",
					"Type":          "ContainerAWSNeuronCore",
					"PodName":       "testPod",
					"ContainerName": "testContainer",
					"kubernetes":    "{\"host\":\"test\",\"labels\":\"label\"}",
				},
				{
					"ClusterName":  "cluster",
					"runtime_tag":  "10",
					"NeuronCore":   "core0",
					"NeuronDevice": "device0",
					"Type":         "PodAWSNeuronCore",
					"PodName":      "testPod",
					"kubernetes":   "{\"host\":\"test\",\"labels\":\"label\"}",
				},
				{
					"ClusterName":  "cluster",
					"runtime_tag":  "10",
					"NeuronCore":   "core0",
					"NeuronDevice": "device0",
					"Type":         "NodeAWSNeuronCore",
					"kubernetes":   "{\"host\":\"test\",\"labels\":\"label\"}",
				},
				{
					"ClusterName":   "cluster",
					"runtime_tag":   "10",
					"NeuronCore":    "core0",
					"NeuronDevice":  "device0",
					"Type":          "ContainerAWSNeuronCore",
					"PodName":       "testPod",
					"ContainerName": "testContainer",
					"kubernetes":    "{\"host\":\"test\",\"labels\":\"label\"}",
				},
				{
					"ClusterName":  "cluster",
					"runtime_tag":  "10",
					"NeuronCore":   "core0",
					"NeuronDevice": "device0",
					"Type":         "PodAWSNeuronCore",
					"PodName":      "testPod",
					"kubernetes":   "{\"host\":\"test\",\"labels\":\"label\"}",
				},
				{
					"ClusterName":  "cluster",
					"runtime_tag":  "10",
					"NeuronCore":   "core0",
					"NeuronDevice": "device0",
					"Type":         "NodeAWSNeuronCore",
					"kubernetes":   "{\"host\":\"test\",\"labels\":\"label\"}",
				},
			},
		},
		"neuronMemoryMetricsAggregated": {
			metrics: generateNeuronMetrics("neuroncore_memory_usage_constants", []map[string]string{
				{
					"ClusterName":  "cluster",
					"Drop":         "val",
					"runtime_tag":  "10",
					"NeuronCore":   "0",
					"NeuronDevice": "0",
					"kubernetes":   "{\"host\":\"test\",\"drop\":\"2\",\"labels\":\"label\"}",
				},
			}),
			wantMetricCnt: 3,
			want: []map[string]string{
				{
					"ClusterName":  "cluster",
					"Drop":         "val",
					"runtime_tag":  "10",
					"NeuronCore":   "core0",
					"NeuronDevice": "device0",
					"kubernetes":   "{\"host\":\"test\",\"drop\":\"2\",\"labels\":\"label\"}",
				},
				{
					"ClusterName":  "cluster",
					"runtime_tag":  "10",
					"NeuronCore":   "core0",
					"NeuronDevice": "device0",
					"Type":         "NodeAWSNeuronCore",
					"kubernetes":   "{\"host\":\"test\",\"labels\":\"label\"}",
				},
				{
					"ClusterName":  "cluster",
					"runtime_tag":  "10",
					"NeuronCore":   "core0",
					"NeuronDevice": "device0",
					"Type":         "NodeAWSNeuronCore",
					"kubernetes":   "{\"host\":\"test\",\"labels\":\"label\"}",
				},
			},
		},
		"neuronDeviceHardwareMetrics_labelsAreDropped": {
			metrics: generateNeuronMetrics("neurondevice_hw_ecc_events", []map[string]string{
				{
					"ClusterName":   "cluster",
					"Drop":          "val",
					"runtime_tag":   "10",
					"NeuronCore":    "0",
					"NeuronDevice":  "0",
					"event_type":    "mem_ecc_corrected",
					"kubernetes":    "{\"host\":\"test\",\"drop\":\"2\",\"labels\":\"label\"}",
					"PodName":       "testPod",
					"ContainerName": "testContainer",
				},
			}),
			wantMetricCnt: 7,
			want: []map[string]string{
				{
					"ClusterName":   "cluster",
					"Drop":          "val",
					"runtime_tag":   "10",
					"NeuronCore":    "core0",
					"NeuronDevice":  "device0",
					"event_type":    "mem_ecc_corrected",
					"kubernetes":    "{\"host\":\"test\",\"drop\":\"2\",\"labels\":\"label\"}",
					"PodName":       "testPod",
					"ContainerName": "testContainer",
				},
				{
					"ClusterName":   "cluster",
					"runtime_tag":   "10",
					"NeuronCore":    "core0",
					"NeuronDevice":  "device0",
					"Type":          "ContainerAWSNeuronDevice",
					"kubernetes":    "{\"host\":\"test\"}",
					"PodName":       "testPod",
					"ContainerName": "testContainer",
				},
				{
					"ClusterName":  "cluster",
					"runtime_tag":  "10",
					"NeuronCore":   "core0",
					"NeuronDevice": "device0",
					"Type":         "PodAWSNeuronDevice",
					"kubernetes":   "{\"host\":\"test\"}",
					"PodName":      "testPod",
				},
				{
					"ClusterName":  "cluster",
					"runtime_tag":  "10",
					"NeuronCore":   "core0",
					"NeuronDevice": "device0",
					"Type":         "NodeAWSNeuronDevice",
					"kubernetes":   "{\"host\":\"test\"}",
				},
				{
					"ClusterName":   "cluster",
					"runtime_tag":   "10",
					"NeuronCore":    "core0",
					"NeuronDevice":  "device0",
					"Type":          "ContainerAWSNeuronDevice",
					"kubernetes":    "{\"host\":\"test\"}",
					"PodName":       "testPod",
					"ContainerName": "testContainer",
				},
				{
					"ClusterName":  "cluster",
					"runtime_tag":  "10",
					"NeuronCore":   "core0",
					"NeuronDevice": "device0",
					"Type":         "PodAWSNeuronDevice",
					"kubernetes":   "{\"host\":\"test\"}",
					"PodName":      "testPod",
				},
				{
					"ClusterName":  "cluster",
					"runtime_tag":  "10",
					"NeuronCore":   "core0",
					"NeuronDevice": "device0",
					"Type":         "NodeAWSNeuronDevice",
					"kubernetes":   "{\"host\":\"test\"}",
				},
			},
		},
	}

	for tname, tc := range testcases {
		fmt.Printf("running %s\n", tname)
		ms, _ := gp.processMetrics(ctx, tc.metrics)
		assert.Equal(t, tc.wantMetricCnt, ms.MetricCount())
		if tc.wantMetricCnt > 0 {
			resourceMetricsAttributes := ms.ResourceMetrics().At(0).Resource().Attributes()
			assert.Equal(t, 0, resourceMetricsAttributes.Len())
			for i, dim := range tc.want {
				dpAttr := ms.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(i).Sum().DataPoints().At(0).Attributes()
				assert.Equal(t, len(dim), dpAttr.Len())
				for k, v := range dim {
					got, ok := dpAttr.Get(k)
					assert.True(t, ok)
					assert.Equal(t, v, got.Str())
				}
			}
		}
	}
}

func generateGPUMetrics(prefix string, dimensions []map[string]string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	ms := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	ms.SetName(prefix + gpuMetricIdentifier)
	dps := ms.SetEmptyGauge().DataPoints()
	for _, dim := range dimensions {
		dp := dps.AppendEmpty()
		dp.SetIntValue(10)
		for k, v := range dim {
			dp.Attributes().PutStr(k, v)
		}
	}
	return md
}

func generateNeuronMetrics(prefix string, dimensions []map[string]string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	ms := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	md.ResourceMetrics().At(0).Resource().Attributes().PutStr("service.name", "containerInsightsNeuronMonitorScraper")
	ms.SetName(prefix)
	dps := ms.SetEmptyGauge().DataPoints()
	for _, dim := range dimensions {
		dp := dps.AppendEmpty()
		dp.SetIntValue(10)
		for k, v := range dim {
			dp.Attributes().PutStr(k, v)
		}
	}
	return md
}
