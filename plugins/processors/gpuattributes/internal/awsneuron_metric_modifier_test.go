// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

var staticAttributes = map[string]any{
	ClusterName:      "dummyAttribute",
	InstanceId:       "dummyAttribute",
	InstanceType:     "dummyAttribute",
	NodeName:         "dummyAttribute",
	AvailabilityZone: "dummyAttribute",
	Kubernetes:       "dummyAttribute",
	RuntimeTag:       "dummyAttribute",
	SubnetId:         "dummyAttribute",
}
var staticTimestamp = pcommon.NewTimestampFromTime(time.Date(2023, time.March, 12, 11, 0, 0, 0, time.UTC))

const (
	NonNeuronMetric                            = "non_neuron_metric"
	NeuronCoreMemoryUsageModelSharedScratchpad = "neuroncore_memory_usage_model_shared_scratchpad"
	NeuronDeviceRuntimeMemoryUsedBytes         = "neurondevice_runtime_memory_used_bytes"
	NeuronExecutionLatency                     = "neuron_execution_latency"
	DummyPod                                   = "DummyPod"
	Type                                       = "Type"
	NodeAWSNeuronCore                          = "NodeAWSNeuronCore"
	PodAWSNeuronCore                           = "PodAWSNeuronCore"
	ContainerAWSNeuronCore                     = "ContainerAWSNeuronCore"
	NodeAWSNeuron                              = "NodeAWSNeuron"
)

type MetricDefinition struct {
	MetricType        pmetric.MetricType
	MetricValues      []float64
	SpecialAttributes [][]string
	Unit              string
}

var metricNameToMetricLayout = map[string]MetricDefinition{
	NonNeuronMetric: {MetricType: pmetric.MetricTypeGauge, MetricValues: []float64{1}, SpecialAttributes: [][]string{}, Unit: Count},
	NeuronCoreMemoryUsageModelSharedScratchpad: {MetricType: pmetric.MetricTypeGauge, MetricValues: []float64{1, 2, 3}, SpecialAttributes: [][]string{{NeuronCore, "0", NeuronDevice, "0", MemoryLocation, "None", PodName, DummyPod}, {NeuronCore, "1", NeuronDevice, "0", MemoryLocation, "None", PodName, DummyPod}, {NeuronCore, "2", NeuronDevice, "1", MemoryLocation, "None", PodName, DummyPod}}, Unit: Bytes},
	NeuronDeviceRuntimeMemoryUsedBytes:         {MetricType: pmetric.MetricTypeGauge, MetricValues: []float64{1, 2}, SpecialAttributes: [][]string{{MemoryLocation, "host"}, {MemoryLocation, "neuron_device"}}, Unit: Bytes},
	NeuronExecutionLatency:                     {MetricType: pmetric.MetricTypeGauge, MetricValues: []float64{0, 0, 0, 0, 1, 0, 0}, SpecialAttributes: [][]string{{Percentile, "p0"}, {Percentile, "p1"}, {Percentile, "p100"}, {Percentile, "p25"}, {Percentile, "p50"}, {Percentile, "p75"}, {Percentile, "p99"}}, Unit: Seconds},
}

func setupMetricModifier() *AwsNeuronMetricModifier {
	logger, _ := zap.NewDevelopment()
	return &AwsNeuronMetricModifier{logger: logger}
}
func TestMetricModifierForExecutionLatencyMetric(t *testing.T) {
	metricModifier := setupMetricModifier()
	metricsList := pmetric.NewMetricSlice()
	createActualMetricForKey(NeuronExecutionLatency).CopyTo(metricsList.AppendEmpty())
	metricModifier.ModifyMetric(metricsList.At(0), metricsList)

	expectedMetrics := map[string]pmetric.Metric{
		NeuronExecutionLatency:          metricsList.At(0),
		"node_neuron_execution_latency": createExpectedMetric("node_neuron_execution_latency", false, []map[string]string{{Type: NodeAWSNeuron}}, []float64{1}, pmetric.MetricTypeSum, Seconds),
	}

	assertModifiedMetric(t, metricsList, expectedMetrics)
}

