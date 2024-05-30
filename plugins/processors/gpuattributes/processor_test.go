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

func TestProcessMetrics(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	gp := newGpuAttributesProcessor(createDefaultConfig().(*Config), logger)
	ctx := context.Background()

	testcases := map[string]struct {
		resource string
		metrics  pmetric.Metrics
		wantCnt  int
		want     map[string]string
	}{
		"nonNode": {
			metrics: generateMetrics("prefix", map[string]string{
				"ClusterName": "cluster",
			}),
			wantCnt: 1,
			want: map[string]string{
				"ClusterName": "cluster",
			},
		},
		"nodeDropSimple": {
			metrics: generateMetrics("node", map[string]string{
				"ClusterName": "cluster",
				"Drop":        "val",
			}),
			wantCnt: 1,
			want: map[string]string{
				"ClusterName": "cluster",
			},
		},
		"nodeDropJson": {
			metrics: generateMetrics("node", map[string]string{
				"ClusterName": "cluster",
				"kubernetes":  "{\"host\":\"test\"}",
			}),
			wantCnt: 1,
			want: map[string]string{
				"ClusterName": "cluster",
				"kubernetes":  "{\"host\":\"test\"}",
			},
		},
		"nodeDropMixed": {
			metrics: generateMetrics("node", map[string]string{
				"ClusterName": "cluster",
				"Drop":        "val",
				"kubernetes":  "{\"host\":\"test\",\"b\":\"2\"}",
			}),
			wantCnt: 1,
			want: map[string]string{
				"ClusterName": "cluster",
				"kubernetes":  "{\"host\":\"test\"}",
			},
		},
		"dropPodWithoutPodName": {
			metrics: generateMetrics("pod", map[string]string{
				"ClusterName": "cluster",
				"kubernetes":  "{\"host\":\"test\",\"b\":\"2\"}",
			}),
			wantCnt: 0,
			want:    map[string]string{},
		},
		"keepPodWithoutPodName": {
			metrics: generateMetrics("pod", map[string]string{
				"ClusterName": "cluster",
				"PodName":     "pod",
				"kubernetes":  "{\"host\":\"test\",\"b\":\"2\"}",
			}),
			wantCnt: 1,
			want: map[string]string{
				"ClusterName": "cluster",
				"PodName":     "pod",
				"kubernetes":  "{\"host\":\"test\"}",
			},
		},
		"dropContainerWithoutPodName": {
			metrics: generateMetrics("container", map[string]string{
				"ClusterName": "cluster",
				"kubernetes":  "{\"host\":\"test\",\"b\":\"2\"}",
			}),
			wantCnt: 0,
			want:    map[string]string{},
		},
		"keepContainerWithoutPodName": {
			metrics: generateMetrics("container", map[string]string{
				"ClusterName": "cluster",
				"PodName":     "pod",
				"kubernetes":  "{\"host\":\"test\",\"b\":\"2\"}",
			}),
			wantCnt: 1,
			want: map[string]string{
				"ClusterName": "cluster",
				"PodName":     "pod",
				"kubernetes":  "{\"host\":\"test\"}",
			},
		},
	}

	for tname, tc := range testcases {
		fmt.Printf("running %s\n", tname)
		ms, _ := gp.processMetrics(ctx, tc.metrics)
		assert.Equal(t, tc.wantCnt, ms.MetricCount())
		if tc.wantCnt > 0 {
			attrs := ms.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes()
			assert.Equal(t, len(tc.want), attrs.Len())
			for k, v := range tc.want {
				got, ok := attrs.Get(k)
				assert.True(t, ok)
				assert.Equal(t, v, got.Str())
			}
		}
	}
}

func generateMetrics(prefix string, dimensions map[string]string) pmetric.Metrics {
	md := pmetric.NewMetrics()

	m := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName(prefix + gpuMetricIdentifier)
	gauge := m.SetEmptyGauge().DataPoints().AppendEmpty()
	gauge.SetIntValue(10)

	for k, v := range dimensions {
		gauge.Attributes().PutStr(k, v)
	}

	return md
}
