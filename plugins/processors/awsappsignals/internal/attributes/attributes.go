// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package attributes

const (
	// aws attributes
	AWSLocalService        = "aws.local.service"
	AWSLocalOperation      = "aws.local.operation"
	AWSRemoteService       = "aws.remote.service"
	AWSRemoteOperation     = "aws.remote.operation"
	AWSRemoteTarget        = "aws.remote.target"
	AWSHostedInEnvironment = "aws.hostedin.environment"

	// kubernetes resource attributes
	K8SDeploymentName  = "k8s.deployment.name"
	K8SStatefulSetName = "k8s.statefulset.name"
	K8SDaemonSetName   = "k8s.daemonset.name"
	K8SJobName         = "k8s.job.name"
	K8SCronJobName     = "k8s.cronjob.name"
	K8SPodName         = "k8s.pod.name"
	K8SRemoteNamespace = "K8s.RemoteNamespace"

	// hosted in attribute names
	HostedInClusterName  = "HostedIn.EKS.Cluster"
	HostedInK8SNamespace = "HostedIn.K8s.Namespace"
	HostedInEnvironment  = "HostedIn.Environment"
)
