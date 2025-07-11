// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package constants

const (
	ClusterNameKey          = "ClusterName"
	NodeNameKey             = "NodeName" // Attribute names
	InstanceIdKey           = "InstanceId"
	InstanceTypeKey         = "InstanceType"
	AutoScalingGroupNameKey = "AutoScalingGroupName"
	VersionKey              = "Version"
	MetricType              = "Type"
	SourcesKey              = "Sources"
	GpuDeviceKey            = "GpuDevice"

	ClusterQueueNameKey     = "ClusterQueue"
	ClusterQueueStatusKey   = "Status"
	ClusterQueueReasonKey   = "Reason"
	ClusterQueueResourceKey = "Resource"
	Flavor                  = "Flavor"

	GpuUtilization    = "gpu_utilization"
	GpuMemUtilization = "gpu_memory_utilization"
	GpuMemUsed        = "gpu_memory_used"
	GpuMemTotal       = "gpu_memory_total"
	GpuTemperature    = "gpu_temperature"
	GpuPowerDraw      = "gpu_power_draw"
	GpuUniqueId       = "UUID"

	NeuronCoreUtilization                       = "neuroncore_utilization"
	NeuronCoreMemoryUtilizationTotal            = "neuroncore_memory_usage_total"
	NeuronCoreMemoryUtilizationConstants        = "neuroncore_memory_usage_constants"
	NeuronCoreMemoryUtilizationModelCode        = "neuroncore_memory_usage_model_code"
	NeuronCoreMemoryUtilizationSharedScratchpad = "neuroncore_memory_usage_model_shared_scratchpad"
	NeuronCoreMemoryUtilizationRuntimeMemory    = "neuroncore_memory_usage_runtime_memory"
	NeuronCoreMemoryUtilizationTensors          = "neuroncore_memory_usage_tensors"
	NeuronDeviceHardwareEccEvents               = "neurondevice_hw_ecc_events"
	NeuronExecutionStatus                       = "neuron_execution_status"
	NeuronExecutionErrors                       = "neuron_execution_errors"
	NeuronRuntimeMemoryUsage                    = "neurondevice_runtime_memory_used_bytes"
	NeuronInstanceInfo                          = "instance_info"
	NeuronHardware                              = "neuron_hardware"
	NeuronExecutionLatency                      = "neuron_execution_latency"

	// Converted metrics for NVME metrics
	NvmeReadOpsTotal        = "diskio_ebs_total_read_ops"
	NvmeWriteOpsTotal       = "diskio_ebs_total_write_ops"
	NvmeReadBytesTotal      = "diskio_ebs_total_read_bytes"
	NvmeWriteBytesTotal     = "diskio_ebs_total_write_bytes"
	NvmeReadTime            = "diskio_ebs_total_read_time"
	NvmeWriteTime           = "diskio_ebs_total_write_time"
	NvmeExceededIOPSTime    = "diskio_ebs_volume_performance_exceeded_iops"
	NvmeExceededTPTime      = "diskio_ebs_volume_performance_exceeded_tp"
	NvmeExceededEC2IOPSTime = "diskio_ebs_ec2_instance_performance_exceeded_iops"
	NvmeExceededEC2TPTime   = "diskio_ebs_ec2_instance_performance_exceeded_tp"
	NvmeVolumeQueueLength   = "diskio_ebs_volume_queue_length"

	TypeCluster = "Cluster"
	TypeService = "Service"

	// Both TypeInstance and TypeNode mean EC2 Instance, they are used in ECS and EKS separately
	TypeInstance     = "Instance"
	TypeNode         = "Node"
	TypeGpuContainer = "ContainerGPU"
	TypeGpuPod       = "PodGPU"
	TypeGpuNode      = "NodeGPU"
	TypeGpuCluster   = "ClusterGPU"
	TypeNodeEBS      = "NodeEBS"
	TypePod          = "Pod"
	TypeContainer    = "Container"

	Kubernetes       = "kubernetes"
	K8sNamespace     = "Namespace"
	PodIdKey         = "PodId"
	FullPodNameKey   = "FullPodName"
	PodNameKey       = "PodName"
	K8sPodNameKey    = "K8sPodName"
	ContainerNamekey = "ContainerName"
	ContainerIdkey   = "ContainerId"
	PodOwnersKey     = "PodOwners"
	HostKey          = "host"
	K8sKey           = "kubernetes"
	K8sLabelsKey     = "labels"

	Timestamp = "Timestamp"
)
