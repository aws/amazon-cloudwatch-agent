// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package procstat

type MMap struct{}

const MMapKey = "properties"

func (mm *MMap) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})

	if measurementArray, exists := m["measurement"]; exists {
		for _, val := range measurementArray.([]interface{}) {
			if strVal, ok := val.(string); ok && strVal == "memory_swap" {
				returnKey = MMapKey
				returnVal = []string{"mmap"}
			}
		}
	}

	return
}

func init() {
	mm := new(MMap)
	RegisterRule(MMapKey, mm)
}
