// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"strings"
)

const (
	aggregatedMetricSuffix = "_total"
	ErrorType              = "error_type"
	StatusType             = "status_type"
	EventType              = "event_type"
	logTypeSuffix          = "AWSNeuron"
	MemoryLocation         = "memory_location"

	Core       = "Core"
	Device     = "Device"
	Percentile = "percentile"
	PodName    = "PodName"
	Count      = "Count"
	Bytes      = "Bytes"
	Seconds    = "Seconds"
	Percent    = "Percent"
)

type MetricModifier struct {
	logger *zap.Logger
}

type MetricModifications struct {
	DuplicationTypes         []string
	AttributeKeysToBeRemoved []string
	AggregationAttributeKey  string
	LogTypeSuffix            string
	Unit                     string
}

var (
	metricModificationsMap = map[string]MetricModifications{
		containerinsightscommon.NeuronExecutionErrors:                       {DuplicationTypes: []string{containerinsightscommon.TypeNode}, AttributeKeysToBeRemoved: []string{ErrorType}, AggregationAttributeKey: ErrorType, LogTypeSuffix: "", Unit: Count},
		containerinsightscommon.NeuronExecutionStatus:                       {DuplicationTypes: []string{containerinsightscommon.TypeNode}, AttributeKeysToBeRemoved: []string{StatusType}, AggregationAttributeKey: StatusType, LogTypeSuffix: "", Unit: Count},
		containerinsightscommon.NeuronRuntimeMemoryUsage:                    {DuplicationTypes: []string{containerinsightscommon.TypeNode}, AttributeKeysToBeRemoved: []string{MemoryLocation}, AggregationAttributeKey: "", LogTypeSuffix: "", Unit: Bytes},
		containerinsightscommon.NeuronCoreMemoryUtilizationConstants:        {DuplicationTypes: []string{containerinsightscommon.TypeContainer, containerinsightscommon.TypePod, containerinsightscommon.TypeNode}, AttributeKeysToBeRemoved: []string{MemoryLocation}, AggregationAttributeKey: "", LogTypeSuffix: Core, Unit: Bytes},
		containerinsightscommon.NeuronCoreMemoryUtilizationModelCode:        {DuplicationTypes: []string{containerinsightscommon.TypeContainer, containerinsightscommon.TypePod, containerinsightscommon.TypeNode}, AttributeKeysToBeRemoved: []string{MemoryLocation}, AggregationAttributeKey: "", LogTypeSuffix: Core, Unit: Bytes},
		containerinsightscommon.NeuronCoreMemoryUtilizationSharedScratchpad: {DuplicationTypes: []string{containerinsightscommon.TypeContainer, containerinsightscommon.TypePod, containerinsightscommon.TypeNode}, AttributeKeysToBeRemoved: []string{MemoryLocation}, AggregationAttributeKey: "", LogTypeSuffix: Core, Unit: Bytes},
		containerinsightscommon.NeuronCoreMemoryUtilizationRuntimeMemory:    {DuplicationTypes: []string{containerinsightscommon.TypeContainer, containerinsightscommon.TypePod, containerinsightscommon.TypeNode}, AttributeKeysToBeRemoved: []string{MemoryLocation}, AggregationAttributeKey: "", LogTypeSuffix: Core, Unit: Bytes},
		containerinsightscommon.NeuronCoreMemoryUtilizationTensors:          {DuplicationTypes: []string{containerinsightscommon.TypeContainer, containerinsightscommon.TypePod, containerinsightscommon.TypeNode}, AttributeKeysToBeRemoved: []string{MemoryLocation}, AggregationAttributeKey: "", LogTypeSuffix: Core, Unit: Bytes},
		containerinsightscommon.NeuronCoreUtilization:                       {DuplicationTypes: []string{containerinsightscommon.TypeContainer, containerinsightscommon.TypePod, containerinsightscommon.TypeNode}, AttributeKeysToBeRemoved: []string{}, AggregationAttributeKey: "", LogTypeSuffix: Core, Unit: Percent},
		containerinsightscommon.NeuronInstanceInfo:                          {DuplicationTypes: []string{}, AttributeKeysToBeRemoved: []string{}, AggregationAttributeKey: "", LogTypeSuffix: "", Unit: Count},
		containerinsightscommon.NeuronHardware:                              {DuplicationTypes: []string{}, AttributeKeysToBeRemoved: []string{}, AggregationAttributeKey: "", LogTypeSuffix: "", Unit: Count},
		containerinsightscommon.NeuronExecutionLatency:                      {DuplicationTypes: []string{containerinsightscommon.TypeNode}, AttributeKeysToBeRemoved: []string{Percentile}, AggregationAttributeKey: "", LogTypeSuffix: "", Unit: Seconds},
		containerinsightscommon.NeuronDeviceHardwareEccEvents:               {DuplicationTypes: []string{containerinsightscommon.TypeContainer, containerinsightscommon.TypePod, containerinsightscommon.TypeNode}, AttributeKeysToBeRemoved: []string{EventType}, AggregationAttributeKey: EventType, LogTypeSuffix: Device, Unit: Count},
	}
	attributeValuePrefixingMap = map[string]string{"NeuronCore": "core", "NeuronDevice": "device"}
)

