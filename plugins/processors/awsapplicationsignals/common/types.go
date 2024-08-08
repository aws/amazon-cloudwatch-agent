// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package common

// Metric attribute used as CloudWatch metric dimension.
const (
	CWMetricAttributeLocalService             = "Service"
	CWMetricAttributeLocalOperation           = "Operation"
	CWMetricAttributeEnvironment              = "Environment"
	CWMetricAttributeRemoteService            = "RemoteService"
	CWMetricAttributeRemoteEnvironment        = "RemoteEnvironment"
	CWMetricAttributeRemoteOperation          = "RemoteOperation"
	CWMetricAttributeRemoteResourceIdentifier = "RemoteResourceIdentifier"
	CWMetricAttributeRemoteResourceType       = "RemoteResourceType"
)

// Platform attribute used as CloudWatch EMF log field and X-Ray trace annotation.
const (
	AttributePlatformType        = "PlatformType"
	AttributeEKSClusterName      = "EKS.Cluster"
	AttributeK8SClusterName      = "K8s.Cluster"
	AttributeK8SNamespace        = "K8s.Namespace"
	AttributeK8SWorkload         = "K8s.Workload"
	AttributeK8SPod              = "K8s.Pod"
	AttributeEC2AutoScalingGroup = "EC2.AutoScalingGroup"
	AttributeEC2InstanceId       = "EC2.InstanceId"
	AttributeHost                = "Host"
)

// Telemetry attributes used as CloudWatch EMF log fields.
const (
	MetricAttributeTelemetrySDK    = "Telemetry.SDK"
	MetricAttributeTelemetryAgent  = "Telemetry.Agent"
	MetricAttributeTelemetrySource = "Telemetry.Source"
)

// Resource attributes used as CloudWatch EMF log fields.
const (
	MetricAttributeRemoteDbUser = "RemoteDbUser"
)

const (
	AttributeTmpReserved = "aws.tmp.reserved"
)

var CWMetricAttributes = []string{
	CWMetricAttributeLocalService,
	CWMetricAttributeLocalOperation,
	CWMetricAttributeEnvironment,
	CWMetricAttributeRemoteService,
	CWMetricAttributeRemoteEnvironment,
	CWMetricAttributeRemoteOperation,
	CWMetricAttributeRemoteResourceIdentifier,
	CWMetricAttributeRemoteResourceType,
}
