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
	AttributeEKSClusterName      = "EKS.Cluster"
	AttributeK8SClusterName      = "K8s.Cluster"
	AttributeK8SNamespace        = "K8s.Namespace"
	AttributeEC2AutoScalingGroup = "EC2.AutoScalingGroup"
	AttributeEC2InstanceId       = "EC2.InstanceId"
	AttributeHost                = "Host"
	AttributePlatformType        = "PlatformType"
	AttributeTelemetrySDK        = "Telemetry.SDK"
	AttributeTelemetryAgent      = "Telemetry.Agent"
	AttributeTelemetrySource     = "Telemetry.Source"
)

const (
	AttributeTmpReserved = "aws.tmp.reserved"
)

var IndexableMetricAttributes = []string{
	MetricAttributeLocalService,
	MetricAttributeLocalOperation,
	MetricAttributeEnvironment,
	MetricAttributeRemoteService,
	MetricAttributeRemoteEnvironment,
	MetricAttributeRemoteOperation,
	MetricAttributeRemoteResourceIdentifier,
	MetricAttributeRemoteResourceType,
}
