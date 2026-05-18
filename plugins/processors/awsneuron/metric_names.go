// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsneuron

// Metric name constants for neuroncore and execution metrics.
// These match the real Neuron Monitor Prometheus endpoint output.
const (
	NeuronCoreUtilization                       = "neuroncore_utilization_ratio"
	NeuronCoreMemoryUtilizationConstants        = "neuroncore_memory_usage_constants"
	NeuronCoreMemoryUtilizationModelCode        = "neuroncore_memory_usage_model_code"
	NeuronCoreMemoryUtilizationSharedScratchpad = "neuroncore_memory_usage_model_shared_scratchpad"
	NeuronCoreMemoryUtilizationRuntimeMemory    = "neuroncore_memory_usage_runtime_memory"
	NeuronCoreMemoryUtilizationTensors          = "neuroncore_memory_usage_tensors"
	NeuronExecutionStatus                       = "execution_status_total"
	NeuronExecutionErrors                       = "execution_errors_total"
	NeuronRuntimeMemoryUsage                    = "neuron_runtime_memory_used_bytes"
	NeuronExecutionLatency                      = "execution_latency_seconds"
)

// Attribute name constants used by the neuron metrics.
// The real Prometheus endpoint uses lowercase "neuroncore" (no uppercase NeuronCore/NeuronDevice).
const (
	NeuronHardwareInfoKey         = "neuron_hardware_info"
	NeuronHardwareKey             = "neuron_hardware"
	NeuronCoreAttributeKey        = "neuroncore"
	NeuronDeviceAttributeKey      = "neurondevice"
	NeuronDeviceCountAttributeKey = "neuron_device_count"
	NeuronCorePerDeviceKey        = "neuroncore_per_device_count"
	RuntimeTag                    = "runtime_tag"
	MemoryLocationNone            = "None"
)

// Type-specific attribute keys for non-core metrics.
const (
	StatusType     = "status_type"
	ErrorType      = "error_type"
	MemoryLocation = "memory_location"
	Percentile     = "percentile"
)

// attributeConfig maps metric names to the attribute keys that distinguish their datapoints.
// Per-core metrics use the lowercase neuroncore attribute only.
// Per-node metrics use a type-specific attribute (e.g., status_type, error_type).
var attributeConfig = map[string][]string{
	NeuronExecutionStatus:                       {StatusType},
	NeuronExecutionErrors:                       {ErrorType},
	NeuronRuntimeMemoryUsage:                    {MemoryLocation},
	NeuronExecutionLatency:                      {Percentile},
	NeuronCoreUtilization:                       {NeuronCoreAttributeKey},
	NeuronCoreMemoryUtilizationConstants:        {NeuronCoreAttributeKey},
	NeuronCoreMemoryUtilizationModelCode:        {NeuronCoreAttributeKey},
	NeuronCoreMemoryUtilizationSharedScratchpad: {NeuronCoreAttributeKey},
	NeuronCoreMemoryUtilizationRuntimeMemory:    {NeuronCoreAttributeKey},
	NeuronCoreMemoryUtilizationTensors:          {NeuronCoreAttributeKey},
}

// nonCoreVariants maps type-specific attribute keys to all their possible values.
// When synthesizing zero-valued datapoints for missing non-core metrics, one datapoint
// is created per variant value (e.g., 5 error types, 6 status types).
var nonCoreVariants = map[string][]string{
	StatusType:     {"completed", "completed_with_err", "completed_with_num_err", "timed_out", "incorrect_input", "failed_to_queue"},
	ErrorType:      {"numerical", "transient", "model", "runtime", "hardware"},
	MemoryLocation: {"host", "neuron_device"},
	Percentile:     {"p0", "p1", "p100", "p25", "p50", "p75", "p99"},
}

// instanceLabelKeys are the instance-level labels present on every neuron-specific metric
// from the Prometheus endpoint. These are copied from neuron_hardware_info to all
// synthesized datapoints.
var instanceLabelKeys = []string{
	"availability_zone",
	"instance_id",
	"instance_name",
	"instance_type",
	"region",
	"subnet_id",
}

// counterMetrics identifies metrics that should be synthesized as Sum (monotonic=true)
// rather than Gauge. These correspond to Prometheus Counter-type metrics.
var counterMetrics = map[string]bool{
	NeuronExecutionErrors: true,
	NeuronExecutionStatus: true,
}

// coreMemoryMetrics identifies per-core metrics that should have memory_location="None"
// set on their synthesized datapoints. neuroncore_utilization_ratio is NOT in this set
// because it does not carry a memory_location attribute.
var coreMemoryMetrics = map[string]bool{
	NeuronCoreMemoryUtilizationConstants:        true,
	NeuronCoreMemoryUtilizationModelCode:        true,
	NeuronCoreMemoryUtilizationSharedScratchpad: true,
	NeuronCoreMemoryUtilizationRuntimeMemory:    true,
	NeuronCoreMemoryUtilizationTensors:          true,
}
