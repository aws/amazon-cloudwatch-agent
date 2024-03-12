package internal

import (
	"github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"strings"
)

const (
	aggregatedMetricSuffix = "_total"
	logTypeSuffix          = "AwsNeuron"
)

type MetricModifier struct {
	logger *zap.Logger
}

type MetricModifications struct {
	DuplicationTypes         []string
	AttributeKeysToBeRemoved []string
	AggregationAttributeKey  string
	LogTypeSuffix            string
}

var metricModificationsMap = map[string]MetricModifications{
	containerinsightscommon.NeuronExecutionErrors:                       {DuplicationTypes: []string{containerinsightscommon.TypeNode}, AttributeKeysToBeRemoved: []string{"error_type"}, AggregationAttributeKey: "error_type", LogTypeSuffix: ""},
	containerinsightscommon.NeuronExecutionStatus:                       {DuplicationTypes: []string{containerinsightscommon.TypeNode}, AttributeKeysToBeRemoved: []string{"status_type"}, AggregationAttributeKey: "status_type", LogTypeSuffix: ""},
	containerinsightscommon.NeuronRuntimeMemoryUsage:                    {DuplicationTypes: []string{containerinsightscommon.TypeNode}, AttributeKeysToBeRemoved: []string{"memory_location"}, AggregationAttributeKey: "", LogTypeSuffix: ""},
	containerinsightscommon.NeuronCoreMemoryUtilizationConstants:        {DuplicationTypes: []string{containerinsightscommon.TypeContainer, containerinsightscommon.TypePod, containerinsightscommon.TypeNode}, AttributeKeysToBeRemoved: []string{"memory_location"}, AggregationAttributeKey: "", LogTypeSuffix: "Core"},
	containerinsightscommon.NeuronCoreMemoryUtilizationModelCode:        {DuplicationTypes: []string{containerinsightscommon.TypeContainer, containerinsightscommon.TypePod, containerinsightscommon.TypeNode}, AttributeKeysToBeRemoved: []string{"memory_location"}, AggregationAttributeKey: "", LogTypeSuffix: "Core"},
	containerinsightscommon.NeuronCoreMemoryUtilizationSharedScratchpad: {DuplicationTypes: []string{containerinsightscommon.TypeContainer, containerinsightscommon.TypePod, containerinsightscommon.TypeNode}, AttributeKeysToBeRemoved: []string{"memory_location"}, AggregationAttributeKey: "", LogTypeSuffix: "Core"},
	containerinsightscommon.NeuronCoreMemoryUtilizationRuntimeMemory:    {DuplicationTypes: []string{containerinsightscommon.TypeContainer, containerinsightscommon.TypePod, containerinsightscommon.TypeNode}, AttributeKeysToBeRemoved: []string{"memory_location"}, AggregationAttributeKey: "", LogTypeSuffix: "Core"},
	containerinsightscommon.NeuronCoreMemoryUtilizationTensors:          {DuplicationTypes: []string{containerinsightscommon.TypeContainer, containerinsightscommon.TypePod, containerinsightscommon.TypeNode}, AttributeKeysToBeRemoved: []string{"memory_location"}, AggregationAttributeKey: "", LogTypeSuffix: "Core"},
	containerinsightscommon.NeuronCoreUtilization:                       {DuplicationTypes: []string{containerinsightscommon.TypeContainer, containerinsightscommon.TypePod, containerinsightscommon.TypeNode}, AttributeKeysToBeRemoved: []string{}, AggregationAttributeKey: "", LogTypeSuffix: "Core"},
	containerinsightscommon.NeuronInstanceInfo:                          {DuplicationTypes: []string{}, AttributeKeysToBeRemoved: []string{}, AggregationAttributeKey: "", LogTypeSuffix: ""},
	containerinsightscommon.NeuronHardware:                              {DuplicationTypes: []string{}, AttributeKeysToBeRemoved: []string{}, AggregationAttributeKey: "", LogTypeSuffix: ""},
	containerinsightscommon.NeuronExecutionLatency:                      {DuplicationTypes: []string{containerinsightscommon.TypeNode}, AttributeKeysToBeRemoved: []string{"percentile"}, AggregationAttributeKey: "", LogTypeSuffix: ""},
	containerinsightscommon.NeuronDeviceHardwareEccEvents:               {DuplicationTypes: []string{containerinsightscommon.TypeContainer, containerinsightscommon.TypePod, containerinsightscommon.TypeNode}, AttributeKeysToBeRemoved: []string{"event_type"}, AggregationAttributeKey: "event_type", LogTypeSuffix: "Device"},
}

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

	modifiedMetricSlice := pmetric.NewMetricSlice()

	if originalMetricName == containerinsightscommon.NeuronExecutionLatency {
		modifiedMetricSlice = keepSpecificDatapointBasedOnAttribute(originalMetric, metricModificationsMap[originalMetricName].AttributeKeysToBeRemoved[0], "p50")
	} else if originalMetricName == containerinsightscommon.NeuronRuntimeMemoryUsage {
		modifiedMetricSlice = keepSpecificDatapointBasedOnAttribute(originalMetric, metricModificationsMap[originalMetricName].AttributeKeysToBeRemoved[0], "neuron_device")
	} else {
		modifiedMetricSlice = md.createAggregatedSumMetrics(originalMetric)
	}
	filterLabels(modifiedMetricSlice, originalMetricName)
	return md.duplicateMetrics(modifiedMetricSlice, originalMetricName, getMetricDatapoints(originalMetric))
}

