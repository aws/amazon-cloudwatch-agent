// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resourcemap

var resourceMap *ResourceMap

type ec2Info struct {
	InstanceID       string
	AutoScalingGroup string
}

type ecsInfo struct {
	ClusterName string
}

type eksInfo struct {
	ClusterName string
}

type ResourceMap struct {
	// mode should be EC2, ECS, EKS, and K8S
	mode string

	// ec2Info stores information about EC2 instances such as instance ID and
	// auto scaling groups
	ec2Info ec2Info

	// ecsInfo stores information about ECS such as cluster name
	// TODO: This struct may need to be expanded to include task role arn and more
	ecsInfo ecsInfo

	// ekeInfo stores information about EKS such as cluster
	// TODO: This struct may need to be expanded to include namespace, pod, node, etc
	eksInfo eksInfo

	// This variable is reserved for communication between OTEL components and LogAgent
	// in order to achieve process correlations
	logFiles map[string]string
}

func GetResourceMap() *ResourceMap {
	if resourceMap == nil {
		InitResourceMap()
	}
	return resourceMap
}

func InitResourceMap() {
	// Add logic to store attributes such as instance ID, cluster name, etc here
}

func (r *ResourceMap) LogFiles() map[string]string {
	return r.logFiles
}

func (r *ResourceMap) EC2Info() ec2Info {
	return r.ec2Info
}

func (r *ResourceMap) ECSInfo() ecsInfo {
	return r.ecsInfo
}

func (r *ResourceMap) EKSInfo() eksInfo {
	return r.eksInfo
}