func NewMetricModifier(logger *zap.Logger) *MetricModifier {
	d := &MetricModifier{
		logger: logger,
	}
	return d
}

func (md *MetricModifier) ModifyMetric(originalMetric pmetric.Metric) pmetric.MetricSlice {
	// only decorate Aws Neuron metrics
	// another option is to separate Aws Neuron in its own pipeline to minimize extra processing of metrics
	newMetricSlice := pmetric.NewMetricSlice()
	if _, isNeuronMetric := metricModificationsMap[originalMetric.Name()]; !isNeuronMetric {
		originalMetric.CopyTo(newMetricSlice.AppendEmpty())
		return newMetricSlice
	}

	originalMetricName := originalMetric.Name()
	if originalMetric.Type() == pmetric.MetricTypeGauge {
		originalMetric = convertGaugeToSum(originalMetric)
	}

	addUnit(originalMetric)
	modifiedMetricSlice := pmetric.NewMetricSlice()

	if originalMetricName == containerinsightscommon.NeuronExecutionLatency {
		modifiedMetricSlice = keepSpecificDatapointBasedOnAttribute(originalMetric, metricModificationsMap[originalMetricName].AttributeKeysToBeRemoved[0], "p50")
	} else if originalMetricName == containerinsightscommon.NeuronRuntimeMemoryUsage {
		modifiedMetricSlice = keepSpecificDatapointBasedOnAttribute(originalMetric, metricModificationsMap[originalMetricName].AttributeKeysToBeRemoved[0], "neuron_device")
	} else {
		modifiedMetricSlice = md.createAggregatedSumMetrics(originalMetric)
	}
	filterLabels(modifiedMetricSlice, originalMetricName)
	return md.duplicateMetrics(modifiedMetricSlice, originalMetricName, originalMetric.Sum().DataPoints())
}

func (md *MetricModifier) createAggregatedSumMetrics(originalMetric pmetric.Metric) pmetric.MetricSlice {
	newMetricSlice := pmetric.NewMetricSlice()
	originalMetricDatapoints := originalMetric.Sum().DataPoints()

	if aggregationAttributeKey := metricModificationsMap[originalMetric.Name()].AggregationAttributeKey; aggregationAttributeKey != "" && originalMetric.Type() == pmetric.MetricTypeSum {
		aggregatedMetric := pmetric.NewMetric()
		if originalMetric.Name() != containerinsightscommon.NeuronDeviceHardwareEccEvents {
			//aggregated metric for ecc error is not required
			aggregatedMetric = newMetricSlice.AppendEmpty()
		}

		// Creating body for the aggregated metric and add it to the new newMetricSlice
		aggregatedMetric.SetName(originalMetric.Name() + aggregatedMetricSuffix)
		aggregatedMetric.SetUnit(originalMetric.Unit())
		originalMetricDatapoints.At(0).CopyTo(aggregatedMetric.SetEmptySum().DataPoints().AppendEmpty())
		aggregatedValue := 0.0
		for i := 0; i < originalMetricDatapoints.Len(); i++ {
			originalDatapoint := originalMetricDatapoints.At(i)
			aggregatedValue += originalDatapoint.DoubleValue()

			// Creating a new metric from the current datapoint and adding it to the new newMetricSlice
			newNameMetric := newMetricSlice.AppendEmpty()
			originalDatapoint.CopyTo(newNameMetric.SetEmptySum().DataPoints().AppendEmpty())
			subtypeValue, _ := originalDatapoint.Attributes().Get(aggregationAttributeKey)
			newNameMetric.SetName(originalMetric.Name() + "_" + subtypeValue.Str())
			newNameMetric.SetUnit(originalMetric.Unit())
			newNameMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		}
		aggregatedMetric.Sum().DataPoints().At(0).SetDoubleValue(aggregatedValue)
		aggregatedMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	} else {
		originalMetric.CopyTo(newMetricSlice.AppendEmpty())
	}

	return newMetricSlice
}

