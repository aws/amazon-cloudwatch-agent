// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resolver

import (
	"context"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/collector/semconv/v1.22.0"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/common"
	attr "github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/internal/attributes"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
)

type ecsResourceAttributesResolver struct {
	resourceAttributesResolver
	hostIn string
}

func (e *ecsResourceAttributesResolver) Process(attributes, resourceAttributes pcommon.Map) error {
	for attrKey, mappingKey := range e.attributeMap {
		if val, ok := resourceAttributes.Get(attrKey); ok {
			attributes.PutStr(mappingKey, val.Str())
		}
	}

	clusterName, taskId := getECSResourcesFromResourceAttributes(resourceAttributes)
	if clusterName == "" {
		clusterName = ecsutil.GetECSUtilSingleton().Cluster
	}

	attributes.PutStr(common.AttributePlatformType, e.platformType)
	attributes.PutStr(attr.AWSLocalEnvironment, e.getLocalEnvironment(attributes, resourceAttributes, clusterName))
	attributes.PutStr(attr.AWSECSClusterName, clusterName)
	if taskId != "" {
		attributes.PutStr(attr.AWSECSTaskID, taskId)
	}
	return nil
}

// getLocalEnvironment determines the environment based on the following priority:
// 1. aws.local.environment (from deployment.environment)
// 2. aws.hostedin.environment (deprecated soon)
// 3. hosted_in (user-specified)
// 4. aws.ecs.cluster.arn (auto-detected)
// 5. aws.ecs.task.arn (auto-detected)
// 6. Cluster name from CWA (auto-detected)
// 7. Hardcoded `default`
func (e *ecsResourceAttributesResolver) getLocalEnvironment(attributes pcommon.Map, resourceAttributes pcommon.Map, clusterName string) string {
	if val, ok := attributes.Get(attr.AWSLocalEnvironment); ok {
		return val.Str()
	}
	if val, found := resourceAttributes.Get(attr.AWSHostedInEnvironment); found {
		return val.Str()
	}
	if e.hostIn != "" {
		return generateLocalEnvironment(e.defaultEnvPrefix, e.hostIn)
	}
	if clusterName != "" {
		return generateLocalEnvironment(e.defaultEnvPrefix, clusterName)
	}
	return generateLocalEnvironment(e.defaultEnvPrefix, AttributeEnvironmentDefault)
}

func (e *ecsResourceAttributesResolver) Stop(ctx context.Context) error {
	return nil
}

func newECSResourceAttributesResolver(defaultEnvPrefix string, hostIn string) *ecsResourceAttributesResolver {
	return &ecsResourceAttributesResolver{
		resourceAttributesResolver: resourceAttributesResolver{
			defaultEnvPrefix: defaultEnvPrefix,
			platformType:     AttributePlatformECS,
			attributeMap:     DefaultInheritedAttributes,
		},
		hostIn: hostIn,
	}
}

func getECSResourcesFromResourceAttributes(resourceAttributes pcommon.Map) (clusterName, taskId string) {
	if clusterAttr, ok := resourceAttributes.Get(semconv.AttributeAWSECSClusterARN); ok {
		parts := strings.Split(clusterAttr.Str(), "/")
		clusterName = parts[len(parts)-1]
	}
	if taskAttr, ok := resourceAttributes.Get(semconv.AttributeAWSECSTaskARN); ok {
		parts := strings.SplitAfterN(taskAttr.Str(), ":task/", 2)
		if len(parts) == 2 {
			taskParts := strings.Split(parts[1], "/")
			// New Task ARN format "task/cluster-name/task-id".
			if len(taskParts) == 2 {
				taskId = taskParts[1]
				if clusterName == "" {
					clusterName = taskParts[0]
				}
			} else if len(taskParts) == 1 {
				// Legacy Task ARN format "task/task-id".
				taskId = taskParts[0]
			}
		}
	}
	return
}
