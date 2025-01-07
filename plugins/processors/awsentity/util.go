// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsentity

import "go.opentelemetry.io/collector/pdata/pcommon"

func AddAttributeIfNonEmpty(p pcommon.Map, key string, value string) {
	if value != "" {
		p.PutStr(key, value)
	}
}
