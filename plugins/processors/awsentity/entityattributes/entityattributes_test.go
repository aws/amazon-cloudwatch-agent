// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entityattributes

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestProcessAndRemoveEntityAttributes(t *testing.T) {
	testCases := []struct {
		name               string
		resourceAttributes map[string]any
		wantedAttributes   map[string]string
		leftoverAttributes map[string]any
	}{
		{
			name: "key_attributes",
			resourceAttributes: map[string]any{
				AttributeEntityServiceName:           "my-service",
				AttributeEntityDeploymentEnvironment: "my-environment",
			},
			wantedAttributes: map[string]string{
				ServiceName:           "my-service",
				DeploymentEnvironment: "my-environment",
			},
			leftoverAttributes: make(map[string]any),
		},
		{
			name: "non-key_attributes",
			resourceAttributes: map[string]any{
				AttributeEntityNamespace:    "my-namespace",
				AttributeEntityNode:         "my-node",
				AttributeEntityWorkload:     "my-workload",
				AttributeEntityPlatformType: "AWS::EKS",
			},
			wantedAttributes: map[string]string{
				NamespaceField: "my-namespace",
				Node:           "my-node",
				Workload:       "my-workload",
				Platform:       "AWS::EKS",
			},
			leftoverAttributes: make(map[string]any),
		},
		{
			name: "key_and_non_key_attributes",
			resourceAttributes: map[string]any{
				AttributeEntityServiceName:           "my-service",
				AttributeEntityDeploymentEnvironment: "my-environment",
				AttributeEntityNamespace:             "my-namespace",
				AttributeEntityNode:                  "my-node",
				AttributeEntityWorkload:              "my-workload",
				AttributeEntityPlatformType:          "K8s",
			},
			wantedAttributes: map[string]string{
				ServiceName:           "my-service",
				DeploymentEnvironment: "my-environment",
				NamespaceField:        "my-namespace",
				Node:                  "my-node",
				Workload:              "my-workload",
				Platform:              "K8s",
			},
			leftoverAttributes: make(map[string]any),
		},
		{
			name: "key_and_non_key_attributes_plus_extras",
			resourceAttributes: map[string]any{
				"extra_attribute":                    "extra_value",
				AttributeEntityServiceName:           "my-service",
				AttributeEntityDeploymentEnvironment: "my-environment",
				AttributeEntityNamespace:             "my-namespace",
				AttributeEntityNode:                  "my-node",
				AttributeEntityWorkload:              "my-workload",
				AttributeEntityPlatformType:          "K8s",
			},
			wantedAttributes: map[string]string{
				ServiceName:           "my-service",
				DeploymentEnvironment: "my-environment",
				NamespaceField:        "my-namespace",
				Node:                  "my-node",
				Workload:              "my-workload",
				Platform:              "K8s",
			},
			leftoverAttributes: map[string]any{
				"extra_attribute": "extra_value",
			},
		},
		{
			name: "key_and_non_key_attributes_plus_unsupported_entity_field",
			resourceAttributes: map[string]any{
				AWSEntityPrefix + "not.real.values":  "unsupported",
				AttributeEntityServiceName:           "my-service",
				AttributeEntityDeploymentEnvironment: "my-environment",
				AttributeEntityNamespace:             "my-namespace",
				AttributeEntityNode:                  "my-node",
				AttributeEntityWorkload:              "my-workload",
				AttributeEntityPlatformType:          "AWS::EKS",
			},
			wantedAttributes: map[string]string{
				ServiceName:           "my-service",
				DeploymentEnvironment: "my-environment",
				NamespaceField:        "my-namespace",
				Node:                  "my-node",
				Workload:              "my-workload",
				Platform:              "AWS::EKS",
			},
			leftoverAttributes: map[string]any{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			attrs := pcommon.NewMap()
			err := attrs.FromRaw(tc.resourceAttributes)

			// resetting fields for current test case
			entityAttrMap := []map[string]string{keyAttributeEntityToShortNameMap}
			platformType := ""
			if platformTypeValue, ok := attrs.Get(AttributeEntityPlatformType); ok {
				platformType = platformTypeValue.Str()
			}
			if platformType != "" {
				delete(attributeEntityToShortNameMap, AttributeEntityCluster)
				entityAttrMap = append(entityAttrMap, attributeEntityToShortNameMap)
			}
			assert.Nil(t, err)
			targetMap := make(map[string]string)
			for _, entityMap := range entityAttrMap {
				processEntityAttributes(entityMap, targetMap, attrs)
			}
			removeEntityFields(attrs)
			assert.Equal(t, tc.leftoverAttributes, attrs.AsRaw())
			assert.Equal(t, tc.wantedAttributes, targetMap)
		})
	}
}

func TestCreateCloudWatchEntityFromAttributes_WithoutAccountID(t *testing.T) {
	resourceMetrics := pmetric.NewResourceMetrics()
	resourceMetrics.Resource().Attributes().PutStr(AttributeEntityType, "Service")
	resourceMetrics.Resource().Attributes().PutStr(AttributeEntityDeploymentEnvironment, "my-environment")
	resourceMetrics.Resource().Attributes().PutStr(AttributeEntityServiceName, "my-service")
	resourceMetrics.Resource().Attributes().PutStr(AttributeEntityNode, "my-node")
	resourceMetrics.Resource().Attributes().PutStr(AttributeEntityCluster, "my-cluster")
	resourceMetrics.Resource().Attributes().PutStr(AttributeEntityNamespace, "my-namespace")
	resourceMetrics.Resource().Attributes().PutStr(AttributeEntityWorkload, "my-workload")
	resourceMetrics.Resource().Attributes().PutStr(AttributeEntityPlatformType, "AWS::EKS")
	assert.Equal(t, 8, resourceMetrics.Resource().Attributes().Len())

	expectedEntity := types.Entity{
		KeyAttributes: nil,
		Attributes:    nil,
	}
	entity := CreateCloudWatchEntityFromAttributes(resourceMetrics.Resource().Attributes())
	assert.Equal(t, 8, resourceMetrics.Resource().Attributes().Len())
	assert.Equal(t, expectedEntity, entity)
}

