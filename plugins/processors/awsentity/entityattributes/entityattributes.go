// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entityattributes

const (

	// The following are the possible values for EntityType config options
	Resource = "Resource"
	Service  = "Service"

	AttributeServiceNameSource           = "service.name.source"
	AttributeDeploymentEnvironmentSource = "deployment.environment.source"
	AttributeServiceNameSourceUserConfig = "UserConfiguration"

	// The following are entity related attributes
	AWSEntityPrefix                      = "com.amazonaws.cloudwatch.entity.internal."
	AttributeEntityType                  = AWSEntityPrefix + "type"
	AttributeEntityAWSResource           = "AWS::Resource"
	AttributeEntityResourceType          = AWSEntityPrefix + "resource.type"
	AttributeEntityEC2InstanceResource   = "AWS::EC2::Instance"
	AttributeEntityIdentifier            = AWSEntityPrefix + "identifier"
	AttributeEntityAwsAccountId          = AWSEntityPrefix + "aws.account.id"
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

	// The following Fields are the actual names attached to the Entity requests.
	ServiceName           = "Name"
	DeploymentEnvironment = "Environment"
	EntityType            = "Type"
	ResourceType          = "ResourceType"
	Identifier            = "Identifier"
	AwsAccountId          = "AwsAccountId"
	Cluster               = "Cluster"
	Workload              = "Workload"
	Node                  = "Node"
	ServiceNameSource     = "Source"
	Platform              = "Platform"
	InstanceID            = "InstanceID"
	AutoscalingGroup      = "AutoScalingGroup"

	// The following are values used for the environment fallbacks required on EC2
	DeploymentEnvironmentFallbackPrefix = "ec2:"
	DeploymentEnvironmentDefault        = DeploymentEnvironmentFallbackPrefix + "default"
)

// KeyAttributeEntityToShortNameMap is used to map key attributes from otel to the actual values used in the Entity object
var KeyAttributeEntityToShortNameMap = map[string]string{
	AttributeEntityType:                  EntityType,
	AttributeEntityResourceType:          ResourceType,
	AttributeEntityIdentifier:            Identifier,
	AttributeEntityAwsAccountId:          AwsAccountId,
	AttributeEntityServiceName:           ServiceName,
	AttributeEntityDeploymentEnvironment: DeploymentEnvironment,
}

// AttributeEntityToShortNameMap is used to map attributes from otel to the actual values used in the Entity object
var AttributeEntityToShortNameMap = map[string]string{
	AttributeEntityCluster:           Cluster,
	AttributeEntityNamespace:         Namespace,
	AttributeEntityWorkload:          Workload,
	AttributeEntityNode:              Node,
	AttributeEntityPlatformType:      Platform,
	AttributeEntityInstanceID:        InstanceID,
	AttributeEntityAutoScalingGroup:  AutoscalingGroup,
	AttributeEntityServiceNameSource: ServiceNameSource,
}

// Container Insights attributes used for scraping EKS related information
const (
	NodeName  = "NodeName"
	Namespace = "Namespace"
	// PodName in Container Insights is the workload(Deployment, Daemonset, etc) name
	PodName = "PodName"
)
