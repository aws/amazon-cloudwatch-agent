// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package kueueattributes

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func TestProcessMetricsForKueueMetrics(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	kp := newKueueAttributesProcessor(createDefaultConfig().(*Config), logger)
	ctx := context.Background()

	testcases := map[string]struct {
		resource      string
		metrics       pmetric.Metrics
		wantMetriccnt int
		want          []map[string]string
	}{
		"nonKueue": {
			metrics: generateKueueMetrics("someOthermetric", []map[string]string{
				{
					"ClusterName": "cluster",
				},
			}),
			wantMetriccnt: 1,
			want: []map[string]string{
				{
					"ClusterName": "cluster",
				},
			},
		},
		"KeepAll": {
			metrics: generateKueueMetrics("kueue_pending_workloads", []map[string]string{
				{
					"ClusterName":  "cluster",
					"ClusterQueue": "production",
					"Status":       "active",
				},
				{
					"ClusterName":  "cluster",
					"ClusterQueue": "development",
					"Status":       "inadmissible",
					"NodeName":     "kubernetes-kueue",
				},
			}),
			wantMetriccnt: 1,
			want: []map[string]string{
				{
					"ClusterName":  "cluster",
					"ClusterQueue": "production",
					"Status":       "active",
				},
				{
					"ClusterName":  "cluster",
					"ClusterQueue": "development",
					"Status":       "inadmissible",
					"NodeName":     "kubernetes-kueue",
				},
			},
		},
		"dropLabel": {
			metrics: generateKueueMetrics("kueue_pending_workloads", []map[string]string{
				{
					"ClusterName":  "cluster",
					"ClusterQueue": "production",
					"Status":       "active",
					"Pod":          "somepod",
				},
			}),
			wantMetriccnt: 1,
			want: []map[string]string{
				{
					"ClusterName":  "cluster",
					"ClusterQueue": "production",
					"Status":       "active",
				},
			},
		},
	}

	for tname, tc := range testcases {
		fmt.Printf("running %s\n", tname)
		ms, _ := kp.processMetrics(ctx, tc.metrics)
		assert.Equal(t, tc.wantMetriccnt, ms.MetricCount())
		if tc.wantMetriccnt > 0 {
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

func generateKueueMetrics(metricName string, dimensions []map[string]string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	ms := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	ms.SetName(metricName)
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
