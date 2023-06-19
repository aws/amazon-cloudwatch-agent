// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package accumulator

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
)

// Otel Attributes = Telegraf Tags = CloudWatch Dimensions
func addTagsToAttributes(attributes pcommon.Map, tags map[string]string) {
	for tag, value := range tags {
		attributes.PutStr(tag, value)
	}
}
