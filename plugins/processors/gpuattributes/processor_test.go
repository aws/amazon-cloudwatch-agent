// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package gpuattributes

import (
	"context"
	"fmt"
	"testing"

	"github.com/grafana/regexp"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

var normalizedNameRegex = regexp.MustCompile("^(container|pod|node)_gpu_[a-z_]+$")

func TestProcessMetrics(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	gp := newGpuAttributesProcessor(createDefaultConfig().(*Config), logger)
	ctx := context.Background()

	testcases := map[string]struct {
		resource string
		metrics  pmetric.Metrics
		want     map[string]string
	}{
		"nonNode": {
			metrics: generateMetrics("prefix", map[string]string{
				"ClusterName": "cluster",
			}),
			want: map[string]string{
				"ClusterName": "cluster",
			},
		},
		"nodeDropSimple": {
			metrics: generateMetrics("node", map[string]string{
				"ClusterName": "cluster",
				"Drop":        "val",
			}),
			want: map[string]string{
				"ClusterName": "cluster",
			},
		},
		"nodeDropJson": {
			metrics: generateMetrics("node", map[string]string{
				"ClusterName": "cluster",
				"kubernetes":  "{\"host\":\"test\"}",
			}),
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
			want: map[string]string{
				"ClusterName": "cluster",
				"kubernetes":  "{\"host\":\"test\"}",
			},
		},
	}

	for tname, tc := range testcases {
		fmt.Printf("running %s\n", tname)
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
	m.SetName(prefix + gpuMetricIdentifier)
	gauge := m.SetEmptyGauge().DataPoints().AppendEmpty()
	gauge.SetIntValue(10)

	for k, v := range dimensions {
		gauge.Attributes().PutStr(k, v)
	}

	return md
}
