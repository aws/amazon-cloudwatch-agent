// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build !windows
// +build !windows

package metricsdecorator

import (
	"context"
	"path/filepath"
	"sort"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/aws/amazon-cloudwatch-agent/internal/metric"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/testutil"
	translatorconfig "github.com/aws/amazon-cloudwatch-agent/translator/config"
	translatorcontext "github.com/aws/amazon-cloudwatch-agent/translator/context"
)

func TestTranslate(t *testing.T) {
	translatorcontext.CurrentContext().SetOs(translatorconfig.OS_TYPE_LINUX)
	transl := NewTranslator().(*translator)
	expectedCfg := transl.factory.CreateDefaultConfig().(*transformprocessor.Config)
	c := testutil.GetConf(t, filepath.Join("testdata", "unix", "config.yaml"))
	require.NoError(t, c.Unmarshal(&expectedCfg))

	conf := confmap.NewFromStringMap(testutil.GetJson(t, filepath.Join("testdata", "unix", "config.json")))
	translatedCfg, err := transl.Translate(conf)
	assert.NoError(t, err)
	actualCfg, ok := translatedCfg.(*transformprocessor.Config)
	assert.True(t, ok)

	// sort the statements for consistency
	assert.Len(t, expectedCfg.MetricStatements, 1)
	assert.Len(t, actualCfg.MetricStatements, 1)
	sort.Strings(expectedCfg.MetricStatements[0].Statements)
	sort.Strings(actualCfg.MetricStatements[0].Statements)
}

// TestMetricDecoration - This test is used to verify that metrics are receiving decorations correctly.
// This is done by using a test TransformProcessor yaml configuration, starting the processor
// and having it consume test metrics.
func TestMetricDecoration(t *testing.T) {
	translatorcontext.CurrentContext().SetOs(translatorconfig.OS_TYPE_LINUX)
	transl := NewTranslator().(*translator)
	cfg := transl.factory.CreateDefaultConfig().(*transformprocessor.Config)
	conf := testutil.GetConf(t, filepath.Join("testdata", "unix", "config.yaml"))
	require.NoError(t, conf.Unmarshal(&cfg))
	sink := new(consumertest.MetricsSink)

	expectedMetrics := pmetric.NewMetrics()
	metrics := metric.NewMetrics(expectedMetrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics())
	metrics.AddGaugeMetricDataPoint("CPU_USAGE_IDLE", "unit", 0.0, 0, 0, nil)
	metrics.AddGaugeMetricDataPoint("cpu_time_active_renamed", "none", 0.0, 0, 0, nil)
	metrics.AddGaugeMetricDataPoint("other_metric", "none", 0.0, 0, 0, nil)

	ctx := context.Background()
	proc, err := transl.factory.CreateMetricsProcessor(ctx, processortest.NewNopCreateSettings(), cfg, sink)
	require.NotNil(t, proc)
	require.NoError(t, err)
	actualMetrics := pmetric.NewMetrics()
	metrics = metric.NewMetrics(actualMetrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics())
	metrics.AddGaugeMetricDataPoint("cpu_usage_idle", "none", 0.0, 0, 0, nil)
	metrics.AddGaugeMetricDataPoint("cpu_time_active", "none", 0.0, 0, 0, nil)
	metrics.AddGaugeMetricDataPoint("other_metric", "none", 0.0, 0, 0, nil)
	assert.NoError(t, proc.ConsumeMetrics(ctx, actualMetrics))

	assert.Equal(t, expectedMetrics, actualMetrics)
}
