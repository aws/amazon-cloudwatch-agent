// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package containerinsightscommon

const (
	GoPSUtilProcDirEnv = "HOST_PROC"

	MinTimeDiff = 50 * 1000 // We assume 50 micro-seconds is the minimal gap between two collected data sample to be valid to calculate delta

	ClusterNameKey          = "ClusterName"
	NodeNameKey             = "NodeName" // Attribute names
	InstanceIdKey           = "InstanceId"
	InstanceTypeKey         = "InstanceType"
	AutoScalingGroupNameKey = "AutoScalingGroupName"
	VersionKey              = "Version"
	MetricType              = "Type"
	SourcesKey              = "Sources"
	GpuDeviceKey            = "GpuDevice"

	// metric collected
	CpuTotal                   = "cpu_usage_total"
	CpuUser                    = "cpu_usage_user"
	CpuSystem                  = "cpu_usage_system"
	CpuLimit                   = "cpu_limit"
	CpuUtilization             = "cpu_utilization"
	CpuRequest                 = "cpu_request"
	CpuReservedCapacity        = "cpu_reserved_capacity"
	CpuUtilizationOverPodLimit = "cpu_utilization_over_pod_limit"

	MemUsage                   = "memory_usage"
	MemCache                   = "memory_cache"
	MemRss                     = "memory_rss"
	MemMaxusage                = "memory_max_usage"
	MemSwap                    = "memory_swap"
	MemFailcnt                 = "memory_failcnt"
	MemMappedfile              = "memory_mapped_file"
	MemWorkingset              = "memory_working_set"
	MemPgfault                 = "memory_pgfault"
	MemPgmajfault              = "memory_pgmajfault"
	MemHierarchicalPgfault     = "memory_hierarchical_pgfault"
	MemHierarchicalPgmajfault  = "memory_hierarchical_pgmajfault"
	MemLimit                   = "memory_limit"
	MemRequest                 = "memory_request"
	MemUtilization             = "memory_utilization"
	MemReservedCapacity        = "memory_reserved_capacity"
	MemUtilizationOverPodLimit = "memory_utilization_over_pod_limit"

	NetIfce       = "interface"
	NetRxBytes    = "network_rx_bytes"
	NetRxPackets  = "network_rx_packets"
	NetRxDropped  = "network_rx_dropped"
	NetRxErrors   = "network_rx_errors"
	NetTxBytes    = "network_tx_bytes"
	NetTxPackets  = "network_tx_packets"
	NetTxDropped  = "network_tx_dropped"
	NetTxErrors   = "network_tx_errors"
	NetTotalBytes = "network_total_bytes"

	DiskDev     = "device"
	EbsVolumeId = "ebs_volume_id"

	FSType        = "fstype"
	FSUsage       = "filesystem_usage"
	FSCapacity    = "filesystem_capacity"
	FSAvailable   = "filesystem_available"
	FSInodes      = "filesystem_inodes"
	FSInodesfree  = "filesystem_inodes_free"
	FSUtilization = "filesystem_utilization"

	DiskIOServiceBytesPrefix = "diskio_io_service_bytes_"
	DiskIOServicedPrefix     = "diskio_io_serviced_"
	DiskIOAsync              = "Async"
	DiskIORead               = "Read"
	DiskIOSync               = "Sync"
	DiskIOWrite              = "Write"
	DiskIOTotal              = "Total"

	GpuUtilization    = "gpu_utilization"
	GpuMemUtilization = "gpu_memory_utilization"
	GpuMemUsed        = "gpu_memory_used"
	GpuMemTotal       = "gpu_memory_total"
	GpuTemperature    = "gpu_temperature"
	GpuPowerDraw      = "gpu_power_draw"
	GpuRequest        = "gpu_request"
	GpuLimit          = "gpu_limit"
	GpuTotal          = "gpu_total"
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

	TypeCluster          = "Cluster"
	TypeClusterService   = "ClusterService"
	TypeClusterNamespace = "ClusterNamespace"
	TypeService          = "Service"

	// Both TypeInstance and TypeNode mean EC2 Instance, they are used in ECS and EKS separately
	TypeInstance       = "Instance"
	TypeNode           = "Node"
	TypeInstanceFS     = "InstanceFS"
	TypeNodeFS         = "NodeFS"
	TypeInstanceNet    = "InstanceNet"
	TypeNodeNet        = "NodeNet"
	TypeInstanceDiskIO = "InstanceDiskIO"
	TypeNodeDiskIO     = "NodeDiskIO"
	TypeGpuContainer   = "ContainerGPU"
	TypeGpuPod         = "PodGPU"
	TypeGpuNode        = "NodeGPU"
	TypeGpuCluster     = "ClusterGPU"

	TypePod             = "Pod"
	TypePodNet          = "PodNet"
	TypeContainer       = "Container"
	TypeContainerFS     = "ContainerFS"
	TypeContainerDiskIO = "ContainerDiskIO"
	// Special type for pause container, introduced in https://github.com/aws/amazon-cloudwatch-agent/issues/188
	// because containerd does not set container name pause container name to POD like docker does.
	TypeInfraContainer = "InfraContainer"
)
