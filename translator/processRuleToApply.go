// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package translator

// ProcessRuleToApply check if the specification configuration provide some configs that need translation with some rules
// In json the default configuration should be like "cpu":{"per_cpu":false}
func ProcessRuleToApply(input interface{}, childRule map[string]Rule, result map[string]interface{}) map[string]interface{} {
	for _, rule := range childRule {
		key, val := rule.ApplyRule(input)
		if key != "" {
			result[key] = val
		}
	}
	return result
}
