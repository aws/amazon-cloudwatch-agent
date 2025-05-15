// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsentity

import (
	"slices"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

func (p *awsEntityProcessor) PutAttribute(resourceAttributes pcommon.Map, k string, v string) {
	if len(p.config.AttributeAllowList) == 0 || slices.Contains(p.config.AttributeAllowList, k) {
		resourceAttributes.PutStr(k, v)
	}
}

func (p *awsEntityProcessor) AddAttributeIfNonEmpty(resourceAttributes pcommon.Map, key string, value string) {
	if value != "" {
		p.PutAttribute(resourceAttributes, key, value)
	}
}
