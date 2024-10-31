// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package translator

import (
	"golang.org/x/exp/maps"
)

func ProcessRuleToMergeAndApply(input interface{}, childRule map[string]Rule, result map[string]interface{}) map[string]interface{} {
	for _, rule := range childRule {
		key, val := rule.ApplyRule(input)
		if _, ok := result[key]; ok {
			maps.Copy(result[key].(map[string]interface{}), val.(map[string]interface{}))
		} else if key != "" {
			result[key] = val
		}
	}
	return result
}
