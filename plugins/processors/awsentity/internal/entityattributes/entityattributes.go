// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entityattributes

const (
	AWSEntityPrefix                      = "com.amazonaws.cloudwatch.entity.internal."
	AttributeEntityServiceName           = AWSEntityPrefix + "service.name"
	AttributeEntityDeploymentEnvironment = AWSEntityPrefix + "deployment.environment"
	AttributeEntityCluster               = AWSEntityPrefix + "k8s.cluster.name"
	AttributeEntityNamespace             = AWSEntityPrefix + "k8s.namespace.name"
	AttributeEntityWorkload              = AWSEntityPrefix + "k8s.workload.name"
	AttributeEntityNode                  = AWSEntityPrefix + "k8s.node.name"
	AttributeEntityServiceNameSource     = AWSEntityPrefix + "service.name.source"
	AttributeEntityPlatformType          = AWSEntityPrefix + "platform.type"
	AttributeEntityInstanceID            = AWSEntityPrefix + "instance.id"
	AttributeEntityAutoScalingGroup      = AWSEntityPrefix + "auto.scaling.group"
)

// Container Insights attributes used for scraping EKS related information
const (
	NodeName  = "NodeName"
	Namespace = "Namespace"
	// PodName in Container Insights is the workload(Deployment, Daemonset, etc) name
	PodName = "PodName"
)
