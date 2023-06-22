// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package procstat

type Exe struct{}

const ExeKey = "exe"

func (t *Exe) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})
	if _, ok := m[ExeKey]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		returnKey = ExeKey
		returnVal = m[ExeKey]
	}
	return
}

func init() {
	e := new(Exe)

	RegisterRule(ExeKey, e)
}
