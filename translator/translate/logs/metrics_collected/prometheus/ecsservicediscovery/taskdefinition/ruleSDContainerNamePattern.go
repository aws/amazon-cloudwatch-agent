// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package taskdefinition

const (
	SectionKeySDContainerNamePattern = "sd_container_name_pattern"
)

type SDContainerNamePattern struct {
}

// Optional Key
func (d *SDContainerNamePattern) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {

	im := input.(map[string]interface{})
	if val, ok := im[SectionKeySDContainerNamePattern]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		returnKey = SectionKeySDContainerNamePattern
		returnVal = val
	}
	return
}

func init() {
	RegisterRule(SectionKeySDContainerNamePattern, new(SDContainerNamePattern))
}
