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
		labels  map[string]map[string]interface{}
		want    map[string]string
	}{
		"nonNode": {
			metrics: generateMetrics("prefix", map[string]string{
				"ClusterName": "cluster",
			}),
			labels: map[string]map[string]interface{}{},
			want: map[string]string{
				"ClusterName": "cluster",
			},
		},
		"nodeDropSimple": {
			metrics: generateMetrics("node", map[string]string{
				"ClusterName": "cluster",
				"Drop":        "val",
			}),
			labels: map[string]map[string]interface{}{
				"ClusterName": {},
			},
			want: map[string]string{
				"ClusterName": "cluster",
			},
		},
		"nodeDropJson": {
			metrics: generateMetrics("node", map[string]string{
				"ClusterName": "cluster",
				"kubernetes":  "{\"a\":\"1\",\"b\":\"2\"}",
			}),
			labels: map[string]map[string]interface{}{
				"ClusterName": {},
				"kubernetes":  {"a": map[string]interface{}{}},
			},
			want: map[string]string{
				"ClusterName": "cluster",
				"kubernetes":  "{\"a\":\"1\"}",
			},
		},
		"nodeDropMixed": {
			metrics: generateMetrics("node", map[string]string{
				"ClusterName": "cluster",
				"Drop":        "val",
				"kubernetes":  "{\"a\":\"1\",\"b\":\"2\"}",
			}),
			labels: map[string]map[string]interface{}{
				"ClusterName": {},
				"kubernetes":  {"a": map[string]interface{}{}},
			},
			want: map[string]string{
				"ClusterName": "cluster",
				"kubernetes":  "{\"a\":\"1\"}",
			},
		},
	}

	for _, tc := range testcases {
		nodeMetricLabels = tc.labels
		ms, _ := gp.processMetrics(ctx, tc.metrics)
		attrs := ms.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes()
		assert.Equal(t, len(tc.want), attrs.Len())
		for k, v := range tc.want {
			got, ok := attrs.Get(k)
			assert.True(t, ok)
			assert.Equal(t, v, got.Str())
		}
	}
}

func generateMetrics(prefix string, dimensions map[string]string) pmetric.Metrics {
	md := pmetric.NewMetrics()

	m := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName(prefix + gpuMetric)
	gauge := m.SetEmptyGauge().DataPoints().AppendEmpty()
	gauge.SetIntValue(10)

	for k, v := range dimensions {
		gauge.Attributes().PutStr(k, v)
	}

	return md
}
