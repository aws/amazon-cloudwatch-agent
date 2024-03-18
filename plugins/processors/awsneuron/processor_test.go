// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsneuron

import (
	"context"
	"github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func TestAwsNeuronProcessor_ProcessMetrics(t *testing.T) {
	logger := zap.NewNop()
	config := &Config{}
	processor := newAwsNeuronProcessor(config, logger)
	processor.started = true

	md := constructSampleMetrics()

	ctx := context.Background()
	modifiedMd, err := processor.processMetrics(ctx, md)

	assert.NoError(t, err)
	assert.NotNil(t, modifiedMd)
	assert.Equal(t, 1, modifiedMd.ResourceMetrics().Len())
	assert.Equal(t, 1, modifiedMd.ResourceMetrics().At(0).ScopeMetrics().Len())
	assert.Equal(t, 8, modifiedMd.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())
}

func TestAwsNeuronProcessor_Start(t *testing.T) {
	logger := zap.NewNop()
	config := &Config{}
	processor := newAwsNeuronProcessor(config, logger)

	err := processor.Start(context.Background(), nil)

	assert.NoError(t, err)
	assert.True(t, processor.started)
}

func constructSampleMetrics() pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	metrics := rm.ScopeMetrics().AppendEmpty().Metrics()

	neuronMemoryMetric := metrics.AppendEmpty()
	neuronMemoryMetric.SetName(containerinsightscommon.NeuronCoreMemoryUtilizationConstants)
	neuronMemoryMetricDatapoints := neuronMemoryMetric.SetEmptyGauge().DataPoints()
	neuronMemoryMetricDatapoint := neuronMemoryMetricDatapoints.AppendEmpty()
	neuronMemoryMetricDatapoint.SetDoubleValue(0)

	neuronLatencyMetric := metrics.AppendEmpty()
	neuronLatencyMetric.SetName(containerinsightscommon.NeuronExecutionLatency)
	neuronLatencyMetric.SetEmptyGauge()
	neuronLatencyMetricDatapoints := neuronMemoryMetric.SetEmptyGauge().DataPoints()
	neuronLatencyMetricDatapoint := neuronLatencyMetricDatapoints.AppendEmpty()
	neuronLatencyMetricDatapoint.SetDoubleValue(0)

	nonNeuronMetric := metrics.AppendEmpty()
	nonNeuronMetric.SetName("NonNeuronMetric")
	nonNeuronMetric.SetEmptySum()
	nonNeuronMetricDatapoints := neuronMemoryMetric.SetEmptyGauge().DataPoints()
	nonNeuronMetricDatapoint := nonNeuronMetricDatapoints.AppendEmpty()
	nonNeuronMetricDatapoint.SetDoubleValue(0)

	return md
}
