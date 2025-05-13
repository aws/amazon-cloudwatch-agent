// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsentity

import "go.opentelemetry.io/collector/pdata/pcommon"

func (p *awsEntityProcessor) PutAttribute(resourceAttributes pcommon.Map, k string, v string) {
	attributeAllowList := p.config.AttributeAllowList
	for _, allowedAttribute := range attributeAllowList {
		if k == allowedAttribute {
			resourceAttributes.PutStr(k, v)
		}
	}
}

func (p *awsEntityProcessor) AddAttributeIfNonEmpty(resourceAttributes pcommon.Map, key string, value string) {
	if value != "" {
		p.PutAttribute(resourceAttributes, key, value)
	}
}
