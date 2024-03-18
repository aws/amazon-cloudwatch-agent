// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
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
}

type MemoryMetricAggregator struct {
	memoryMetricValuesAggregator     map[NeuronCoreInfo]float64
	aggregatedMemoryMetricAttributes pcommon.Map
	metricTimestamp                  pcommon.Timestamp
	MemoryMetricsFound               bool
}

func NewMemoryMemoryAggregator() *MemoryMetricAggregator {
	return &MemoryMetricAggregator{memoryMetricValuesAggregator: map[NeuronCoreInfo]float64{}, MemoryMetricsFound: false}
}

func (d *MemoryMetricAggregator) AggregateMemoryMetric(originalMetric pmetric.Metric) {
	if _, exists := memoryMetricsNames[originalMetric.Name()]; exists {
		datapoints := originalMetric.Gauge().DataPoints()
		if datapoints.Len() > 0 {
			d.MemoryMetricsFound = true
			d.aggregatedMemoryMetricAttributes = datapoints.At(0).Attributes()
			d.metricTimestamp = datapoints.At(0).Timestamp()

			for i := 0; i < datapoints.Len(); i++ {
				datapoint := datapoints.At(i)

				neuronCoreIndexValue, neuronCoreIndexValueExists := datapoint.Attributes().Get(NeuronCoreAttributeKey)
				neuronDeviceIndexValue, neuronDeviceIndexValueExists := datapoint.Attributes().Get(NeuronDeviceAttributeKey)

				if neuronCoreIndexValueExists && neuronDeviceIndexValueExists {
					neuronCoreInfo := NeuronCoreInfo{neuronCoreIndex: neuronCoreIndexValue.AsString(), neuronDeviceIndex: neuronDeviceIndexValue.AsString()}

					currentValue, exists := d.memoryMetricValuesAggregator[neuronCoreInfo]
					if exists {
						d.memoryMetricValuesAggregator[neuronCoreInfo] = currentValue + datapoint.DoubleValue()
					} else {
						d.memoryMetricValuesAggregator[neuronCoreInfo] = datapoint.DoubleValue()
					}
				}
			}
		}
	}
}

func (d *MemoryMetricAggregator) FlushAggregatedMemoryMetric() pmetric.Metric {
	aggregatedMemoryMetric := pmetric.NewMetric()
	aggregatedMemoryMetric.SetName(containerinsightscommon.NeuronCoreMemoryUtilizationTotal)
	datapoints := aggregatedMemoryMetric.SetEmptySum().DataPoints()

	for neuronCoreInfo, totalMemoryUsed := range d.memoryMetricValuesAggregator {
		datapoint := datapoints.AppendEmpty()
		datapoint.SetDoubleValue(totalMemoryUsed)
		d.aggregatedMemoryMetricAttributes.CopyTo(datapoint.Attributes())

		datapoint.Attributes().PutStr(NeuronCoreAttributeKey, neuronCoreInfo.neuronCoreIndex)
		datapoint.Attributes().PutStr(NeuronDeviceAttributeKey, neuronCoreInfo.neuronDeviceIndex)
		datapoint.SetTimestamp(d.metricTimestamp)
	}

	// Reset the aggregator
	d.resetMemoryMetricAggregator()
	return aggregatedMemoryMetric
}

func (d *MemoryMetricAggregator) resetMemoryMetricAggregator() {
	d.memoryMetricValuesAggregator = map[NeuronCoreInfo]float64{}
	d.MemoryMetricsFound = false
}