func (md *MetricModifier) duplicateMetrics(metricsSlice pmetric.MetricSlice, originalMetricName string, originalMetricDatapoints pmetric.NumberDataPointSlice) pmetric.MetricSlice {
	newMetricsSlice := pmetric.NewMetricSlice()
	metricModifications := metricModificationsMap[originalMetricName]

	duplicateForNodeOnly := false
	if originalMetricName == containerinsightscommon.NeuronDeviceHardwareEccEvents {
		podname, exists := originalMetricDatapoints.At(0).Attributes().Get(PodName)
		if !exists || len(podname.Str()) == 0 {
			duplicateForNodeOnly = true
		}
	}

	for i := 0; i < metricsSlice.Len(); i++ {
		metric := metricsSlice.At(i)
		if duplicateForNodeOnly {
			duplicateMetricForType(metric, containerinsightscommon.TypeNode, originalMetricName).CopyTo(newMetricsSlice.AppendEmpty())
		} else {
			for _, prefix := range metricModifications.DuplicationTypes {
				duplicateMetricForType(metric, prefix, originalMetricName).CopyTo(newMetricsSlice.AppendEmpty())
			}
		}
	}

	return newMetricsSlice
}

func duplicateMetricForType(metric pmetric.Metric, duplicateType string, originalMetricName string) *pmetric.Metric {
	metricCopy := pmetric.NewMetric()
	metric.CopyTo(metricCopy)
	metricCopy.SetName(strings.ToLower(duplicateType) + "_" + metricCopy.Name())

	datapoints := metricCopy.Sum().DataPoints()
	for i := 0; i < datapoints.Len(); i++ {
		datapoints.At(i).Attributes().PutStr(containerinsightscommon.MetricType, duplicateType+logTypeSuffix+metricModificationsMap[originalMetricName].LogTypeSuffix)
	}

	return &metricCopy
}

func filterLabels(slice pmetric.MetricSlice, originalMetricName string) {
	for i := 0; i < slice.Len(); i++ {
		m := slice.At(i)

		dps := m.Sum().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			for _, attributeRemovalKey := range metricModificationsMap[originalMetricName].AttributeKeysToBeRemoved {
				dp.Attributes().Remove(attributeRemovalKey)
			}
			for attributeKey, attributeValuePrefix := range attributeValuePrefixingMap {
				if value, exists := dp.Attributes().Get(attributeKey); exists {
					dp.Attributes().PutStr(attributeKey, attributeValuePrefix+value.AsString())
				}
			}
		}
	}
}

func keepSpecificDatapointBasedOnAttribute(originalMetric pmetric.Metric, attributeKey string, attributeValueToKeep string) pmetric.MetricSlice {
	originalMetricDatapoints := originalMetric.Sum().DataPoints()

	newMetricSlice := pmetric.NewMetricSlice()
	newMetric := newMetricSlice.AppendEmpty()
	newMetric.SetName(originalMetric.Name())
	newMetric.SetUnit(originalMetric.Unit())
	datapoint := newMetric.SetEmptySum().DataPoints().AppendEmpty()

	for i := 0; i < originalMetricDatapoints.Len(); i++ {
		dp := originalMetricDatapoints.At(i)
		if value, exists := dp.Attributes().Get(attributeKey); exists && value.AsString() == attributeValueToKeep {
			dp.CopyTo(datapoint)
			break
		}
	}

	return newMetricSlice
}

func convertGaugeToSum(originalMetric pmetric.Metric) pmetric.Metric {
	convertedMetric := pmetric.NewMetric()
	convertedMetric.SetName(originalMetric.Name())
	convertedMetric.SetUnit(originalMetric.Unit())
	convertedMetric.SetEmptySum()
	originalMetric.Gauge().DataPoints().CopyTo(convertedMetric.Sum().DataPoints())
	return convertedMetric
}

func addUnit(originalMetric pmetric.Metric) {
	originalMetric.SetUnit(metricModificationsMap[originalMetric.Name()].Unit)
}
