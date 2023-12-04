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

var DefaultHostedInAttributes = map[string]string{
	attr.AWSHostedInEnvironment: attr.HostedInEnvironment,
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
		if resolver.Platform == appsignalsconfig.PlatformEKS {
			subResolvers = append(subResolvers, getEksResolver(logger), newEKSHostedInAttributeResolver())
		} else {
			subResolvers = append(subResolvers, newHostedInAttributeResolver(DefaultHostedInAttributes))
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
	attributeMap map[string]string
}

func newHostedInAttributeResolver(attributeMap map[string]string) *hostedInAttributeResolver {
	return &hostedInAttributeResolver{
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
		hostedInEnv := "Generic"
		attributes.PutStr(attr.HostedInEnvironment, hostedInEnv)
	}

	return nil
}

func (h *hostedInAttributeResolver) Stop(ctx context.Context) error {
	return nil
}
