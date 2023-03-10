// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package procstat

type Pattern struct{}

const PatternKey = "pattern"

func (p *Pattern) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})
	if _, ok := m[PatternKey]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		returnKey = PatternKey
		returnVal = m[PatternKey]
	}
	return
}

func init() {
	p := new(Pattern)
	RegisterRule(PatternKey, p)
}
