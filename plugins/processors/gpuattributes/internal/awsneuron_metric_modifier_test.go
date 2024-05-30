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
	"golang.org/x/exp/maps"
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
	NeuronExecutionErrors                      = "neuron_execution_errors"
	NeuronExecutionStatus                      = "neuron_execution_status"
	NeuronCoreMemoryUsageModelSharedScratchpad = "neuroncore_memory_usage_model_shared_scratchpad"
	NeuronDeviceRuntimeMemoryUsedBytes         = "neurondevice_runtime_memory_used_bytes"
	NeuronExecutionLatency                     = "neuron_execution_latency"
	NeuronDeviceHwEccEvents                    = "neurondevice_hw_ecc_events"
	NeuronDeviceIndex                          = "neuron_device_index"
	DummyPod                                   = "DummyPod"
	Type                                       = "Type"
	NodeAWSNeuronDevice                        = "NodeAWSNeuronDevice"
	PodAWSNeuronDevice                         = "PodAWSNeuronDevice"
	ContainerAWSNeuronDevice                   = "ContainerAWSNeuronDevice"
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
	NonNeuronMetric:                            {MetricType: pmetric.MetricTypeGauge, MetricValues: []float64{1}, SpecialAttributes: [][]string{}, Unit: Count},
	NeuronExecutionErrors:                      {MetricType: pmetric.MetricTypeSum, MetricValues: []float64{1, 2, 3, 4, 5, 6}, SpecialAttributes: [][]string{{ErrorType, "generic", RuntimeTag, "1"}, {ErrorType, "numerical", RuntimeTag, "1"}, {ErrorType, "transient", RuntimeTag, "1"}, {ErrorType, "model", RuntimeTag, "1"}, {ErrorType, "runtime", RuntimeTag, "1"}, {ErrorType, "hardware", RuntimeTag, "1"}}, Unit: Count},
	NeuronExecutionStatus:                      {MetricType: pmetric.MetricTypeSum, MetricValues: []float64{1, 2, 3, 4, 5, 6}, SpecialAttributes: [][]string{{StatusType, "completed", RuntimeTag, "1"}, {StatusType, "completed_with_err", RuntimeTag, "1"}, {StatusType, "completed_with_num_err", RuntimeTag, "1"}, {StatusType, "timed_out", RuntimeTag, "1"}, {StatusType, "incorrect_input", RuntimeTag, "1"}, {StatusType, "failed_to_queue", RuntimeTag, "1"}}, Unit: Count},
	NeuronCoreMemoryUsageModelSharedScratchpad: {MetricType: pmetric.MetricTypeGauge, MetricValues: []float64{1, 2, 3}, SpecialAttributes: [][]string{{NeuronCore, "0", NeuronDevice, "0", MemoryLocation, "None", PodName, DummyPod}, {NeuronCore, "1", NeuronDevice, "0", MemoryLocation, "None", PodName, DummyPod}, {NeuronCore, "2", NeuronDevice, "1", MemoryLocation, "None", PodName, DummyPod}}, Unit: Bytes},
	NeuronDeviceRuntimeMemoryUsedBytes:         {MetricType: pmetric.MetricTypeGauge, MetricValues: []float64{1, 2}, SpecialAttributes: [][]string{{MemoryLocation, "host"}, {MemoryLocation, "neuron_device"}}, Unit: Bytes},
	NeuronExecutionLatency:                     {MetricType: pmetric.MetricTypeGauge, MetricValues: []float64{0, 0, 0, 0, 1, 0, 0}, SpecialAttributes: [][]string{{Percentile, "p0"}, {Percentile, "p1"}, {Percentile, "p100"}, {Percentile, "p25"}, {Percentile, "p50"}, {Percentile, "p75"}, {Percentile, "p99"}}, Unit: Seconds},
	NeuronDeviceHwEccEvents:                    {MetricType: pmetric.MetricTypeSum, MetricValues: []float64{1, 2, 3, 4}, SpecialAttributes: [][]string{{NeuronDeviceIndex, "1", NeuronDevice, "1", EventType, "mem_ecc_corrected", PodName, DummyPod, RuntimeTag, "1"}, {NeuronDeviceIndex, "1", NeuronDevice, "1", EventType, "mem_ecc_uncorrected", PodName, DummyPod, RuntimeTag, "1"}, {NeuronDeviceIndex, "1", NeuronDevice, "1", EventType, "sram_ecc_corrected", PodName, DummyPod, RuntimeTag, "1"}, {NeuronDeviceIndex, "1", NeuronDevice, "1", EventType, "sram_ecc_uncorrected", PodName, DummyPod, RuntimeTag, "1"}}, Unit: Count},
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
func TestMetricModifierForExecutionErrorMetric(t *testing.T) {
	metricModifier := setupMetricModifier()
	metricsList := pmetric.NewMetricSlice()
	createActualMetricForKey(NeuronExecutionErrors).CopyTo(metricsList.AppendEmpty())
	metricModifier.ModifyMetric(metricsList.At(0), metricsList)

	expectedMetrics := map[string]pmetric.Metric{
		NeuronExecutionErrors:                    metricsList.At(0),
		"node_neuron_execution_errors_generic":   createExpectedMetric("node_neuron_execution_errors_generic", true, []map[string]string{{Type: NodeAWSNeuron, RuntimeTag: "1"}}, []float64{1}, pmetric.MetricTypeSum, Count),
		"node_neuron_execution_errors_numerical": createExpectedMetric("node_neuron_execution_errors_numerical", true, []map[string]string{{Type: NodeAWSNeuron, RuntimeTag: "1"}}, []float64{2}, pmetric.MetricTypeSum, Count),
		"node_neuron_execution_errors_transient": createExpectedMetric("node_neuron_execution_errors_transient", true, []map[string]string{{Type: NodeAWSNeuron, RuntimeTag: "1"}}, []float64{3}, pmetric.MetricTypeSum, Count),
		"node_neuron_execution_errors_model":     createExpectedMetric("node_neuron_execution_errors_model", true, []map[string]string{{Type: NodeAWSNeuron, RuntimeTag: "1"}}, []float64{4}, pmetric.MetricTypeSum, Count),
		"node_neuron_execution_errors_runtime":   createExpectedMetric("node_neuron_execution_errors_runtime", true, []map[string]string{{Type: NodeAWSNeuron, RuntimeTag: "1"}}, []float64{5}, pmetric.MetricTypeSum, Count),
		"node_neuron_execution_errors_hardware":  createExpectedMetric("node_neuron_execution_errors_hardware", true, []map[string]string{{Type: NodeAWSNeuron, RuntimeTag: "1"}}, []float64{6}, pmetric.MetricTypeSum, Count),
		"node_neuron_execution_errors_total":     createExpectedMetric("node_neuron_execution_errors_total", true, []map[string]string{{Type: NodeAWSNeuron, RuntimeTag: "1"}}, []float64{21}, pmetric.MetricTypeSum, Count),
	}

	assertModifiedMetric(t, metricsList, expectedMetrics)
}

