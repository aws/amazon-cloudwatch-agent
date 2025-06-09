// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheusadapter

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func TestProcessMetrics(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	pap := newPrometheusAdapterProcessor(createDefaultConfig().(*Config), logger)
	require.NotNil(t, pap)

	ctx := context.Background()
	hostname, err := os.Hostname()
	require.NoError(t, err)

	m := generateGaugeMetrics(
		"test_gauge_metrics",
		map[string]string{
			"http.scheme":         "http",
			"server.port":         "8101",
			"net.host.port":       "8101",
			"url.scheme":          "http",
			"service.instance.id": "i-xxxx",
			"service.name":        "test_service",
		},
		map[string]string{
			"include":   "yes",
			"label1":    "test1",
			"my_name":   "prometheus_test_gauge",
			"prom_type": "gauge",
		},
	)

	expectedDatapointAttrs := map[string]string{
		"include":   "yes",
		"label1":    "test1",
		"my_name":   "prometheus_test_gauge",
		"prom_type": "gauge",
		"job":       "test_service",
		"host":      hostname,
		"instance":  "i-xxxx",
		"receiver":  "prometheus",
	}
	pap.processMetrics(ctx, m)

	for i := 0; i < m.ResourceMetrics().Len(); i++ {
		assert.Len(t, m.ResourceMetrics().At(0).Resource().Attributes().AsRaw(), 0, "There should be no resource attributes. resource %d has one or more attributes", i)
	}

	assert.Equal(t, 1, m.MetricCount())
	dps := m.ResourceMetrics().
		At(0).
		ScopeMetrics().
		At(0).
		Metrics().
		At(0).
		Gauge().
		DataPoints()
	assert.Equal(t, 1, dps.Len())
	for i := 0; i < dps.Len(); i++ {
		attrs := dps.At(i).Attributes()
		verifyDatapointAttributes(t, attrs, expectedDatapointAttrs)
	}
}

func TestUntypedMetrics(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	pap := newPrometheusAdapterProcessor(createDefaultConfig().(*Config), logger)
	require.NotNil(t, pap)
}

func verifyDatapointAttributes(t *testing.T, attrs pcommon.Map, expectedDatapointAttrs map[string]string) {
	// ensure all expected attributes are present
	for k, v := range expectedDatapointAttrs {
		val, ok := attrs.Get(k)
		assert.True(t, ok)
		assert.Equal(t, v, val.AsString(), "unexpected datapoint attribute {%q} value", k)
	}
	// ensure there are no other extra attributes
	for k, v := range attrs.AsRaw() {
		_, ok := expectedDatapointAttrs[k]
		assert.True(t, ok, "datapoint has attribute {%q=%q} which was not expected", k, v)
	}
}

func generateGaugeMetrics(metricName string, resourceAttrs map[string]string, datapointAttrs map[string]string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rma := rm.Resource().Attributes()
	for k, v := range resourceAttrs {
		rma.PutStr(k, v)
	}
	ms := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	ms.SetName(metricName)
	dps := ms.SetEmptyGauge().DataPoints()
	dp := dps.AppendEmpty()
	dp.SetIntValue(10)
	for k, v := range datapointAttrs {
		dp.Attributes().PutStr(k, v)
	}
	return md
}
