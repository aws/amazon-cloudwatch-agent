// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entityattributes

import (
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatch"
)

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

// shortNameToEntityMap is the reverse mapping of keyAttributeEntityToShortNameMap
var keyAttributeEntityToLongNameMap = map[string]string{
	EntityType:            AttributeEntityType,
	ResourceType:          AttributeEntityResourceType,
	Identifier:            AttributeEntityIdentifier,
	AwsAccountId:          AttributeEntityAwsAccountId,
	ServiceName:           AttributeEntityServiceName,
	DeploymentEnvironment: AttributeEntityDeploymentEnvironment,
}

// shortNameToAttributeMap is the reverse mapping of attributeEntityToShortNameMap
var attributeEntityToLongNameMap = map[string]string{
	NamespaceField:    AttributeEntityNamespace,
	Workload:          AttributeEntityWorkload,
	Node:              AttributeEntityNode,
	Platform:          AttributeEntityPlatformType,
	InstanceID:        AttributeEntityInstanceID,
	AutoscalingGroup:  AttributeEntityAutoScalingGroup,
	ServiceNameSource: AttributeEntityServiceNameSource,
}

// GetFullAttributeName returns the full attribute name for a given short name
func GetFullAttributeName(shortName string) (string, bool) {
	// First check key attributes
	if fullName, ok := keyAttributeEntityToLongNameMap[shortName]; ok {
		return fullName, true
	}
	// Then check regular attributes
	if fullName, ok := attributeEntityToLongNameMap[shortName]; ok {
		return fullName, true
	}
	return "", false
}

// IsAllowedKeyAttribute checks if the given key is an allowed entity key attribute name
func IsAllowedKeyAttribute(key string) bool {
	_, exists := keyAttributeEntityToLongNameMap[key]
	return exists
}

// IsAllowedAttribute checks if the given key is an allowed attribute name
func IsAllowedAttribute(key string) bool {
	_, exists := attributeEntityToLongNameMap[key]
	return exists
}

func CreateCloudWatchEntityFromAttributes(resourceAttributes pcommon.Map) cloudwatch.Entity {
	keyAttributesMap := map[string]*string{}
	attributeMap := map[string]*string{}

	// Process KeyAttributes and return empty entity if AwsAccountId is not found
	processEntityAttributes(keyAttributeEntityToShortNameMap, keyAttributesMap, resourceAttributes)
	if _, ok := keyAttributesMap[AwsAccountId]; !ok {
		return cloudwatch.Entity{}
	}

	// Process Attributes and add cluster attribute if on EKS/K8s
	processEntityAttributes(attributeEntityToShortNameMap, attributeMap, resourceAttributes)
	if platformTypeValue, ok := resourceAttributes.Get(AttributeEntityPlatformType); ok {
		platformType := clusterType(platformTypeValue.Str())
		if clusterNameValue, ok := resourceAttributes.Get(AttributeEntityCluster); ok {
			attributeMap[platformType] = aws.String(clusterNameValue.Str())
		}
	}

	// Remove entity fields from attributes and return the entity
	removeEntityFields(resourceAttributes)
	return cloudwatch.Entity{
		KeyAttributes: keyAttributesMap,
		Attributes:    attributeMap,
	}
}

// processEntityAttributes fetches the fields with entity prefix and creates an entity to be sent at the PutMetricData call.
func processEntityAttributes(entityMap map[string]string, targetMap map[string]*string, incomingResourceAttributes pcommon.Map) {
	for entityField, shortName := range entityMap {
		if val, ok := incomingResourceAttributes.Get(entityField); ok {
			if strVal := val.Str(); strVal != "" {
				targetMap[shortName] = aws.String(strVal)
			}
		}
	}
}

func clusterType(platformType string) string {
	if platformType == AttributeEntityEKSPlatform {
		return EksCluster
	} else if platformType == AttributeEntityK8sPlatform {
		return K8sCluster
	}
	return ""
}

// removeEntityFields so that it is not tagged as a dimension, and reduces the size of the PMD payload.
func removeEntityFields(mutableResourceAttributes pcommon.Map) {
	mutableResourceAttributes.RemoveIf(func(s string, _ pcommon.Value) bool {
		return strings.HasPrefix(s, AWSEntityPrefix)
	})
}
