// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package translator

// Don't modify map1 or map2, it's hard to make sure we don't reference map1 or map2 reference in whole proj
// This method is mainly to merge maps which don't have same key, it's a shallow merge. Fail this method if not serving this purpose.
func MergeTwoUniqueMaps(map1 map[string]interface{}, map2 map[string]interface{}) map[string]interface{} {
	mergeResult := make(map[string]interface{})
	errorMessage := "Fail to merge two map, since both of them use same key"

	for k, v := range map1 {
		if _, ok := mergeResult[k]; ok {
			panic(errorMessage)
		} else {
			mergeResult[k] = v
		}
	}

	for k, v := range map2 {
		if _, ok := mergeResult[k]; ok {
			panic(errorMessage)
		} else {
			mergeResult[k] = v
		}
	}

	return mergeResult
}

// Don't modify map1 or map2, it's hard to make sure we don't reference map1 or map2 reference in whole proj
// This method is mainly to merge different plugins or multiple instances of same plugins
func MergePlugins(map1 map[string]interface{}, map2 map[string]interface{}) map[string]interface{} {
	mergeResult := make(map[string]interface{})

	for k, v := range map1 {
		mergeResult[k] = v
	}

	for k, v := range map2 {
		if _, ok := mergeResult[k]; ok {
			// merge instances into the array if they belong to same plugin
			instances := []interface{}{}
			instances = append(instances, mergeResult[k].([]interface{})...)
			instances = append(instances, v.([]interface{})...)
			mergeResult[k] = instances
		} else {
			mergeResult[k] = v
		}
	}
	return mergeResult
}