func TestMetricModifierForNeuronCoreMemoryUsageMetric(t *testing.T) {
	metricModifier := setupMetricModifier()
	metricsList := pmetric.NewMetricSlice()
	createActualMetricForKey(NeuronCoreMemoryUsageModelSharedScratchpad).CopyTo(metricsList.AppendEmpty())
	metricModifier.ModifyMetric(metricsList.At(0), metricsList)

	expectedMetrics := map[string]pmetric.Metric{
		NeuronCoreMemoryUsageModelSharedScratchpad:                  metricsList.At(0),
		"node_neuroncore_memory_usage_model_shared_scratchpad":      createExpectedMetric("node_neuroncore_memory_usage_model_shared_scratchpad", false, []map[string]string{{NeuronCore: "core0", NeuronDevice: "device0", Type: NodeAWSNeuronCore, PodName: DummyPod}, {NeuronCore: "core1", NeuronDevice: "device0", Type: NodeAWSNeuronCore, PodName: DummyPod}, {NeuronCore: "core2", NeuronDevice: "device1", Type: NodeAWSNeuronCore, PodName: DummyPod}}, []float64{1, 2, 3}, pmetric.MetricTypeSum, Bytes),
		"pod_neuroncore_memory_usage_model_shared_scratchpad":       createExpectedMetric("pod_neuroncore_memory_usage_model_shared_scratchpad", false, []map[string]string{{NeuronCore: "core0", NeuronDevice: "device0", Type: PodAWSNeuronCore, PodName: DummyPod}, {NeuronCore: "core1", NeuronDevice: "device0", Type: PodAWSNeuronCore, PodName: DummyPod}, {NeuronCore: "core2", NeuronDevice: "device1", Type: PodAWSNeuronCore, PodName: DummyPod}}, []float64{1, 2, 3}, pmetric.MetricTypeSum, Bytes),
		"container_neuroncore_memory_usage_model_shared_scratchpad": createExpectedMetric("container_neuroncore_memory_usage_model_shared_scratchpad", false, []map[string]string{{NeuronCore: "core0", NeuronDevice: "device0", Type: ContainerAWSNeuronCore, PodName: DummyPod}, {NeuronCore: "core1", NeuronDevice: "device0", Type: ContainerAWSNeuronCore, PodName: DummyPod}, {NeuronCore: "core2", NeuronDevice: "device1", Type: ContainerAWSNeuronCore, PodName: DummyPod}}, []float64{1, 2, 3}, pmetric.MetricTypeSum, Bytes),
	}

	assertModifiedMetric(t, metricsList, expectedMetrics)
}

func TestMetricModifierForNeuronCoreMemoryUsageMetric_PodNameMissing(t *testing.T) {
	metricModifier := setupMetricModifier()
	metricsList := pmetric.NewMetricSlice()
	removeAttributefromMetric(createActualMetricForKey(NeuronCoreMemoryUsageModelSharedScratchpad), PodName).CopyTo(metricsList.AppendEmpty())
	metricModifier.ModifyMetric(metricsList.At(0), metricsList)

	expectedMetrics := map[string]pmetric.Metric{
		NeuronCoreMemoryUsageModelSharedScratchpad:             metricsList.At(0),
		"node_neuroncore_memory_usage_model_shared_scratchpad": createExpectedMetric("node_neuroncore_memory_usage_model_shared_scratchpad", false, []map[string]string{{NeuronCore: "core0", NeuronDevice: "device0", Type: NodeAWSNeuronCore}, {NeuronCore: "core1", NeuronDevice: "device0", Type: NodeAWSNeuronCore}, {NeuronCore: "core2", NeuronDevice: "device1", Type: NodeAWSNeuronCore}}, []float64{1, 2, 3}, pmetric.MetricTypeSum, Bytes),
	}

	assertModifiedMetric(t, metricsList, expectedMetrics)
}

func TestMetricModifierForNeuronDeviceRuntimeMemoryUsageMetric(t *testing.T) {
	metricModifier := setupMetricModifier()
	metricsList := pmetric.NewMetricSlice()
	createActualMetricForKey(NeuronDeviceRuntimeMemoryUsedBytes).CopyTo(metricsList.AppendEmpty())
	metricModifier.ModifyMetric(metricsList.At(0), metricsList)

	expectedMetrics := map[string]pmetric.Metric{
		NeuronDeviceRuntimeMemoryUsedBytes:            metricsList.At(0),
		"node_neurondevice_runtime_memory_used_bytes": createExpectedMetric("node_neurondevice_runtime_memory_used_bytes", false, []map[string]string{{Type: NodeAWSNeuron}}, []float64{2}, pmetric.MetricTypeSum, Bytes),
	}

	assertModifiedMetric(t, metricsList, expectedMetrics)
}

