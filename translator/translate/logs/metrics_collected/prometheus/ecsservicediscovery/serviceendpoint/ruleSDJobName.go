// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package serviceendpoint

const (
	SectionKeySDJobName = "sd_job_name"
)

type SDJobName struct {
}

// Optional Key
func (d *SDJobName) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	im := input.(map[string]interface{})
	if val, ok := im[SectionKeySDJobName]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		returnKey = SectionKeySDJobName
		returnVal = val
	}
	return
}

func init() {
	RegisterRule(SectionKeySDJobName, new(SDJobName))
}
