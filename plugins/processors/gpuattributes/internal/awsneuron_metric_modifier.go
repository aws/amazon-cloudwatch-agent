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

	Core                            = "Core"
	Device                          = "Device"
	Percentile                      = "percentile"
	PodName                         = "PodName"
	Count                           = "Count"
	Bytes                           = "Bytes"
	Seconds                         = "Seconds"
	Percent                         = "Percent"
	NeuronCoreAttributeKey          = "NeuronCore"
	duplicateNeuronCoreAttributeKey = "neuroncore"
	NeuronDeviceAttributeKey        = "NeuronDevice"
	RuntimeTag                      = "runtime_tag"
)

type AwsNeuronMetricModifier struct {
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
		containerinsightscommon.NeuronCoreMemoryUtilizationTotal:            {DuplicationTypes: []string{containerinsightscommon.TypeContainer, containerinsightscommon.TypePod, containerinsightscommon.TypeNode}, AttributeKeysToBeRemoved: []string{MemoryLocation, duplicateNeuronCoreAttributeKey}, AggregationAttributeKey: "", LogTypeSuffix: Core, Unit: Bytes},
		containerinsightscommon.NeuronCoreMemoryUtilizationConstants:        {DuplicationTypes: []string{containerinsightscommon.TypeContainer, containerinsightscommon.TypePod, containerinsightscommon.TypeNode}, AttributeKeysToBeRemoved: []string{MemoryLocation, duplicateNeuronCoreAttributeKey}, AggregationAttributeKey: "", LogTypeSuffix: Core, Unit: Bytes},
		containerinsightscommon.NeuronCoreMemoryUtilizationModelCode:        {DuplicationTypes: []string{containerinsightscommon.TypeContainer, containerinsightscommon.TypePod, containerinsightscommon.TypeNode}, AttributeKeysToBeRemoved: []string{MemoryLocation, duplicateNeuronCoreAttributeKey}, AggregationAttributeKey: "", LogTypeSuffix: Core, Unit: Bytes},
		containerinsightscommon.NeuronCoreMemoryUtilizationSharedScratchpad: {DuplicationTypes: []string{containerinsightscommon.TypeContainer, containerinsightscommon.TypePod, containerinsightscommon.TypeNode}, AttributeKeysToBeRemoved: []string{MemoryLocation, duplicateNeuronCoreAttributeKey}, AggregationAttributeKey: "", LogTypeSuffix: Core, Unit: Bytes},
		containerinsightscommon.NeuronCoreMemoryUtilizationRuntimeMemory:    {DuplicationTypes: []string{containerinsightscommon.TypeContainer, containerinsightscommon.TypePod, containerinsightscommon.TypeNode}, AttributeKeysToBeRemoved: []string{MemoryLocation, duplicateNeuronCoreAttributeKey}, AggregationAttributeKey: "", LogTypeSuffix: Core, Unit: Bytes},
		containerinsightscommon.NeuronCoreMemoryUtilizationTensors:          {DuplicationTypes: []string{containerinsightscommon.TypeContainer, containerinsightscommon.TypePod, containerinsightscommon.TypeNode}, AttributeKeysToBeRemoved: []string{MemoryLocation, duplicateNeuronCoreAttributeKey}, AggregationAttributeKey: "", LogTypeSuffix: Core, Unit: Bytes},
		containerinsightscommon.NeuronCoreUtilization:                       {DuplicationTypes: []string{containerinsightscommon.TypeContainer, containerinsightscommon.TypePod, containerinsightscommon.TypeNode}, AttributeKeysToBeRemoved: []string{duplicateNeuronCoreAttributeKey}, AggregationAttributeKey: "", LogTypeSuffix: Core, Unit: Percent},
		containerinsightscommon.NeuronInstanceInfo:                          {DuplicationTypes: []string{}, AttributeKeysToBeRemoved: []string{}, AggregationAttributeKey: "", LogTypeSuffix: "", Unit: Count},
		containerinsightscommon.NeuronHardware:                              {DuplicationTypes: []string{}, AttributeKeysToBeRemoved: []string{}, AggregationAttributeKey: "", LogTypeSuffix: "", Unit: Count},
		containerinsightscommon.NeuronExecutionLatency:                      {DuplicationTypes: []string{containerinsightscommon.TypeNode}, AttributeKeysToBeRemoved: []string{Percentile}, AggregationAttributeKey: "", LogTypeSuffix: "", Unit: Seconds},
		containerinsightscommon.NeuronDeviceHardwareEccEvents:               {DuplicationTypes: []string{containerinsightscommon.TypeContainer, containerinsightscommon.TypePod, containerinsightscommon.TypeNode}, AttributeKeysToBeRemoved: []string{EventType}, AggregationAttributeKey: EventType, LogTypeSuffix: Device, Unit: Count},
	}
	attributeValuePrefixingMap = map[string]string{NeuronCoreAttributeKey: "core", NeuronDeviceAttributeKey: "device"}
)

func NewMetricModifier(logger *zap.Logger) *AwsNeuronMetricModifier {
	d := &AwsNeuronMetricModifier{
		logger: logger,
	}
	return d
}