func TestMetricModifierForNonNeuronMonitorMetric(t *testing.T) {
	metricModifier := setupMetricModifier()
	metricsList := pmetric.NewMetricSlice()
	createActualMetricForKey(NonNeuronMetric).CopyTo(metricsList.AppendEmpty())
	metricModifier.ModifyMetric(metricsList.At(0), metricsList)

	expectedMetrics := map[string]pmetric.Metric{
		NonNeuronMetric: metricsList.At(0),
	}

	assertModifiedMetric(t, metricsList, expectedMetrics)
}

func TestListWithMultipleMetrics(t *testing.T) {
	metricModifier := setupMetricModifier()
	metricsList := pmetric.NewMetricSlice()
	createActualMetricForKey(NeuronExecutionLatency).CopyTo(metricsList.AppendEmpty())
	createActualMetricForKey(NeuronCoreMemoryUsageModelSharedScratchpad).CopyTo(metricsList.AppendEmpty())
	createActualMetricForKey(NeuronDeviceRuntimeMemoryUsedBytes).CopyTo(metricsList.AppendEmpty())
	createActualMetricForKey(NonNeuronMetric).CopyTo(metricsList.AppendEmpty())

	for i := 0; i < metricsList.Len(); i++ {
		metricModifier.ModifyMetric(metricsList.At(i), metricsList)
	}

	expectedMetrics := map[string]pmetric.Metric{
		NeuronExecutionLatency:                     metricsList.At(0),
		NeuronCoreMemoryUsageModelSharedScratchpad: metricsList.At(1),
		NeuronDeviceRuntimeMemoryUsedBytes:         metricsList.At(2),
		NonNeuronMetric:                            metricsList.At(3),

		"node_neuron_execution_latency": createExpectedMetric("node_neuron_execution_latency", false, []map[string]string{{Type: NodeAWSNeuron}}, []float64{1}, pmetric.MetricTypeSum, Seconds),

		"node_neuroncore_memory_usage_model_shared_scratchpad":      createExpectedMetric("node_neuroncore_memory_usage_model_shared_scratchpad", false, []map[string]string{{NeuronCore: "core0", NeuronDevice: "device0", Type: NodeAWSNeuronCore, PodName: DummyPod}, {NeuronCore: "core1", NeuronDevice: "device0", Type: NodeAWSNeuronCore, PodName: DummyPod}, {NeuronCore: "core2", NeuronDevice: "device1", Type: NodeAWSNeuronCore, PodName: DummyPod}}, []float64{1, 2, 3}, pmetric.MetricTypeSum, Bytes),
		"pod_neuroncore_memory_usage_model_shared_scratchpad":       createExpectedMetric("pod_neuroncore_memory_usage_model_shared_scratchpad", false, []map[string]string{{NeuronCore: "core0", NeuronDevice: "device0", Type: PodAWSNeuronCore, PodName: DummyPod}, {NeuronCore: "core1", NeuronDevice: "device0", Type: PodAWSNeuronCore, PodName: DummyPod}, {NeuronCore: "core2", NeuronDevice: "device1", Type: PodAWSNeuronCore, PodName: DummyPod}}, []float64{1, 2, 3}, pmetric.MetricTypeSum, Bytes),
		"container_neuroncore_memory_usage_model_shared_scratchpad": createExpectedMetric("container_neuroncore_memory_usage_model_shared_scratchpad", false, []map[string]string{{NeuronCore: "core0", NeuronDevice: "device0", Type: ContainerAWSNeuronCore, PodName: DummyPod}, {NeuronCore: "core1", NeuronDevice: "device0", Type: ContainerAWSNeuronCore, PodName: DummyPod}, {NeuronCore: "core2", NeuronDevice: "device1", Type: ContainerAWSNeuronCore, PodName: DummyPod}}, []float64{1, 2, 3}, pmetric.MetricTypeSum, Bytes),

		"node_neurondevice_runtime_memory_used_bytes": createExpectedMetric("node_neurondevice_runtime_memory_used_bytes", false, []map[string]string{{Type: NodeAWSNeuron}}, []float64{2}, pmetric.MetricTypeSum, Bytes),
	}
	assertModifiedMetric(t, metricsList, expectedMetrics)
}

