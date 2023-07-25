// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package procstat

type PidFile struct{}

const PidFileKey = "pid_file"

func (p *PidFile) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})
	if _, ok := m[PidFileKey]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		returnKey = PidFileKey
		returnVal = m[PidFileKey]
	}
	return
}

func init() {
	p := new(PidFile)
	RegisterRule(PidFileKey, p)
}
