// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package metricsdecorator

import (
	"context"
	"path/filepath"
	"reflect"
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
	translatorcontext.CurrentContext().SetOs(translatorconfig.OS_TYPE_WINDOWS)
	transl := NewTranslator().(*translator)
	expectedCfg := transl.factory.CreateDefaultConfig().(*transformprocessor.Config)
	c := testutil.GetConf(t, filepath.Join("testdata", "windows", "config.yaml"))
	err := c.Unmarshal(&expectedCfg)
	assert.NoError(t, err)

	conf := confmap.NewFromStringMap(testutil.GetJson(t, filepath.Join("testdata", "windows", "config.json")))
	translatedCfg, err := transl.Translate(conf)
	assert.NoError(t, err)
	actualCfg, ok := translatedCfg.(*transformprocessor.Config)
	assert.True(t, ok)

	// sort the statements for consistency
	assert.Len(t, expectedCfg.MetricStatements, 1)
	assert.Len(t, actualCfg.MetricStatements, 1)
	sort.Strings(expectedCfg.MetricStatements[0].Statements)
	sort.Strings(actualCfg.MetricStatements[0].Statements)

	assert.True(t, reflect.DeepEqual(expectedCfg, actualCfg))
}

func TestMetricDecoration(t *testing.T) {
	translatorcontext.CurrentContext().SetOs(translatorconfig.OS_TYPE_WINDOWS)
	transl := NewTranslator().(*translator)
	cfg := transl.factory.CreateDefaultConfig().(*transformprocessor.Config)
	conf := testutil.GetConf(t, filepath.Join("testdata", "windows", "config.yaml"))
	err := conf.Unmarshal(&cfg)
	assert.NoError(t, err)
	sink := new(consumertest.MetricsSink)

	expectedMetrics := pmetric.NewMetrics()
	metrics := metric.NewMetrics(expectedMetrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics())
	metrics.AddGaugeMetricDataPoint("LogicalDisk % Idle Time", "PERCENT", 0.0, 0, 0, nil)
	metrics.AddGaugeMetricDataPoint("CPU_IDLE", "PERCENT", 0.0, 0, 0, nil)
	metrics.AddGaugeMetricDataPoint("DISK_READ", "none", 0.0, 0, 0, nil)
	metrics.AddGaugeMetricDataPoint("Connections_Established", "none", 0.0, 0, 0, nil)

	ctx := context.Background()
	proc, err := transl.factory.CreateMetricsProcessor(ctx, processortest.NewNopCreateSettings(), cfg, sink)
	require.NotNil(t, proc)
	require.NoError(t, err)
	actualMetrics := pmetric.NewMetrics()
	metrics = metric.NewMetrics(actualMetrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics())
	metrics.AddGaugeMetricDataPoint("LogicalDisk % Idle Time", "PERCENT", 0.0, 0, 0, nil)
	metrics.AddGaugeMetricDataPoint("Processor % Idle Time", "PERCENT", 0.0, 0, 0, nil)
	metrics.AddGaugeMetricDataPoint("LogicalDisk % Disk Read Time", "none", 0.0, 0, 0, nil)
	metrics.AddGaugeMetricDataPoint("TCPv4 Connections Established", "none", 0.0, 0, 0, nil)
	err = proc.ConsumeMetrics(ctx, actualMetrics)
	assert.NoError(t, err)
	assert.Equal(t, expectedMetrics, actualMetrics)
}