func TestMetricModifierForExecutionStatusMetric(t *testing.T) {
	metricModifier := setupMetricModifier()
	metricsList := pmetric.NewMetricSlice()
	createActualMetricForKey(NeuronExecutionStatus).CopyTo(metricsList.AppendEmpty())
	metricModifier.ModifyMetric(metricsList.At(0), metricsList)

	expectedMap := maps.Clone(staticAttributes)
	expectedMap[Type] = NodeAWSNeuron

	expectedMetrics := map[string]pmetric.Metric{
		NeuronExecutionStatus:                                 metricsList.At(0),
		"node_neuron_execution_status_completed":              createExpectedMetric("node_neuron_execution_status_completed", true, []map[string]string{{Type: NodeAWSNeuron, RuntimeTag: "1"}}, []float64{1}, pmetric.MetricTypeSum, Count),
		"node_neuron_execution_status_completed_with_err":     createExpectedMetric("node_neuron_execution_status_completed_with_err", true, []map[string]string{{Type: NodeAWSNeuron, RuntimeTag: "1"}}, []float64{2}, pmetric.MetricTypeSum, Count),
		"node_neuron_execution_status_completed_with_num_err": createExpectedMetric("node_neuron_execution_status_completed_with_num_err", true, []map[string]string{{Type: NodeAWSNeuron, RuntimeTag: "1"}}, []float64{3}, pmetric.MetricTypeSum, Count),
		"node_neuron_execution_status_timed_out":              createExpectedMetric("node_neuron_execution_status_timed_out", true, []map[string]string{{Type: NodeAWSNeuron, RuntimeTag: "1"}}, []float64{4}, pmetric.MetricTypeSum, Count),
		"node_neuron_execution_status_incorrect_input":        createExpectedMetric("node_neuron_execution_status_incorrect_input", true, []map[string]string{{Type: NodeAWSNeuron, RuntimeTag: "1"}}, []float64{5}, pmetric.MetricTypeSum, Count),
		"node_neuron_execution_status_failed_to_queue":        createExpectedMetric("node_neuron_execution_status_failed_to_queue", true, []map[string]string{{Type: NodeAWSNeuron, RuntimeTag: "1"}}, []float64{6}, pmetric.MetricTypeSum, Count),
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

func TestMetricModifierForNeuronDeviceEccEventMetric(t *testing.T) {
	metricModifier := setupMetricModifier()
	metricsList := pmetric.NewMetricSlice()
	createActualMetricForKey(NeuronDeviceHwEccEvents).CopyTo(metricsList.AppendEmpty())
	metricModifier.ModifyMetric(metricsList.At(0), metricsList)

	expectedMetrics := map[string]pmetric.Metric{
		NeuronDeviceHwEccEvents:                                     metricsList.At(0),
		"node_neurondevice_hw_ecc_events_mem_ecc_corrected":         createExpectedMetric("node_neurondevice_hw_ecc_events_mem_ecc_corrected", false, []map[string]string{{NeuronDevice: "device1", PodName: DummyPod, Type: NodeAWSNeuronDevice, RuntimeTag: "1"}}, []float64{1}, pmetric.MetricTypeSum, Count),
		"node_neurondevice_hw_ecc_events_mem_ecc_uncorrected":       createExpectedMetric("node_neurondevice_hw_ecc_events_mem_ecc_uncorrected", false, []map[string]string{{NeuronDevice: "device1", PodName: DummyPod, Type: NodeAWSNeuronDevice, RuntimeTag: "1"}}, []float64{2}, pmetric.MetricTypeSum, Count),
		"node_neurondevice_hw_ecc_events_sram_ecc_corrected":        createExpectedMetric("node_neurondevice_hw_ecc_events_sram_ecc_corrected", false, []map[string]string{{NeuronDevice: "device1", PodName: DummyPod, Type: NodeAWSNeuronDevice, RuntimeTag: "1"}}, []float64{3}, pmetric.MetricTypeSum, Count),
		"node_neurondevice_hw_ecc_events_sram_ecc_uncorrected":      createExpectedMetric("node_neurondevice_hw_ecc_events_sram_ecc_uncorrected", false, []map[string]string{{NeuronDevice: "device1", PodName: DummyPod, Type: NodeAWSNeuronDevice, RuntimeTag: "1"}}, []float64{4}, pmetric.MetricTypeSum, Count),
		"node_neurondevice_hw_ecc_events_total":                     createExpectedMetric("node_neurondevice_hw_ecc_events_total", false, []map[string]string{{NeuronDevice: "device1", PodName: DummyPod, Type: NodeAWSNeuronDevice, RuntimeTag: "1"}}, []float64{10}, pmetric.MetricTypeSum, Count),
		"pod_neurondevice_hw_ecc_events_mem_ecc_corrected":          createExpectedMetric("pod_neurondevice_hw_ecc_events_mem_ecc_corrected", false, []map[string]string{{NeuronDevice: "device1", PodName: DummyPod, Type: PodAWSNeuronDevice, RuntimeTag: "1"}}, []float64{1}, pmetric.MetricTypeSum, Count),
		"pod_neurondevice_hw_ecc_events_mem_ecc_uncorrected":        createExpectedMetric("pod_neurondevice_hw_ecc_events_mem_ecc_uncorrected", false, []map[string]string{{NeuronDevice: "device1", PodName: DummyPod, Type: PodAWSNeuronDevice, RuntimeTag: "1"}}, []float64{2}, pmetric.MetricTypeSum, Count),
		"pod_neurondevice_hw_ecc_events_sram_ecc_corrected":         createExpectedMetric("pod_neurondevice_hw_ecc_events_sram_ecc_corrected", false, []map[string]string{{NeuronDevice: "device1", PodName: DummyPod, Type: PodAWSNeuronDevice, RuntimeTag: "1"}}, []float64{3}, pmetric.MetricTypeSum, Count),
		"pod_neurondevice_hw_ecc_events_sram_ecc_uncorrected":       createExpectedMetric("pod_neurondevice_hw_ecc_events_sram_ecc_uncorrected", false, []map[string]string{{NeuronDevice: "device1", PodName: DummyPod, Type: PodAWSNeuronDevice, RuntimeTag: "1"}}, []float64{4}, pmetric.MetricTypeSum, Count),
		"pod_neurondevice_hw_ecc_events_total":                      createExpectedMetric("pod_neurondevice_hw_ecc_events_total", false, []map[string]string{{NeuronDevice: "device1", PodName: DummyPod, Type: PodAWSNeuronDevice, RuntimeTag: "1"}}, []float64{10}, pmetric.MetricTypeSum, Count),
		"container_neurondevice_hw_ecc_events_mem_ecc_corrected":    createExpectedMetric("container_neurondevice_hw_ecc_events_mem_ecc_corrected", false, []map[string]string{{NeuronDevice: "device1", PodName: DummyPod, Type: ContainerAWSNeuronDevice, RuntimeTag: "1"}}, []float64{1}, pmetric.MetricTypeSum, Count),
		"container_neurondevice_hw_ecc_events_mem_ecc_uncorrected":  createExpectedMetric("container_neurondevice_hw_ecc_events_mem_ecc_uncorrected", false, []map[string]string{{NeuronDevice: "device1", PodName: DummyPod, Type: ContainerAWSNeuronDevice, RuntimeTag: "1"}}, []float64{2}, pmetric.MetricTypeSum, Count),
		"container_neurondevice_hw_ecc_events_sram_ecc_corrected":   createExpectedMetric("container_neurondevice_hw_ecc_events_sram_ecc_corrected", false, []map[string]string{{NeuronDevice: "device1", PodName: DummyPod, Type: ContainerAWSNeuronDevice, RuntimeTag: "1"}}, []float64{3}, pmetric.MetricTypeSum, Count),
		"container_neurondevice_hw_ecc_events_sram_ecc_uncorrected": createExpectedMetric("container_neurondevice_hw_ecc_events_sram_ecc_uncorrected", false, []map[string]string{{NeuronDevice: "device1", PodName: DummyPod, Type: ContainerAWSNeuronDevice, RuntimeTag: "1"}}, []float64{4}, pmetric.MetricTypeSum, Count),
		"container_neurondevice_hw_ecc_events_total":                createExpectedMetric("container_neurondevice_hw_ecc_events_total", false, []map[string]string{{NeuronDevice: "device1", PodName: DummyPod, Type: ContainerAWSNeuronDevice, RuntimeTag: "1"}}, []float64{10}, pmetric.MetricTypeSum, Count),
	}

	assertModifiedMetric(t, metricsList, expectedMetrics)
}

func TestMetricModifierForNeuronDeviceEccEventMetric_PodNameMissing(t *testing.T) {
	metricModifier := setupMetricModifier()
	metricsList := pmetric.NewMetricSlice()
	removeAttributefromMetric(createActualMetricForKey(NeuronDeviceHwEccEvents), PodName).CopyTo(metricsList.AppendEmpty())
	metricModifier.ModifyMetric(metricsList.At(0), metricsList)

	expectedMetrics := map[string]pmetric.Metric{
		NeuronDeviceHwEccEvents:                                metricsList.At(0),
		"node_neurondevice_hw_ecc_events_mem_ecc_corrected":    createExpectedMetric("node_neurondevice_hw_ecc_events_mem_ecc_corrected", false, []map[string]string{{NeuronDevice: "device1", Type: NodeAWSNeuronDevice, RuntimeTag: "1"}}, []float64{1}, pmetric.MetricTypeSum, Count),
		"node_neurondevice_hw_ecc_events_mem_ecc_uncorrected":  createExpectedMetric("node_neurondevice_hw_ecc_events_mem_ecc_uncorrected", false, []map[string]string{{NeuronDevice: "device1", Type: NodeAWSNeuronDevice, RuntimeTag: "1"}}, []float64{2}, pmetric.MetricTypeSum, Count),
		"node_neurondevice_hw_ecc_events_sram_ecc_corrected":   createExpectedMetric("node_neurondevice_hw_ecc_events_sram_ecc_corrected", false, []map[string]string{{NeuronDevice: "device1", Type: NodeAWSNeuronDevice, RuntimeTag: "1"}}, []float64{3}, pmetric.MetricTypeSum, Count),
		"node_neurondevice_hw_ecc_events_sram_ecc_uncorrected": createExpectedMetric("node_neurondevice_hw_ecc_events_sram_ecc_uncorrected", false, []map[string]string{{NeuronDevice: "device1", Type: NodeAWSNeuronDevice, RuntimeTag: "1"}}, []float64{4}, pmetric.MetricTypeSum, Count),
		"node_neurondevice_hw_ecc_events_total":                createExpectedMetric("node_neurondevice_hw_ecc_events_total", false, []map[string]string{{NeuronDevice: "device1", Type: NodeAWSNeuronDevice, RuntimeTag: "1"}}, []float64{10}, pmetric.MetricTypeSum, Count),
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
	createActualMetricForKey(NeuronExecutionErrors).CopyTo(metricsList.AppendEmpty())
	createActualMetricForKey(NeuronExecutionStatus).CopyTo(metricsList.AppendEmpty())
	createActualMetricForKey(NeuronCoreMemoryUsageModelSharedScratchpad).CopyTo(metricsList.AppendEmpty())
	createActualMetricForKey(NeuronDeviceRuntimeMemoryUsedBytes).CopyTo(metricsList.AppendEmpty())
	createActualMetricForKey(NeuronDeviceHwEccEvents).CopyTo(metricsList.AppendEmpty())
	createActualMetricForKey(NonNeuronMetric).CopyTo(metricsList.AppendEmpty())

	for i := 0; i < metricsList.Len(); i++ {
		metricModifier.ModifyMetric(metricsList.At(i), metricsList)
	}

	expectedMetrics := map[string]pmetric.Metric{
		NeuronExecutionLatency:                     metricsList.At(0),
		NeuronExecutionErrors:                      metricsList.At(1),
		NeuronExecutionStatus:                      metricsList.At(2),
		NeuronCoreMemoryUsageModelSharedScratchpad: metricsList.At(3),
		NeuronDeviceRuntimeMemoryUsedBytes:         metricsList.At(4),
		NeuronDeviceHwEccEvents:                    metricsList.At(5),
		NonNeuronMetric:                            metricsList.At(6),

		"node_neuron_execution_latency": createExpectedMetric("node_neuron_execution_latency", false, []map[string]string{{Type: NodeAWSNeuron}}, []float64{1}, pmetric.MetricTypeSum, Seconds),

		"node_neuron_execution_errors_generic":   createExpectedMetric("node_neuron_execution_errors_generic", true, []map[string]string{{Type: NodeAWSNeuron, RuntimeTag: "1"}}, []float64{1}, pmetric.MetricTypeSum, Count),
		"node_neuron_execution_errors_numerical": createExpectedMetric("node_neuron_execution_errors_numerical", true, []map[string]string{{Type: NodeAWSNeuron, RuntimeTag: "1"}}, []float64{2}, pmetric.MetricTypeSum, Count),
		"node_neuron_execution_errors_transient": createExpectedMetric("node_neuron_execution_errors_transient", true, []map[string]string{{Type: NodeAWSNeuron, RuntimeTag: "1"}}, []float64{3}, pmetric.MetricTypeSum, Count),
		"node_neuron_execution_errors_model":     createExpectedMetric("node_neuron_execution_errors_model", true, []map[string]string{{Type: NodeAWSNeuron, RuntimeTag: "1"}}, []float64{4}, pmetric.MetricTypeSum, Count),
		"node_neuron_execution_errors_runtime":   createExpectedMetric("node_neuron_execution_errors_runtime", true, []map[string]string{{Type: NodeAWSNeuron, RuntimeTag: "1"}}, []float64{5}, pmetric.MetricTypeSum, Count),
		"node_neuron_execution_errors_hardware":  createExpectedMetric("node_neuron_execution_errors_hardware", true, []map[string]string{{Type: NodeAWSNeuron, RuntimeTag: "1"}}, []float64{6}, pmetric.MetricTypeSum, Count),
		"node_neuron_execution_errors_total":     createExpectedMetric("node_neuron_execution_errors_total", true, []map[string]string{{Type: NodeAWSNeuron, RuntimeTag: "1"}}, []float64{21}, pmetric.MetricTypeSum, Count),

		"node_neuron_execution_status_completed":              createExpectedMetric("node_neuron_execution_status_completed", true, []map[string]string{{Type: NodeAWSNeuron, RuntimeTag: "1"}}, []float64{1}, pmetric.MetricTypeSum, Count),
		"node_neuron_execution_status_completed_with_err":     createExpectedMetric("node_neuron_execution_status_completed_with_err", true, []map[string]string{{Type: NodeAWSNeuron, RuntimeTag: "1"}}, []float64{2}, pmetric.MetricTypeSum, Count),
		"node_neuron_execution_status_completed_with_num_err": createExpectedMetric("node_neuron_execution_status_completed_with_num_err", true, []map[string]string{{Type: NodeAWSNeuron, RuntimeTag: "1"}}, []float64{3}, pmetric.MetricTypeSum, Count),
		"node_neuron_execution_status_timed_out":              createExpectedMetric("node_neuron_execution_status_timed_out", true, []map[string]string{{Type: NodeAWSNeuron, RuntimeTag: "1"}}, []float64{4}, pmetric.MetricTypeSum, Count),
		"node_neuron_execution_status_incorrect_input":        createExpectedMetric("node_neuron_execution_status_incorrect_input", true, []map[string]string{{Type: NodeAWSNeuron, RuntimeTag: "1"}}, []float64{5}, pmetric.MetricTypeSum, Count),
		"node_neuron_execution_status_failed_to_queue":        createExpectedMetric("node_neuron_execution_status_failed_to_queue", true, []map[string]string{{Type: NodeAWSNeuron, RuntimeTag: "1"}}, []float64{6}, pmetric.MetricTypeSum, Count),

		"node_neuroncore_memory_usage_model_shared_scratchpad":      createExpectedMetric("node_neuroncore_memory_usage_model_shared_scratchpad", false, []map[string]string{{NeuronCore: "core0", NeuronDevice: "device0", Type: NodeAWSNeuronCore, PodName: DummyPod}, {NeuronCore: "core1", NeuronDevice: "device0", Type: NodeAWSNeuronCore, PodName: DummyPod}, {NeuronCore: "core2", NeuronDevice: "device1", Type: NodeAWSNeuronCore, PodName: DummyPod}}, []float64{1, 2, 3}, pmetric.MetricTypeSum, Bytes),
		"pod_neuroncore_memory_usage_model_shared_scratchpad":       createExpectedMetric("pod_neuroncore_memory_usage_model_shared_scratchpad", false, []map[string]string{{NeuronCore: "core0", NeuronDevice: "device0", Type: PodAWSNeuronCore, PodName: DummyPod}, {NeuronCore: "core1", NeuronDevice: "device0", Type: PodAWSNeuronCore, PodName: DummyPod}, {NeuronCore: "core2", NeuronDevice: "device1", Type: PodAWSNeuronCore, PodName: DummyPod}}, []float64{1, 2, 3}, pmetric.MetricTypeSum, Bytes),
		"container_neuroncore_memory_usage_model_shared_scratchpad": createExpectedMetric("container_neuroncore_memory_usage_model_shared_scratchpad", false, []map[string]string{{NeuronCore: "core0", NeuronDevice: "device0", Type: ContainerAWSNeuronCore, PodName: DummyPod}, {NeuronCore: "core1", NeuronDevice: "device0", Type: ContainerAWSNeuronCore, PodName: DummyPod}, {NeuronCore: "core2", NeuronDevice: "device1", Type: ContainerAWSNeuronCore, PodName: DummyPod}}, []float64{1, 2, 3}, pmetric.MetricTypeSum, Bytes),

		"node_neurondevice_runtime_memory_used_bytes": createExpectedMetric("node_neurondevice_runtime_memory_used_bytes", false, []map[string]string{{Type: NodeAWSNeuron}}, []float64{2}, pmetric.MetricTypeSum, Bytes),

		"node_neurondevice_hw_ecc_events_mem_ecc_corrected":         createExpectedMetric("node_neurondevice_hw_ecc_events_mem_ecc_corrected", false, []map[string]string{{NeuronDevice: "device1", PodName: DummyPod, Type: NodeAWSNeuronDevice, RuntimeTag: "1"}}, []float64{1}, pmetric.MetricTypeSum, Count),
		"node_neurondevice_hw_ecc_events_mem_ecc_uncorrected":       createExpectedMetric("node_neurondevice_hw_ecc_events_mem_ecc_uncorrected", false, []map[string]string{{NeuronDevice: "device1", PodName: DummyPod, Type: NodeAWSNeuronDevice, RuntimeTag: "1"}}, []float64{2}, pmetric.MetricTypeSum, Count),
		"node_neurondevice_hw_ecc_events_sram_ecc_corrected":        createExpectedMetric("node_neurondevice_hw_ecc_events_sram_ecc_corrected", false, []map[string]string{{NeuronDevice: "device1", PodName: DummyPod, Type: NodeAWSNeuronDevice, RuntimeTag: "1"}}, []float64{3}, pmetric.MetricTypeSum, Count),
		"node_neurondevice_hw_ecc_events_sram_ecc_uncorrected":      createExpectedMetric("node_neurondevice_hw_ecc_events_sram_ecc_uncorrected", false, []map[string]string{{NeuronDevice: "device1", PodName: DummyPod, Type: NodeAWSNeuronDevice, RuntimeTag: "1"}}, []float64{4}, pmetric.MetricTypeSum, Count),
		"node_neurondevice_hw_ecc_events_total":                     createExpectedMetric("node_neurondevice_hw_ecc_events_total", false, []map[string]string{{NeuronDevice: "device1", PodName: DummyPod, Type: NodeAWSNeuronDevice, RuntimeTag: "1"}}, []float64{10}, pmetric.MetricTypeSum, Count),
		"pod_neurondevice_hw_ecc_events_mem_ecc_corrected":          createExpectedMetric("pod_neurondevice_hw_ecc_events_mem_ecc_corrected", false, []map[string]string{{NeuronDevice: "device1", PodName: DummyPod, Type: PodAWSNeuronDevice, RuntimeTag: "1"}}, []float64{1}, pmetric.MetricTypeSum, Count),
		"pod_neurondevice_hw_ecc_events_mem_ecc_uncorrected":        createExpectedMetric("pod_neurondevice_hw_ecc_events_mem_ecc_uncorrected", false, []map[string]string{{NeuronDevice: "device1", PodName: DummyPod, Type: PodAWSNeuronDevice, RuntimeTag: "1"}}, []float64{2}, pmetric.MetricTypeSum, Count),
		"pod_neurondevice_hw_ecc_events_sram_ecc_corrected":         createExpectedMetric("pod_neurondevice_hw_ecc_events_sram_ecc_corrected", false, []map[string]string{{NeuronDevice: "device1", PodName: DummyPod, Type: PodAWSNeuronDevice, RuntimeTag: "1"}}, []float64{3}, pmetric.MetricTypeSum, Count),
		"pod_neurondevice_hw_ecc_events_sram_ecc_uncorrected":       createExpectedMetric("pod_neurondevice_hw_ecc_events_sram_ecc_uncorrected", false, []map[string]string{{NeuronDevice: "device1", PodName: DummyPod, Type: PodAWSNeuronDevice, RuntimeTag: "1"}}, []float64{4}, pmetric.MetricTypeSum, Count),
		"pod_neurondevice_hw_ecc_events_total":                      createExpectedMetric("pod_neurondevice_hw_ecc_events_total", false, []map[string]string{{NeuronDevice: "device1", PodName: DummyPod, Type: PodAWSNeuronDevice, RuntimeTag: "1"}}, []float64{10}, pmetric.MetricTypeSum, Count),
		"container_neurondevice_hw_ecc_events_mem_ecc_corrected":    createExpectedMetric("container_neurondevice_hw_ecc_events_mem_ecc_corrected", false, []map[string]string{{NeuronDevice: "device1", PodName: DummyPod, Type: ContainerAWSNeuronDevice, RuntimeTag: "1"}}, []float64{1}, pmetric.MetricTypeSum, Count),
		"container_neurondevice_hw_ecc_events_mem_ecc_uncorrected":  createExpectedMetric("container_neurondevice_hw_ecc_events_mem_ecc_uncorrected", false, []map[string]string{{NeuronDevice: "device1", PodName: DummyPod, Type: ContainerAWSNeuronDevice, RuntimeTag: "1"}}, []float64{2}, pmetric.MetricTypeSum, Count),
		"container_neurondevice_hw_ecc_events_sram_ecc_corrected":   createExpectedMetric("container_neurondevice_hw_ecc_events_sram_ecc_corrected", false, []map[string]string{{NeuronDevice: "device1", PodName: DummyPod, Type: ContainerAWSNeuronDevice, RuntimeTag: "1"}}, []float64{3}, pmetric.MetricTypeSum, Count),
		"container_neurondevice_hw_ecc_events_sram_ecc_uncorrected": createExpectedMetric("container_neurondevice_hw_ecc_events_sram_ecc_uncorrected", false, []map[string]string{{NeuronDevice: "device1", PodName: DummyPod, Type: ContainerAWSNeuronDevice, RuntimeTag: "1"}}, []float64{4}, pmetric.MetricTypeSum, Count),
		"container_neurondevice_hw_ecc_events_total":                createExpectedMetric("container_neurondevice_hw_ecc_events_total", false, []map[string]string{{NeuronDevice: "device1", PodName: DummyPod, Type: ContainerAWSNeuronDevice, RuntimeTag: "1"}}, []float64{10}, pmetric.MetricTypeSum, Count),
	}
	assertModifiedMetric(t, metricsList, expectedMetrics)
}

func TestMetricWithStaleDatapoint(t *testing.T) {
	metricModifier := setupMetricModifier()
	metricsList := pmetric.NewMetricSlice()
	createActualMetricForKey(NeuronExecutionLatency).CopyTo(metricsList.AppendEmpty())

	originalMetricDatapoint := metricsList.At(0).Gauge().DataPoints().At(0)
	originalMetricDatapoint.SetFlags(originalMetricDatapoint.Flags().WithNoRecordedValue(true))

	metricModifier.ModifyMetric(metricsList.At(0), metricsList)

	expectedMetrics := map[string]pmetric.Metric{
		NeuronExecutionLatency:          metricsList.At(0),
		"node_neuron_execution_latency": createExpectedMetric("node_neuron_execution_latency", false, []map[string]string{{Type: NodeAWSNeuron}}, []float64{1}, pmetric.MetricTypeSum, Seconds),
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
			assert.False(t, actualDatapoint.Flags().NoRecordedValue())
			assert.NotEqual(t, pmetric.NumberDataPointValueTypeEmpty, actualDatapoint.ValueType())
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