func (md *MetricModifier) createAggregatedSumMetrics(originalMetric pmetric.Metric) pmetric.MetricSlice {
	newMetricSlice := pmetric.NewMetricSlice()
	originalMetricDatapoints := getMetricDatapoints(originalMetric)

	if aggregationAttributeKey := metricModificationsMap[originalMetric.Name()].AggregationAttributeKey; aggregationAttributeKey != "" && originalMetric.Type() == pmetric.MetricTypeSum {
		aggregatedMetric := newMetricSlice.AppendEmpty()
		// Creating body for the aggregated metric and add it to the new newMetricSlice
		aggregatedMetric.SetName(originalMetric.Name() + aggregatedMetricSuffix)
		originalMetricDatapoints.At(0).CopyTo(aggregatedMetric.SetEmptySum().DataPoints().AppendEmpty())
		aggregatedValue := 0.0
		for i := 0; i < originalMetricDatapoints.Len(); i++ {
			originalDatapoint := originalMetricDatapoints.At(i)
			md.logger.Info("value type : " + originalDatapoint.ValueType().String())
			aggregatedValue += originalDatapoint.DoubleValue()

			// Creating a new metric from the current datapoint and adding it to the new newMetricSlice
			newNameMetric := newMetricSlice.AppendEmpty()
			originalDatapoint.CopyTo(newNameMetric.SetEmptySum().DataPoints().AppendEmpty())
			subtypeValue, _ := originalDatapoint.Attributes().Get(aggregationAttributeKey)
			newNameMetric.SetName(originalMetric.Name() + "_" + subtypeValue.Str())
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
		podname, exists := originalMetricDatapoints.At(0).Attributes().Get("PodName")
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

	datapoints := getMetricDatapoints(metricCopy)
	for i := 0; i < datapoints.Len(); i++ {
		datapoints.At(i).Attributes().PutStr(containerinsightscommon.MetricType, duplicateType+logTypeSuffix+metricModificationsMap[originalMetricName].LogTypeSuffix)
	}

	return &metricCopy
}

func getMetricDatapoints(m pmetric.Metric) pmetric.NumberDataPointSlice {
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		return m.Gauge().DataPoints()
	case pmetric.MetricTypeSum:
		return m.Sum().DataPoints()
	default:
		return pmetric.NewNumberDataPointSlice()
	}
}
func filterLabels(slice pmetric.MetricSlice, originalMetricName string) {
	for i := 0; i < slice.Len(); i++ {
		m := slice.At(i)
		for _, attributeRemovalKey := range metricModificationsMap[originalMetricName].AttributeKeysToBeRemoved {
			dps := getMetricDatapoints(m)
			for i := 0; i < dps.Len(); i++ {
				dp := dps.At(i)
				dp.Attributes().Remove(attributeRemovalKey)
			}
		}
	}
}

func keepSpecificDatapointBasedOnAttribute(originalMetric pmetric.Metric, attributeKey string, attributeValueToKeep string) pmetric.MetricSlice {
	originalMetricDatapoints := getMetricDatapoints(originalMetric)

	newMetricSlice := pmetric.NewMetricSlice()
	newMetric := newMetricSlice.AppendEmpty()
	newMetric.SetName(originalMetric.Name())
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
	convertedMetric.SetEmptySum()
	originalMetric.Gauge().DataPoints().CopyTo(convertedMetric.Sum().DataPoints())
	return convertedMetric
}
