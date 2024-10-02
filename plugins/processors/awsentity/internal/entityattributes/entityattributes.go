// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entityattributes

const (
	// The following are the possible values for EntityType config options
	Resource = "Resource"
	Service  = "Service"

	// The following are entity related attributes
	AWSEntityPrefix                    = "com.amazonaws.cloudwatch.entity.internal."
	AttributeEntityType                = AWSEntityPrefix + "type"
	AttributeEntityAWSResource         = "AWS::Resource"
	AttributeEntityResourceType        = AWSEntityPrefix + "resource.type"
	AttributeEntityEC2InstanceResource = "AWS::EC2::Instance"
	AttributeEntityIdentifier          = AWSEntityPrefix + "identifier"

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

	// The following are possible platform values
	AttributeEntityEC2Platform = "AWS::EC2"
	AttributeEntityEKSPlatform = "AWS::EKS"
	AttributeEntityK8sPlatform = "K8s"
)
