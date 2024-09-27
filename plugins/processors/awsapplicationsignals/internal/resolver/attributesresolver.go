// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resolver

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/collector/semconv/v1.22.0"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/common"
	appsignalsconfig "github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/config"
	attr "github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/internal/attributes"
)

const (
	AttributeEnvironmentDefault = "default"

	AttributePlatformGeneric = "Generic"
	AttributePlatformEC2     = "AWS::EC2"
	AttributePlatformEKS     = "AWS::EKS"
	AttributePlatformECS     = "AWS::ECS"
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
		case appsignalsconfig.PlatformECS:
			subResolvers = append(subResolvers, newECSResourceAttributesResolver(resolver.Platform, resolver.Name))
		default:
			subResolvers = append(subResolvers, newResourceAttributesResolver(resolver.Platform, AttributePlatformGeneric, GenericInheritedAttributes))
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
	attributes.PutStr(attr.AWSLocalEnvironment, getLocalEnvironment(attributes, resourceAttributes, h.defaultEnvPrefix))
	attributes.PutStr(common.AttributePlatformType, h.platformType)
	return nil
}

func getLocalEnvironment(attributes, resourceAttributes pcommon.Map, defaultEnvPrefix string) string {
	if val, ok := attributes.Get(attr.AWSLocalEnvironment); ok {
		return val.Str()
	}
	if val, found := resourceAttributes.Get(attr.AWSHostedInEnvironment); found {
		return val.Str()
	}
	if defaultEnvPrefix == appsignalsconfig.PlatformEC2 {
		if asgAttr, found := resourceAttributes.Get(attr.ResourceDetectionASG); found {
			return generateLocalEnvironment(defaultEnvPrefix, asgAttr.Str())
		}
	}
	return generateLocalEnvironment(defaultEnvPrefix, AttributeEnvironmentDefault)
}

func generateLocalEnvironment(platformCode, val string) string {
	return fmt.Sprintf("%s:%s", platformCode, val)
}

func (h *resourceAttributesResolver) Stop(ctx context.Context) error {
	return nil
}
