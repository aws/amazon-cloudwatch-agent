// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

// We are doing one level search here
func SetWithSameKeyIfFound(input interface{}, target_key_list []string, result map[string]interface{}) {
	inputmap := input.(map[string]interface{})
	for _, key := range target_key_list {
		if val, ok := inputmap[key]; ok {
			result[key] = val
		}
	}
}

// We are doing one level search here
func SetWithCustomizedKeyIfFound(input interface{}, targetKeyMap map[string]string, result map[string]interface{}) {
	inputmap := input.(map[string]interface{})
	for key := range targetKeyMap {
		if val, ok := inputmap[key]; ok {
			result[targetKeyMap[key]] = val
		}
	}
}
