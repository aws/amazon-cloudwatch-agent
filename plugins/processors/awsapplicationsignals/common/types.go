// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package common

const (
	MetricAttributeLocalService             = "Service"
	MetricAttributeLocalOperation           = "Operation"
	MetricAttributeEnvironment              = "Environment"
	MetricAttributeRemoteService            = "RemoteService"
	MetricAttributeRemoteEnvironment        = "RemoteEnvironment"
	MetricAttributeRemoteOperation          = "RemoteOperation"
	MetricAttributeRemoteResourceIdentifier = "RemoteResourceIdentifier"
	MetricAttributeRemoteResourceType       = "RemoteResourceType"
)
const (
	AttributeEKSClusterName          = "EKS.Cluster"
	AttributeK8SClusterName          = "K8s.Cluster"
	AttributeK8SNamespace            = "K8s.Namespace"
	AttributeEC2AutoScalingGroupName = "EC2.AutoScalingGroupName"
	AttributeEC2InstanceId           = "EC2.InstanceId"
	AttributePlatformType            = "PlatformType"
	AttributeSDK                     = "Telemetry.SDK"
)

const (
	AttributeTmpReserved = "aws.tmp.reserved"
)
