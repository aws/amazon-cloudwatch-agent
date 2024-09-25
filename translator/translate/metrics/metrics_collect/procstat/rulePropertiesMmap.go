// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package procstat

type MMap struct{}

const MMapKey = "properties"

func (mm *MMap) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})

	if measurementArray, exists := m["measurement"]; !exists {
		returnKey = ""
		returnVal = ""
	} else {
		for _, val := range measurementArray.([]interface{}) {
			if val.(string) == "memory_swap" {
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
