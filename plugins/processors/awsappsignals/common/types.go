// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package common

const (
	AttributeRemoteService       = "aws.remote.service"
	AttributeHostedInEnvironment = "aws.hostedin.environment"
)

const (
	MetricAttributeRemoteNamespace = "K8s.RemoteNamespace"
	MetricAttributeLocalService    = "Service"
	MetricAttributeLocalOperation  = "Operation"
	MetricAttributeRemoteService   = "RemoteService"
	MetricAttributeRemoteOperation = "RemoteOperation"
	MetricAttributeRemoteTarget    = "RemoteTarget"
)
const (
	HostedInAttributeClusterName    = "HostedIn.EKS.Cluster"
	HostedInAttributeK8SNamespace   = "HostedIn.K8s.Namespace"
	HostedInAttributeEnvironment    = "HostedIn.Environment"
	HostedInAttributeK8SClusterName = "HostedIn.K8s.Cluster"
	HostedInAttributeEC2Environment = "HostedIn.EC2.Environment"
)

const (
	AttributeTmpReserved = "aws.tmp.reserved"
)
