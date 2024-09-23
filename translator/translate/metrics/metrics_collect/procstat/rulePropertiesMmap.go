// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package procstat

type MMap struct{}

const MMapKey = "properties"

func (mm *MMap) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})
	if _, ok := m[MMapKey]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		returnKey = MMapKey
		returnVal = m[MMapKey]
	}
	return
}

func init() {
	mm := new(MMap)
	RegisterRule(MMapKey, mm)
}
