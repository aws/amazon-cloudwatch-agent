// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resolver

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	appsignalsconfig "github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsappsignals/config"
	attr "github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsappsignals/internal/attributes"
)

const AttributePlatformGeneric = "Generic"

var DefaultHostedInAttributes = map[string]string{
	attr.AWSHostedInEnvironment:    attr.HostedInEnvironment,
	attr.ResourceDetectionHostName: attr.ResourceDetectionHostName,
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
	//TODO: Logic for native k8s needs to be implemented
	subResolvers := []subResolver{}
	for _, resolver := range resolvers {
		switch resolver.Platform {
		case appsignalsconfig.PlatformEKS, appsignalsconfig.PlatformK8s:
			subResolvers = append(subResolvers, getKubernetesResolver(logger), newKubernetesHostedInAttributeResolver(resolver.Name))
		case appsignalsconfig.PlatformEC2:
			subResolvers = append(subResolvers, newEC2HostedInAttributeResolver(resolver.Name))
		default:
			subResolvers = append(subResolvers, newHostedInAttributeResolver(resolver.Name, DefaultHostedInAttributes))
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
		if err := subResolver.Stop(ctx); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	return errs
}

type hostedInAttributeResolver struct {
	name         string
	attributeMap map[string]string
}

func newHostedInAttributeResolver(name string, attributeMap map[string]string) *hostedInAttributeResolver {
	if name == "" {
		name = AttributePlatformGeneric
	}
	return &hostedInAttributeResolver{
		name:         name,
		attributeMap: attributeMap,
	}
}
func (h *hostedInAttributeResolver) Process(attributes, resourceAttributes pcommon.Map) error {
	for attrKey, mappingKey := range h.attributeMap {
		if val, ok := resourceAttributes.Get(attrKey); ok {
			attributes.PutStr(mappingKey, val.AsString())
		}
	}

	if _, ok := resourceAttributes.Get(attr.AWSHostedInEnvironment); !ok {
		attributes.PutStr(attr.HostedInEnvironment, h.name)
	}

	return nil
}

func (h *hostedInAttributeResolver) Stop(ctx context.Context) error {
	return nil
}
