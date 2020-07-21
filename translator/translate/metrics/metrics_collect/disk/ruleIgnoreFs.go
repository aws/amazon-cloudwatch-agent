// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package disk

type IgnoreFs struct {
}

const ignoreFS = "ignore_fs"
const ignoreJsonKey = "ignore_file_system_types"

// This is an optional field, if not declared, this field is omitted.
func (i *IgnoreFs) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})
	if _, ok := m[ignoreJsonKey]; !ok {
		// no default set for ignore FS
		returnKey = ""
		returnVal = ""
		return
	} else {
		returnKey = ignoreFS
		returnVal = m[ignoreJsonKey]
	}
	return
}

func init() {
	i := new(IgnoreFs)
	RegisterRule("ignore_fs", i)
}