func (md *AwsNeuronMetricModifier) ModifyMetric(originalMetric pmetric.Metric) pmetric.MetricSlice {
	// only decorate Aws Neuron metrics
	// another option is to separate Aws Neuron in its own pipeline to minimize extra processing of metrics
	newMetricSlice := pmetric.NewMetricSlice()
	if _, isNeuronMetric := metricModificationsMap[originalMetric.Name()]; !isNeuronMetric {
		originalMetric.CopyTo(newMetricSlice.AppendEmpty())
		return newMetricSlice
	}

	originalMetricName := originalMetric.Name()

	// Since the otel to grouped metrics conversions takes type into account,
	// thus we need to convert all metrics to the same type so that they are grouped together.
	if originalMetric.Type() == pmetric.MetricTypeGauge {
		originalMetric = convertGaugeToSum(originalMetric)
	}

	// Neuron metrics sent by the neuron monitor don't have any units so we add them in the agent.
	addUnit(originalMetric)
	modifiedMetricSlice := pmetric.NewMetricSlice()

	// The neuron metrics sent by the neuron monitor are not homogeneous
	// and some metrics require special processing.
	// We perform those special processing before duplicating metric for pod, node and container.
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

// This method takes a metric and creates an aggregated metric from its datapoint values.
// It also creates a new metric for each datapoint based on the target attribute.
func (md *AwsNeuronMetricModifier) createAggregatedSumMetrics(originalMetric pmetric.Metric) pmetric.MetricSlice {
	newMetricSlice := pmetric.NewMetricSlice()
	originalMetricDatapoints := originalMetric.Sum().DataPoints()

	if aggregationAttributeKey := metricModificationsMap[originalMetric.Name()].AggregationAttributeKey; aggregationAttributeKey != "" && originalMetric.Type() == pmetric.MetricTypeSum {
		aggregatedValuesPerRuntimeTag := map[string]float64{}

		for i := 0; i < originalMetricDatapoints.Len(); i++ {
			originalDatapoint := originalMetricDatapoints.At(i)

			runtimeTag, _ := originalDatapoint.Attributes().Get(RuntimeTag)
			aggregatedValue, _ := aggregatedValuesPerRuntimeTag[runtimeTag.AsString()]
			aggregatedValue += originalDatapoint.DoubleValue()
			aggregatedValuesPerRuntimeTag[runtimeTag.AsString()] = aggregatedValue

			// Creating a new metric from the current datapoint and adding it to the new newMetricSlice
			newNameMetric := newMetricSlice.AppendEmpty()
			originalDatapoint.CopyTo(newNameMetric.SetEmptySum().DataPoints().AppendEmpty())
			subtypeValue, _ := originalDatapoint.Attributes().Get(aggregationAttributeKey)
			newNameMetric.SetName(originalMetric.Name() + "_" + subtypeValue.Str())
			newNameMetric.SetUnit(originalMetric.Unit())
			// setting value of temporality to cumulative so that agent performs delta conversion on this metric
			newNameMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		}

		if originalMetric.Name() != containerinsightscommon.NeuronDeviceHardwareEccEvents {
			// Creating body for the aggregated metric and add it to the new newMetricSlice for each runtime
			for runtimeTag, value := range aggregatedValuesPerRuntimeTag {
				// Aggregated metric for neuron device ecc events is not required
				aggregatedMetric := newMetricSlice.AppendEmpty()

				aggregatedMetric.SetName(originalMetric.Name() + aggregatedMetricSuffix)
				aggregatedMetric.SetUnit(originalMetric.Unit())

				originalMetricDatapoints.At(0).CopyTo(aggregatedMetric.SetEmptySum().DataPoints().AppendEmpty())
				aggregatedMetric.Sum().DataPoints().At(0).SetDoubleValue(value)
				aggregatedMetric.Sum().DataPoints().At(0).Attributes().PutStr(RuntimeTag, runtimeTag)

				// setting value of temporality to cumulative so that agent performs delta conversion on this metric
				aggregatedMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
			}
		}
	} else {
		originalMetric.CopyTo(newMetricSlice.AppendEmpty())
	}

	return newMetricSlice
}

// This method duplicates metrics performs selective duplication of a metric based on the types for which duplication needs to be performed
// and by checking that pod correlation has been performed before duplicating metrics for pod and container.
func (md *AwsNeuronMetricModifier) duplicateMetrics(metricsSlice pmetric.MetricSlice, originalMetricName string, originalMetricDatapoints pmetric.NumberDataPointSlice) pmetric.MetricSlice {
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

// This method creates new metrics by prefixing the metric name with each k8 concepts (pod, node and container)
// and adding logTypes to the attributes
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

// This method performs the following removal and update operations on a datapoint's attributes:
// 1. It removes the attribute keys which are not required. The removal is necessary so that the metrics are grouped thogether
// 2. It prefixes NeuronCore and NeuronDevice values with `core` and `device` respectively.
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

// This method keeps a specific datapoint in the list of datapoints,
// filtering out the rest based on value of the target attribute.
// - For neuron_execution_latency metric we keep p50 percentile
// - For neurondevice_runtime_memory we keep the neuron_device memory datapoint
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

	// default value of temporality is undefined so even after conversion from gauge to sum
	// the agent won't take delta.
	return convertedMetric
}

func addUnit(originalMetric pmetric.Metric) {
	originalMetric.SetUnit(metricModificationsMap[originalMetric.Name()].Unit)
}
