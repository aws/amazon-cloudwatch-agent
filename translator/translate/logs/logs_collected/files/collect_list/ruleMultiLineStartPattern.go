// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collect_list

type MultiLineStartPattern struct {
}

func (m *MultiLineStartPattern) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	im := input.(map[string]interface{})
	if val, ok := im["multi_line_start_pattern"]; ok {
		returnKey = "multi_line_start_pattern"
		if val == "{timestamp_format}" {
			returnVal = "{timestamp_regex}"
		} else {
			returnVal = val
		}
	} else {
		returnKey = ""
		returnVal = ""
	}
	return
}

func init() {
	m := new(MultiLineStartPattern)
	r := []Rule{m}
	RegisterRule("multi_line_start_pattern", r)
}
