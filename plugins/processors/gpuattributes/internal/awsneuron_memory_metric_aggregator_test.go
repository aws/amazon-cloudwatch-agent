// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
)

const (
	dummy = "dummy"
)

var (
	memoryUsageMetricValuesMap = map[string]float64{
		"0": 20,
		"2": 40,
	}
)

func TestMemoryMetricAggregator_AggregateMemoryMetric(t *testing.T) {
	aggregator := NewMemoryMemoryAggregator()

	// Create a sample original metric with gauge data points
	tensorsMemoryUsage := createSampleMetric(containerinsightscommon.NeuronCoreMemoryUtilizationTensors)
	nonNeuronMetric := createSampleMetric(dummy)

	// Call the method being tested
	aggregator.AggregateMemoryMetric(tensorsMemoryUsage)
	aggregator.AggregateMemoryMetric(nonNeuronMetric)

	// Assert that memory metrics were found
	assert.True(t, aggregator.MemoryMetricsFound)
}

func TestMemoryMetricAggregator_NonNeuronMetric(t *testing.T) {
	aggregator := NewMemoryMemoryAggregator()

	// Create a sample original metric with gauge data points
	nonNeuronMetric := createSampleMetric("dummy")

	// Call the method being tested
	aggregator.AggregateMemoryMetric(nonNeuronMetric)

	// Assert that memory metrics were found
	assert.False(t, aggregator.MemoryMetricsFound)
}

func TestMemoryMetricAggregator_FlushAggregatedMemoryMetric(t *testing.T) {
	aggregator := NewMemoryMemoryAggregator()
	aggregator.aggregatedMemoryMetricAttributes = pcommon.NewMap()
	aggregator.aggregatedMemoryMetricAttributes.FromRaw(map[string]any{
		NeuronCoreAttributeKey:   "9",
		NeuronDeviceAttributeKey: "9",
		dummy:                    dummy,
	})

	aggregator.metricTimestamp = staticTimestamp

	// Add some data to the aggregator
	// Create a sample original metric with gauge data points
	tensorsMemoryUsage := createSampleMetric(containerinsightscommon.NeuronCoreMemoryUtilizationTensors)
	constantsMemoryUsage := createSampleMetric(containerinsightscommon.NeuronCoreMemoryUtilizationConstants)
	nonNeuronMetric := createSampleMetric(dummy)

	// Call the method being tested
	aggregator.AggregateMemoryMetric(tensorsMemoryUsage)
	aggregator.AggregateMemoryMetric(constantsMemoryUsage)
	aggregator.AggregateMemoryMetric(nonNeuronMetric)

	// Call the method being tested
	aggregatedMetric := aggregator.FlushAggregatedMemoryMetric()
	aggregatedMetricDatapoints := aggregatedMetric.Sum().DataPoints()
	// Assert the result
	assert.NotNil(t, aggregatedMetric)
	assert.Equal(t, containerinsightscommon.NeuronCoreMemoryUtilizationTotal, aggregatedMetric.Name())
	assert.Equal(t, 2, aggregatedMetricDatapoints.Len())

	for i := 0; i < aggregatedMetricDatapoints.Len(); i++ {
		datapoint := aggregatedMetricDatapoints.At(i)
		assert.Equal(t, staticTimestamp.String(), datapoint.Timestamp().String())
		assert.Equal(t, 4, datapoint.Attributes().Len())

		actualNeuronCoreIndex, _ := datapoint.Attributes().Get(NeuronCoreAttributeKey)
		actualNeuronDeviceIndex, _ := datapoint.Attributes().Get(NeuronDeviceAttributeKey)
		actualRuntimeTag, _ := datapoint.Attributes().Get(RuntimeTag)

		assert.Equal(t, memoryUsageMetricValuesMap[actualNeuronCoreIndex.AsString()], datapoint.DoubleValue())
		assert.Equal(t, "1", actualRuntimeTag.AsString())
		assert.NotEqual(t, "9", actualNeuronCoreIndex.AsString())
		assert.NotEqual(t, "9", actualNeuronDeviceIndex.AsString())
	}
}

func createSampleMetric(metricName string) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(metricName)

	// Add gauge data points
	dataPoints := metric.SetEmptyGauge().DataPoints()
	dataPoint1 := dataPoints.AppendEmpty()
	dataPoint1.SetDoubleValue(10.0)
	dataPoint1.SetTimestamp(staticTimestamp)
	dataPoint1.Attributes().FromRaw(map[string]any{
		NeuronCoreAttributeKey:   "0",
		NeuronDeviceAttributeKey: "0",
		dummy:                    dummy,
		RuntimeTag:               "1",
	})

	dataPoint2 := dataPoints.AppendEmpty()
	dataPoint2.SetDoubleValue(20.0)
	dataPoint1.SetTimestamp(staticTimestamp)
	dataPoint2.Attributes().FromRaw(map[string]any{
		NeuronCoreAttributeKey:   "2",
		NeuronDeviceAttributeKey: "1",
		dummy:                    dummy,
		RuntimeTag:               "1",
	})

	return metric
}
