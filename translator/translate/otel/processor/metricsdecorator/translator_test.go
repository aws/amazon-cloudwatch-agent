// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metricsdecorator

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/internal/metric"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/internal/util/testutil"
)

func TestTranslate(t *testing.T) {
	transl := NewTranslator().(*translator)
	expectedCfg := transl.factory.CreateDefaultConfig().(*transformprocessor.Config)
	c := testutil.GetConf(t, filepath.Join("testdata", "config.yaml"))
	c.Unmarshal(&expectedCfg)

	conf := confmap.NewFromStringMap(testutil.GetJson(t, filepath.Join("testdata", "config.json")))
	actualCfg, err := transl.Translate(conf)
	assert.NoError(t, err)

	assert.True(t, reflect.DeepEqual(expectedCfg, actualCfg))
}

// TestMetricDecoration - This test is used to verify that metrics are receiving decorations correctly.
// This is done by using a test TransformProcessor yaml configuration, starting the processor
// and having it consume test metrics.
func TestMetricDecoration(t *testing.T) {
	transl := NewTranslator().(*translator)
	cfg := transl.factory.CreateDefaultConfig().(*transformprocessor.Config)
	conf := testutil.GetConf(t, filepath.Join("testdata", "config.yaml"))
	conf.Unmarshal(&cfg)
	sink := new(consumertest.MetricsSink)

	expectedMetrics := pmetric.NewMetrics()
	metrics := metric.NewMetrics(expectedMetrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics())
	metrics.AddGaugeMetricDataPoint("CPU_USAGE_IDLE", "unit", 0.0, 0, 0, nil)

	ctx := context.Background()
	proc, err := transl.factory.CreateMetricsProcessor(ctx, processortest.NewNopCreateSettings(), cfg, sink)
	require.NotNil(t, proc)
	require.NoError(t, err)
	actualMetrics := pmetric.NewMetrics()
	metrics = metric.NewMetrics(actualMetrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics())
	metrics.AddGaugeMetricDataPoint("cpu_usage_idle", "none", 0.0, 0, 0, nil)
	proc.ConsumeMetrics(ctx, actualMetrics)

	assert.Equal(t, expectedMetrics, actualMetrics)

}
