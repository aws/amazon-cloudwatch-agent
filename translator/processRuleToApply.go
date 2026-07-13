// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package translator

// ProcessRuleToApply check if the specification configuration provide some configs that need translation with some rules
// In json the default configuration should be like "cpu":{"per_cpu":false}
func ProcessRuleToApply(input interface{}, childRule map[string]Rule, result map[string]interface{}) map[string]interface{} {
	// Process use_dualstack_endpoint rule first if it exists to ensure dualstack endpoint is set
	if rule, exists := childRule["use_dualstack_endpoint"]; exists {
		key, val := rule.ApplyRule(input)
		if key != "" {
			result[key] = val
		}
	}

	for ruleKey, rule := range childRule {
		// Skip use_dualstack_endpoint as it has already been set
		if ruleKey == "use_dualstack_endpoint" {
			continue
		}
		key, val := rule.ApplyRule(input)
		if key != "" {
			result[key] = val
		}
	}
	return result
}
