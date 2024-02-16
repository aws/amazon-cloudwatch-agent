// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package gpu

import (
	"context"
	"testing"

	"github.com/grafana/regexp"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

var normalizedNameRegex = regexp.MustCompile("^(container|pod|node)_gpu_[a-z_]+$")

func TestProcessMetrics(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	gp := &gpuprocessor{
		logger: logger,
		Config: createDefaultConfig().(*Config),
	}
	ctx := context.Background()
	gp.Start(ctx, nil)

	testcases := map[string]struct {
		metrics pmetric.Metrics
		want    map[string]string
	}{
		"keepExisting": {
			metrics: generateMetrics(map[string]string{
				"ClusterName":   "cluster",
				"Namespace":     "namespace",
				"Service":       "service",
				"ContainerName": "container",
				"FullPodName":   "fullpod",
				"PodName":       "pod",
				"GpuDevice":     "gpu",
			}),
			want: map[string]string{
				"ClusterName":   "cluster",
				"Namespace":     "namespace",
				"Service":       "service",
				"ContainerName": "container",
				"FullPodName":   "fullpod",
				"PodName":       "pod",
				"GpuDevice":     "gpu",
			},
		},
		"addMissing": {
			metrics: generateMetrics(map[string]string{
				"ClusterName":   "cluster",
				"Namespace":     "namespace",
				"Service":       "service",
				"ContainerName": "container",
				"FullPodName":   "fullpod",
			}),
			want: map[string]string{
				"ClusterName":   "cluster",
				"Namespace":     "namespace",
				"Service":       "service",
				"ContainerName": "container",
				"FullPodName":   "fullpod",
				"PodName":       "",
				"GpuDevice":     "",
			},
		},
		"addAll": {
			metrics: generateMetrics(map[string]string{}),
			want: map[string]string{
				"ClusterName":   "",
				"Namespace":     "",
				"Service":       "",
				"ContainerName": "",
				"FullPodName":   "",
				"PodName":       "",
				"GpuDevice":     "",
			},
		},
	}

	for _, tc := range testcases {
		ms, _ := gp.processMetrics(ctx, tc.metrics)
		attrs := ms.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes()
		assert.Equal(t, len(defaultGpuLabels), attrs.Len())
		for k, v := range tc.want {
			got, ok := attrs.Get(k)
			assert.True(t, ok)
			assert.Equal(t, v, got.Str())
		}
	}
}

func generateMetrics(dimensions map[string]string) pmetric.Metrics {
	md := pmetric.NewMetrics()

	m := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName("test" + gpuMetric)
	gauge := m.SetEmptyGauge().DataPoints().AppendEmpty()
	gauge.SetIntValue(10)

	for k, v := range dimensions {
		gauge.Attributes().PutStr(k, v)
	}

	return md
}
