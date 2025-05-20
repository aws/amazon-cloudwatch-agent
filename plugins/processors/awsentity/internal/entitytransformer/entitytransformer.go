// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entitytransformer

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/internal/entity"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsentity/entityattributes"
)

type EntityTransformer struct {
	transform *entity.Transform
	logger    *zap.Logger
}

func NewEntityTransformer(transform *entity.Transform, logger *zap.Logger) *EntityTransformer {
	return &EntityTransformer{
		transform: transform,
		logger:    logger,
	}
}

func (p *EntityTransformer) ApplyTransforms(resourceAttrs pcommon.Map) {
	if p.transform == nil {
		return
	}

	// Apply key attributes
	for _, keyAttr := range p.transform.KeyAttributes {
		if fullName, ok := entityattributes.GetFullAttributeName(keyAttr.Key); ok {
			resourceAttrs.PutStr(fullName, keyAttr.Value)
		} else {
			p.logger.Debug("Unrecognized key attribute", zap.String("key", keyAttr.Key))
		}
	}

	// Apply additional attributes
	for _, attr := range p.transform.Attributes {
		if fullName, ok := entityattributes.GetFullAttributeName(attr.Key); ok {
			resourceAttrs.PutStr(fullName, attr.Value)
		} else {
			p.logger.Debug("Unrecognized attribute", zap.String("key", attr.Key))
		}
	}
}

func (p *EntityTransformer) GetOverriddenServiceName() (string, string) {
	if p.transform == nil {
		return "", ""
	}

	var serviceName, source string
	for _, keyAttr := range p.transform.KeyAttributes {
		if keyAttr.Key == entityattributes.ServiceName {
			serviceName = keyAttr.Value
		}
	}

	for _, attr := range p.transform.Attributes {
		if attr.Key == entityattributes.ServiceNameSource {
			source = attr.Value
		}
	}

	return serviceName, source
}
