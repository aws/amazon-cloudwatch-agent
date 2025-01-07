// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resolver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/collector/semconv/v1.22.0"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/common"
	appsignalsconfig "github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/config"
	attr "github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/internal/attributes"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
)

func TestResourceAttributesResolverWithECSClusterName(t *testing.T) {
	testCases := []struct {
		name                    string
		hostIn                  string
		ecsTaskArn              string
		autoDetectedClusterName string
		expectedClusterName     string
		expectedEnvironmentName string
	}{
		{
			name:                    "testECSClusterFromTaskArn",
			hostIn:                  "",
			ecsTaskArn:              "arn:aws:ecs:us-west-1:123456789123:task/my-cluster/10838bed-421f-43ef-870a-f43feacbbb5b",
			expectedClusterName:     "my-cluster",
			expectedEnvironmentName: "ecs:my-cluster",
		},
		{
			name:                    "testECSClusterFromHostIn",
			hostIn:                  "host-in",
			ecsTaskArn:              "arn:aws:ecs:us-west-1:123456789123:task/my-cluster/10838bed-421f-43ef-870a-f43feacbbb5b",
			expectedClusterName:     "my-cluster",
			expectedEnvironmentName: "ecs:host-in",
		},
		{
			name:                    "testECSClusterFromECSUtil",
			hostIn:                  "",
			ecsTaskArn:              "",
			autoDetectedClusterName: "my-cluster",
			expectedClusterName:     "my-cluster",
			expectedEnvironmentName: "ecs:my-cluster",
		},
		{
			name:                    "testECSClusterDefault",
			hostIn:                  "",
			ecsTaskArn:              "",
			autoDetectedClusterName: "",
			expectedClusterName:     "",
			expectedEnvironmentName: "ecs:default",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ecsutil.GetECSUtilSingleton().Cluster = tc.autoDetectedClusterName
			resolver := newECSResourceAttributesResolver(appsignalsconfig.PlatformECS, tc.hostIn)

			attributes := pcommon.NewMap()
			resourceAttributes := pcommon.NewMap()
			resourceAttributes.PutStr(semconv.AttributeAWSECSTaskARN, tc.ecsTaskArn)

			resolver.Process(attributes, resourceAttributes)

			attribute, ok := attributes.Get(common.AttributePlatformType)
			assert.True(t, ok)
			assert.Equal(t, AttributePlatformECS, attribute.Str())

			attribute, ok = attributes.Get(attr.AWSECSClusterName)
			assert.True(t, ok)
			assert.Equal(t, tc.expectedClusterName, attribute.Str())

			attribute, ok = attributes.Get(attr.AWSLocalEnvironment)
			assert.True(t, ok)
			assert.Equal(t, tc.expectedEnvironmentName, attribute.Str())
		})
	}
	ecsutil.GetECSUtilSingleton().Cluster = ""
}

func TestGetClusterName(t *testing.T) {
	resourceAttributes := pcommon.NewMap()
	resourceAttributes.PutStr(semconv.AttributeAWSECSClusterARN, "arn:aws:ecs:us-west-2:123456789123:cluster/my-cluster")
	clusterName, taskId := getECSResourcesFromResourceAttributes(resourceAttributes)
	assert.Equal(t, "my-cluster", clusterName)
	assert.Equal(t, "", taskId)

	resourceAttributes = pcommon.NewMap()
	resourceAttributes.PutStr(semconv.AttributeAWSECSTaskARN, "arn:aws:ecs:us-west-1:123456789123:task/10838bedacbbb5b")
	clusterName, taskId = getECSResourcesFromResourceAttributes(resourceAttributes)
	assert.Equal(t, "", clusterName)
	assert.Equal(t, "10838bedacbbb5b", taskId)

	resourceAttributes = pcommon.NewMap()
	resourceAttributes.PutStr(semconv.AttributeAWSECSTaskARN, "arn:aws:ecs:us-west-1:123456789123:task/my-cluster/10838bedacbbb5b")
	clusterName, taskId = getECSResourcesFromResourceAttributes(resourceAttributes)
	assert.Equal(t, "my-cluster", clusterName)
	assert.Equal(t, "10838bedacbbb5b", taskId)
}
