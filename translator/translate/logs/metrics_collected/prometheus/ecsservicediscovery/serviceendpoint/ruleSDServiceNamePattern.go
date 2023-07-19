// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package serviceendpoint

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

const (
	SectionKeySDServiceNamePattern = "sd_service_name_pattern"
)

type SDServiceNamePattern struct {
}

// Mandatory Key
func (d *SDServiceNamePattern) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	im := input.(map[string]interface{})
	if val, ok := im[SectionKeySDServiceNamePattern]; !ok {
		returnKey = ""
		returnVal = ""
		translator.AddErrorMessages(GetCurPath()+SectionKeySDServiceNamePattern, "sd_service_name_pattern is not defined.")
	} else {
		returnKey = SectionKeySDServiceNamePattern
		returnVal = val
	}
	return
}

func init() {
	RegisterRule(SectionKeySDServiceNamePattern, new(SDServiceNamePattern))
}
