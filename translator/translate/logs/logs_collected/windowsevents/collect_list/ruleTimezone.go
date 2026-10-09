// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collectlist

const TimezoneSectionKey = "timezone"

type Timezone struct {
}

func (t *Timezone) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})
	if val, ok := m[TimezoneSectionKey]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		returnKey = TimezoneSectionKey
		if val == "UTC" {
			returnVal = "UTC"
		} else {
			returnVal = "LOCAL"
		}
	}
	return
}

func init() {
	r := new(Timezone)
	RegisterRule(TimezoneSectionKey, r)
}
