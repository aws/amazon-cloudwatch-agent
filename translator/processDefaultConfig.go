// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package translator

// ProcessDefaultConfig process the input when user want to use the default configuration.
// In json the default configuration should be like "cpu":"default"
func ProcessDefaultConfig(childRule map[string]Rule, result map[string]interface{}) map[string]interface{} {
	defaultTemp := map[string]interface{}{}
	for _, rule := range childRule {
		key, val := rule.ApplyRule(defaultTemp)
		if key != "" {
			result[key] = val
		}
	}
	return result
}