func createActualMetricForKey(key string) pmetric.Metric {
	metricDefinition := metricNameToMetricLayout[key]

	metric := pmetric.NewMetric()
	metric.SetName(key)
	metric.SetUnit(metricDefinition.Unit)
	datapoints := pmetric.NumberDataPointSlice{}
	if metricDefinition.MetricType == pmetric.MetricTypeGauge {
		datapoints = metric.SetEmptyGauge().DataPoints()
	} else {
		datapoints = metric.SetEmptySum().DataPoints()
	}

	for i := 0; i < len(metricDefinition.MetricValues); i++ {
		datapoint := datapoints.AppendEmpty()
		datapoint.SetDoubleValue(metricDefinition.MetricValues[i])
		datapoint.SetTimestamp(staticTimestamp)
		datapoint.Attributes().FromRaw(staticAttributes)

		if len(metricDefinition.SpecialAttributes) > 0 {
			for j := 0; j < len(metricDefinition.SpecialAttributes[i])-1; j = j + 2 {
				datapoint.Attributes().PutStr(metricDefinition.SpecialAttributes[i][j], metricDefinition.SpecialAttributes[i][j+1])
			}
		}
	}

	return metric
}

func assertModifiedMetric(t *testing.T, actualSlice pmetric.MetricSlice, expectedMetrics map[string]pmetric.Metric) {
	assert.Equal(t, len(expectedMetrics), actualSlice.Len())
	for i := 0; i < actualSlice.Len(); i++ {
		actualMetric := actualSlice.At(i)
		expectedMetric, exists := expectedMetrics[actualMetric.Name()]

		assert.True(t, exists)
		assert.Equal(t, expectedMetric.Name(), actualMetric.Name())
		assert.Equal(t, expectedMetric.Type(), actualMetric.Type())
		assert.Equal(t, expectedMetric.Unit(), actualMetric.Unit())

		actualDatapoints := pmetric.NumberDataPointSlice{}
		expectedDatapoints := pmetric.NumberDataPointSlice{}
		if actualMetric.Type() == pmetric.MetricTypeGauge {
			actualDatapoints = actualMetric.Gauge().DataPoints()
			expectedDatapoints = expectedMetric.Gauge().DataPoints()
		} else {
			actualDatapoints = actualMetric.Sum().DataPoints()
			expectedDatapoints = expectedMetric.Sum().DataPoints()
		}

		assert.Equal(t, expectedDatapoints.Len(), actualDatapoints.Len())

		for j := 0; j < actualDatapoints.Len(); j++ {
			actualDatapoint := actualDatapoints.At(j)
			expectedDatapoint := expectedDatapoints.At(j)

			assert.Equal(t, expectedDatapoint.Attributes().Len(), actualDatapoint.Attributes().Len())
			for key, val := range actualDatapoint.Attributes().AsRaw() {
				expectedVal, _ := expectedDatapoint.Attributes().Get(key)
				assert.Equal(t, expectedVal.AsString(), val)
			}

			assert.Equal(t, expectedDatapoint.ValueType(), actualDatapoint.ValueType())
			assert.Equal(t, expectedDatapoint.DoubleValue(), actualDatapoint.DoubleValue())
			assert.Equal(t, expectedDatapoint.Timestamp(), actualDatapoint.Timestamp())
		}
	}
}

func createExpectedMetric(name string, isCumulative bool, attributes []map[string]string, values []float64, metricType pmetric.MetricType, unit string) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	metric.SetUnit(unit)

	datapoints := metric.SetEmptySum().DataPoints()
	if metricType == pmetric.MetricTypeGauge {
		datapoints = metric.SetEmptyGauge().DataPoints()
	}

	if isCumulative {
		metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	}

	for i := 0; i < len(values); i++ {
		datapoint := datapoints.AppendEmpty()
		datapoint.SetTimestamp(staticTimestamp)
		datapoint.SetDoubleValue(values[i])
		datapoint.Attributes().FromRaw(staticAttributes)

		for key, val := range attributes[i] {
			datapoint.Attributes().PutStr(key, val)
		}
	}

	return metric
}

func removeAttributefromMetric(metric pmetric.Metric, key string) pmetric.Metric {
	datapoints := pmetric.NewNumberDataPointSlice()
	if metric.Type() == pmetric.MetricTypeGauge {
		datapoints = metric.Gauge().DataPoints()
	} else {
		datapoints = metric.Sum().DataPoints()
	}

	for i := 0; i < datapoints.Len(); i++ {
		datapoints.At(i).Attributes().Remove(key)
	}
	return metric
}
