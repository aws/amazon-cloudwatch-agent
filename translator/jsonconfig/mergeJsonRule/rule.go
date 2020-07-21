// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package mergeJsonRule

type MergeRule interface {
	Merge(source map[string]interface{}, result map[string]interface{})
}
