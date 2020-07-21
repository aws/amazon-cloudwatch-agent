// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package procstat

type Pattern struct{}

const keyPattern = "pattern"

func (p *Pattern) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})
	if _, ok := m[keyPattern]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		returnKey = keyPattern
		returnVal = m[keyPattern]
	}
	return
}

func init() {
	p := new(Pattern)
	RegisterRule(keyPattern, p)
}
