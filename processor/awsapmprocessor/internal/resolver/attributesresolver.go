// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resolver

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
)

type subResolver interface {
	Process(attributes, resourceAttributes pcommon.Map) error
	Stop(ctx context.Context) error
}

type attributesResolver struct {
	subResolvers []subResolver
}

// create a new attributes resolver
func NewAttributesResolver(resolverNames []string, logger *zap.Logger) *attributesResolver {
	subResolvers := []subResolver{}
	for _, resolverName := range resolverNames {
		if resolverName == "eks" {
			subResolvers = append(subResolvers, getEksResolver(logger))
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
	for _, subResolver := range r.subResolvers {
		if err := subResolver.Stop(ctx); err != nil {
			return err
		}
	}
	return nil
}
