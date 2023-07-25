// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package translator

import (
	"reflect"
)

// ProcessNoRuleToApply check if the the specification configuration provides some configs that don't need translation with rules.
// In this case, the translation of this config entry should be 1:1 map
// In json the default configuration should be like "cpu":{"interval":"10s"}
func ProcessNoRuleToApply(input interface{}, childRule map[string]Rule, result map[string]interface{}) map[string]interface{} {
	for k, v := range input.(map[string]interface{}) {
		if _, ok := childRule[k]; !ok {
			if reflect.TypeOf(v).String() == "map[string]interface {}" {
				temp := map[string]interface{}{}
				for key, val := range v.(map[string]interface{}) {
					if key != "" {
						temp[key] = val
					}
				}
				result[k] = temp
			} else {
				result[k] = v
			}
		}
	}
	return result
}
