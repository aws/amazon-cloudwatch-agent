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
		resource      string
		metrics       pmetric.Metrics
		wantMetricCnt int
		want          []map[string]string
	}{
		"nonNode": {
			metrics: generateMetrics("prefix", []map[string]string{
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
			metrics: generateMetrics("node", []map[string]string{
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
			metrics: generateMetrics("node", []map[string]string{
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
			metrics: generateMetrics("node", []map[string]string{
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
			metrics: generateMetrics("pod", []map[string]string{
				{
					"ClusterName": "cluster",
					"kubernetes":  "{\"host\":\"test\",\"b\":\"2\"}",
				},
			}),
			wantMetricCnt: 0,
			want:          []map[string]string{},
		},
		"keepPodWithPodName": {
			metrics: generateMetrics("pod", []map[string]string{
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
			metrics: generateMetrics("container", []map[string]string{
				{
					"ClusterName": "cluster",
					"kubernetes":  "{\"host\":\"test\",\"b\":\"2\"}",
				},
			}),
			wantMetricCnt: 0,
			want:          []map[string]string{},
		},
		"keepContainerWithPodName": {
			metrics: generateMetrics("container", []map[string]string{
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
			metrics: generateMetrics("container", []map[string]string{
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
			metrics: generateMetrics("container", []map[string]string{
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

func generateMetrics(prefix string, dimensions []map[string]string) pmetric.Metrics {
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
