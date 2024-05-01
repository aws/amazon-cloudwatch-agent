// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resolver

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/collector/semconv/v1.18.0"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/common"
	appsignalsconfig "github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/config"
	attr "github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/internal/attributes"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
)

const (
	AttributeEnvironmentDefault = "default"

	AttributePlatformGeneric = "Generic"
	AttributePlatformEC2     = "AWS::EC2"
	AttributePlatformEKS     = "AWS::EKS"
	AttributePlatformK8S     = "K8s"
)

var GenericInheritedAttributes = map[string]string{
	semconv.AttributeDeploymentEnvironment: attr.AWSLocalEnvironment,
	attr.ResourceDetectionHostName:         common.AttributeHost,
}

// DefaultInheritedAttributes is an allow-list that also renames attributes from the resource detection processor
var DefaultInheritedAttributes = map[string]string{
	semconv.AttributeDeploymentEnvironment: attr.AWSLocalEnvironment,
	attr.ResourceDetectionASG:              common.AttributeEC2AutoScalingGroup,
	attr.ResourceDetectionHostId:           common.AttributeEC2InstanceId,
	attr.ResourceDetectionHostName:         common.AttributeHost,
}

type subResolver interface {
	Process(attributes, resourceAttributes pcommon.Map) error
	Stop(ctx context.Context) error
}

type attributesResolver struct {
	subResolvers []subResolver
}

// create a new attributes resolver
func NewAttributesResolver(resolvers []appsignalsconfig.Resolver, logger *zap.Logger) *attributesResolver {
	subResolvers := []subResolver{}
	for _, resolver := range resolvers {
		switch resolver.Platform {
		case appsignalsconfig.PlatformEKS, appsignalsconfig.PlatformK8s:
			subResolvers = append(subResolvers, getKubernetesResolver(resolver.Platform, resolver.Name, logger), newKubernetesResourceAttributesResolver(resolver.Platform, resolver.Name))
		case appsignalsconfig.PlatformEC2:
			subResolvers = append(subResolvers, newResourceAttributesResolver(resolver.Platform, AttributePlatformEC2, DefaultInheritedAttributes))
		default:
			if ecsutil.GetECSUtilSingleton().IsECS() {
				subResolvers = append(subResolvers, newResourceAttributesResolver(appsignalsconfig.PlatformECS, AttributePlatformGeneric, DefaultInheritedAttributes))
			} else {
				subResolvers = append(subResolvers, newResourceAttributesResolver(resolver.Platform, AttributePlatformGeneric, GenericInheritedAttributes))
			}
		}
	}
	return &attributesResolver{
		subResolvers: subResolvers,
	}
}

// Process the attributes
func (r *attributesResolver) Process(attributes, resourceAttributes pcommon.Map, _ bool) error {
	for _, subResolver := range r.subResolvers {
		if err := subResolver.Process(attributes, resourceAttributes); err != nil {
			return err
		}
	}
	return nil
}

func (r *attributesResolver) Stop(ctx context.Context) error {
	var errs error
	for _, subResolver := range r.subResolvers {
		errs = errors.Join(errs, subResolver.Stop(ctx))
	}
	return errs
}

type resourceAttributesResolver struct {
	defaultEnvPrefix string
	platformType     string
	attributeMap     map[string]string
}

func newResourceAttributesResolver(defaultEnvPrefix, platformType string, attributeMap map[string]string) *resourceAttributesResolver {
	return &resourceAttributesResolver{
		defaultEnvPrefix: defaultEnvPrefix,
		platformType:     platformType,
		attributeMap:     attributeMap,
	}
}
func (h *resourceAttributesResolver) Process(attributes, resourceAttributes pcommon.Map) error {
	for attrKey, mappingKey := range h.attributeMap {
		if val, ok := resourceAttributes.Get(attrKey); ok {
			attributes.PutStr(mappingKey, val.Str())
		}
	}
	if val, ok := attributes.Get(attr.AWSLocalEnvironment); !ok || val.Str() == "" {
		if hostedInAttr, found := resourceAttributes.Get(attr.AWSHostedInEnvironment); found && hostedInAttr.Str() != "" {
			attributes.PutStr(attr.AWSLocalEnvironment, hostedInAttr.Str())
		} else {
			if h.defaultEnvPrefix == appsignalsconfig.PlatformECS {
				if clusterName, found := getECSClusterName(resourceAttributes); found {
					attributes.PutStr(attr.AWSLocalEnvironment, getDefaultEnvironment(h.defaultEnvPrefix, clusterName))
				}
			} else if h.defaultEnvPrefix == appsignalsconfig.PlatformEC2 {
				if asgAttr, found := resourceAttributes.Get(attr.ResourceDetectionASG); found {
					attributes.PutStr(attr.AWSLocalEnvironment, getDefaultEnvironment(h.defaultEnvPrefix, asgAttr.Str()))
				}
			}
		}
	}
	if val, ok := attributes.Get(attr.AWSLocalEnvironment); !ok || val.Str() == "" {
		attributes.PutStr(attr.AWSLocalEnvironment, getDefaultEnvironment(h.defaultEnvPrefix, AttributeEnvironmentDefault))
	}
	attributes.PutStr(common.AttributePlatformType, h.platformType)

	return nil
}

func getECSClusterName(resourceAttributes pcommon.Map) (string, bool) {
	if clusterAttr, ok := resourceAttributes.Get(semconv.AttributeAWSECSClusterARN); ok {
		parts := strings.Split(clusterAttr.Str(), "/")
		clusterName := parts[len(parts)-1]
		return clusterName, true
	} else if taskAttr, ok := resourceAttributes.Get(semconv.AttributeAWSECSTaskARN); ok {
		parts := strings.SplitAfterN(taskAttr.Str(), ":task/", 2)
		if len(parts) == 2 {
			taskParts := strings.Split(parts[1], "/")
			// cluster name in ARN
			if len(taskParts) == 2 {
				return taskParts[0], true
			}
		}
	}
	return "", false
}

func getDefaultEnvironment(platformCode, val string) string {
	return fmt.Sprintf("%s:%s", platformCode, val)
}

func (h *resourceAttributesResolver) Stop(ctx context.Context) error {
	return nil
}