func TestCreateCloudWatchEntityFromAttributes_WithAccountID(t *testing.T) {
	resourceMetrics := pmetric.NewResourceMetrics()
	resourceMetrics.Resource().Attributes().PutStr(AttributeEntityType, "Service")
	resourceMetrics.Resource().Attributes().PutStr(AttributeEntityDeploymentEnvironment, "my-environment")
	resourceMetrics.Resource().Attributes().PutStr(AttributeEntityServiceName, "my-service")
	resourceMetrics.Resource().Attributes().PutStr(AttributeEntityNode, "my-node")
	resourceMetrics.Resource().Attributes().PutStr(AttributeEntityCluster, "my-cluster")
	resourceMetrics.Resource().Attributes().PutStr(AttributeEntityNamespace, "my-namespace")
	resourceMetrics.Resource().Attributes().PutStr(AttributeEntityWorkload, "my-workload")
	resourceMetrics.Resource().Attributes().PutStr(AttributeEntityPlatformType, "AWS::EKS")
	resourceMetrics.Resource().Attributes().PutStr(AttributeEntityAwsAccountId, "123456789")
	assert.Equal(t, 9, resourceMetrics.Resource().Attributes().Len())

	expectedEntity := types.Entity{
		KeyAttributes: map[string]string{
			EntityType:            Service,
			ServiceName:           "my-service",
			DeploymentEnvironment: "my-environment",
			AwsAccountId:          "123456789",
		},
		Attributes: map[string]string{
			Node:           "my-node",
			EksCluster:     "my-cluster",
			NamespaceField: "my-namespace",
			Workload:       "my-workload",
			Platform:       "AWS::EKS",
		},
	}
	entity := CreateCloudWatchEntityFromAttributes(resourceMetrics.Resource().Attributes())
	assert.Equal(t, 0, resourceMetrics.Resource().Attributes().Len())
	assert.Equal(t, expectedEntity, entity)
}

func TestCreateCloudWatchEntityFromAttributesOnK8s(t *testing.T) {
	entityMap := attributeEntityToShortNameMap
	delete(entityMap, AttributeEntityCluster)
	resourceMetrics := pmetric.NewResourceMetrics()
	resourceMetrics.Resource().Attributes().PutStr(AttributeEntityType, "Service")
	resourceMetrics.Resource().Attributes().PutStr(AttributeEntityDeploymentEnvironment, "my-environment")
	resourceMetrics.Resource().Attributes().PutStr(AttributeEntityServiceName, "my-service")
	resourceMetrics.Resource().Attributes().PutStr(AttributeEntityNode, "my-node")
	resourceMetrics.Resource().Attributes().PutStr(AttributeEntityCluster, "my-cluster")
	resourceMetrics.Resource().Attributes().PutStr(AttributeEntityNamespace, "my-namespace")
	resourceMetrics.Resource().Attributes().PutStr(AttributeEntityWorkload, "my-workload")
	resourceMetrics.Resource().Attributes().PutStr(AttributeEntityPlatformType, "K8s")
	resourceMetrics.Resource().Attributes().PutStr(AttributeEntityAwsAccountId, "123456789")
	assert.Equal(t, 9, resourceMetrics.Resource().Attributes().Len())

	expectedEntity := types.Entity{
		KeyAttributes: map[string]string{
			EntityType:            Service,
			ServiceName:           "my-service",
			DeploymentEnvironment: "my-environment",
			AwsAccountId:          "123456789",
		},
		Attributes: map[string]string{
			Node:           "my-node",
			K8sCluster:     "my-cluster",
			NamespaceField: "my-namespace",
			Workload:       "my-workload",
			Platform:       "K8s",
		},
	}
	entity := CreateCloudWatchEntityFromAttributes(resourceMetrics.Resource().Attributes())
	assert.Equal(t, 0, resourceMetrics.Resource().Attributes().Len())
	assert.Equal(t, expectedEntity, entity)
}

func TestCreateCloudWatchEntityFromAttributesOnEc2(t *testing.T) {
	resourceMetrics := pmetric.NewResourceMetrics()
	resourceMetrics.Resource().Attributes().PutStr(AttributeEntityType, "Service")
	resourceMetrics.Resource().Attributes().PutStr(AttributeEntityDeploymentEnvironment, "my-environment")
	resourceMetrics.Resource().Attributes().PutStr(AttributeEntityServiceName, "my-service")
	resourceMetrics.Resource().Attributes().PutStr(AttributeEntityPlatformType, "AWS::EC2")
	resourceMetrics.Resource().Attributes().PutStr(AttributeEntityAwsAccountId, "123456789")
	assert.Equal(t, 5, resourceMetrics.Resource().Attributes().Len())

	expectedEntity := types.Entity{
		KeyAttributes: map[string]string{
			EntityType:            Service,
			ServiceName:           "my-service",
			DeploymentEnvironment: "my-environment",
			AwsAccountId:          "123456789",
		},
		Attributes: map[string]string{
			Platform: "AWS::EC2",
		},
	}
	entity := CreateCloudWatchEntityFromAttributes(resourceMetrics.Resource().Attributes())
	assert.Equal(t, 0, resourceMetrics.Resource().Attributes().Len())
	assert.Equal(t, expectedEntity, entity)
}
