// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
)

var memoryMetricsNames = map[string]struct{}{
	containerinsightscommon.NeuronCoreMemoryUtilizationConstants:        {},
	containerinsightscommon.NeuronCoreMemoryUtilizationModelCode:        {},
	containerinsightscommon.NeuronCoreMemoryUtilizationSharedScratchpad: {},
	containerinsightscommon.NeuronCoreMemoryUtilizationRuntimeMemory:    {},
	containerinsightscommon.NeuronCoreMemoryUtilizationTensors:          {},
}

type NeuronCoreInfo struct {
	neuronCoreIndex   string
	neuronDeviceIndex string
	runtimeTag        string
}

type AwsNeuronMemoryMetricsAggregator struct {
	memoryMetricValuesAggregator     map[NeuronCoreInfo]float64
	aggregatedMemoryMetricAttributes pcommon.Map
	metricTimestamp                  pcommon.Timestamp
	MemoryMetricsFound               bool
}

func NewMemoryMemoryAggregator() *AwsNeuronMemoryMetricsAggregator {
	return &AwsNeuronMemoryMetricsAggregator{memoryMetricValuesAggregator: map[NeuronCoreInfo]float64{}, MemoryMetricsFound: false}
}

func (d *AwsNeuronMemoryMetricsAggregator) AggregateMemoryMetric(originalMetric pmetric.Metric) {
	if _, exists := memoryMetricsNames[originalMetric.Name()]; !exists {
		return
	}

	datapoints := originalMetric.Gauge().DataPoints()

	if datapoints.Len() <= 0 {
		return
	}

	d.MemoryMetricsFound = true
	d.aggregatedMemoryMetricAttributes = datapoints.At(0).Attributes()
	d.metricTimestamp = datapoints.At(0).Timestamp()

	for i := 0; i < datapoints.Len(); i++ {
		datapoint := datapoints.At(i)

		neuronCoreIndexValue, neuronCoreIndexValueExists := datapoint.Attributes().Get(NeuronCoreAttributeKey)
		neuronDeviceIndexValue, neuronDeviceIndexValueExists := datapoint.Attributes().Get(NeuronDeviceAttributeKey)
		runtimeTagValue, runtimeTagExists := datapoint.Attributes().Get(RuntimeTag)

		if neuronCoreIndexValueExists && neuronDeviceIndexValueExists && runtimeTagExists {
			neuronCoreInfo := NeuronCoreInfo{neuronCoreIndex: neuronCoreIndexValue.AsString(), neuronDeviceIndex: neuronDeviceIndexValue.AsString(), runtimeTag: runtimeTagValue.AsString()}
			d.memoryMetricValuesAggregator[neuronCoreInfo] += datapoint.DoubleValue()
		}
	}

}

func (d *AwsNeuronMemoryMetricsAggregator) FlushAggregatedMemoryMetric() pmetric.Metric {
	aggregatedMemoryMetric := pmetric.NewMetric()
	aggregatedMemoryMetric.SetName(containerinsightscommon.NeuronCoreMemoryUtilizationTotal)
	datapoints := aggregatedMemoryMetric.SetEmptySum().DataPoints()

	for neuronCoreInfo, totalMemoryUsed := range d.memoryMetricValuesAggregator {
		datapoint := datapoints.AppendEmpty()
		datapoint.SetDoubleValue(totalMemoryUsed)
		d.aggregatedMemoryMetricAttributes.CopyTo(datapoint.Attributes())

		datapoint.Attributes().PutStr(NeuronCoreAttributeKey, neuronCoreInfo.neuronCoreIndex)
		datapoint.Attributes().PutStr(NeuronDeviceAttributeKey, neuronCoreInfo.neuronDeviceIndex)
		datapoint.Attributes().PutStr(RuntimeTag, neuronCoreInfo.runtimeTag)
		datapoint.SetTimestamp(d.metricTimestamp)
	}

	// Reset the aggregator
	d.resetMemoryMetricAggregator()
	return aggregatedMemoryMetric
}

func (d *AwsNeuronMemoryMetricsAggregator) resetMemoryMetricAggregator() {
	d.memoryMetricValuesAggregator = map[NeuronCoreInfo]float64{}
	d.MemoryMetricsFound = false
}
