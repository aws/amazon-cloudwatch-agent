// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entityattributes

import "sync"

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
	EksCluster            = "EKS.Cluster"
	K8sCluster            = "K8s.Cluster"
	NamespaceField        = "K8s.Namespace"
	Workload              = "K8s.Workload"
	Node                  = "K8s.Node"
	ServiceNameSource     = "AWS.ServiceNameSource"
	Platform              = "PlatformType"
	InstanceID            = "EC2.InstanceId"
	AutoscalingGroup      = "EC2.AutoScalingGroup"

	// The following are values used for the environment fallbacks required on EC2
	DeploymentEnvironmentFallbackPrefix = "ec2:"
	DeploymentEnvironmentDefault        = DeploymentEnvironmentFallbackPrefix + "default"
)

// KeyAttributeEntityToShortNameMap is used to map key attributes from otel to the actual values used in the Entity object
var keyAttributeEntityToShortNameMap = map[string]string{
	AttributeEntityType:                  EntityType,
	AttributeEntityResourceType:          ResourceType,
	AttributeEntityIdentifier:            Identifier,
	AttributeEntityAwsAccountId:          AwsAccountId,
	AttributeEntityServiceName:           ServiceName,
	AttributeEntityDeploymentEnvironment: DeploymentEnvironment,
}

// attributeEntityToShortNameMap is used to map attributes from otel to the actual values used in the Entity object
var attributeEntityToShortNameMap = map[string]string{
	AttributeEntityNamespace:         NamespaceField,
	AttributeEntityWorkload:          Workload,
	AttributeEntityNode:              Node,
	AttributeEntityPlatformType:      Platform,
	AttributeEntityInstanceID:        InstanceID,
	AttributeEntityAutoScalingGroup:  AutoscalingGroup,
	AttributeEntityServiceNameSource: ServiceNameSource,
}

var AttributeEntityToShortNameMapRWMutex = sync.RWMutex{}

func GetKeyAttributeEntityShortNameMap() map[string]string {
	return keyAttributeEntityToShortNameMap
}

// Cluster attribute prefix could be either EKS or K8s. We set the field once at runtime.
func GetAttributeEntityShortNameMap(platformType string) map[string]string {
	AttributeEntityToShortNameMapRWMutex.Lock()
	defer AttributeEntityToShortNameMapRWMutex.Unlock()

	if _, ok := attributeEntityToShortNameMap[AttributeEntityCluster]; !ok {
		attributeEntityToShortNameMap[AttributeEntityCluster] = clusterType(platformType)
	}
	return attributeEntityToShortNameMap
}

func clusterType(platformType string) string {
	if platformType == AttributeEntityEKSPlatform {
		return EksCluster
	} else if platformType == AttributeEntityK8sPlatform {
		return K8sCluster
	}
	return ""
}
