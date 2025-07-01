// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package containerinsightscommon

const (
	GoPSUtilProcDirEnv = "HOST_PROC"

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

	// metric collected
	CpuTotal                   = "cpu_usage_total"
	CpuLimit                   = "cpu_limit"
	CpuUtilization             = "cpu_utilization"
	CpuRequest                 = "cpu_request"
	CpuReservedCapacity        = "cpu_reserved_capacity"
	CpuUtilizationOverPodLimit = "cpu_utilization_over_pod_limit"

	MemWorkingset              = "memory_working_set"
	MemLimit                   = "memory_limit"
	MemRequest                 = "memory_request"
	MemUtilization             = "memory_utilization"
	MemReservedCapacity        = "memory_reserved_capacity"
	MemUtilizationOverPodLimit = "memory_utilization_over_pod_limit"

	NetIfce       = "interface"
	NetRxBytes    = "network_rx_bytes"
	NetTxBytes    = "network_tx_bytes"
	NetTotalBytes = "network_total_bytes"

	DiskDev     = "device"
	EbsVolumeId = "ebs_volume_id"

	FSUtilization = "filesystem_utilization"

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

	TypeCluster          = "Cluster"
	TypeClusterService   = "ClusterService"
	TypeClusterNamespace = "ClusterNamespace"
	TypeService          = "Service"
	TypeClusterQueue     = "ClusterQueue"

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
	TypeNodeEBS        = "NodeEBS"

	TypePod             = "Pod"
	TypePodNet          = "PodNet"
	TypeContainer       = "Container"
	TypeContainerFS     = "ContainerFS"
	TypeContainerDiskIO = "ContainerDiskIO"
)

// ECS
const (
	ContainerInstanceIdKey = "ContainerInstanceId"
	RunningTaskCount       = "number_of_running_tasks"
)

// EKS
const (
	KubeSecurePort = "10250"
	BearerToken    = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	CAFile         = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"

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

	RunningPodCount       = "number_of_running_pods"
	RunningContainerCount = "number_of_running_containers"
	ContainerCount        = "number_of_containers"
	NodeCount             = "node_count"
	FailedNodeCount       = "failed_node_count"
	ContainerRestartCount = "number_of_container_restarts"

	PodStatus       = "pod_status"
	ContainerStatus = "container_status"

	ContainerStatusReason          = "container_status_reason"
	ContainerLastTerminationReason = "container_last_termination_reason"

	Timestamp = "Timestamp"

	//Pod Owners
	ReplicaSet            = "ReplicaSet"
	ReplicationController = "ReplicationController"
	StatefulSet           = "StatefulSet"
	DaemonSet             = "DaemonSet"
	Deployment            = "Deployment"
	Job                   = "Job"
	CronJob               = "CronJob"
)
