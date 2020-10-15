// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package taskdefinition

import (
	"fmt"
	"regexp"

	"github.com/aws/amazon-cloudwatch-agent/translator"
)

const (
	SectionKeySDMetricsPorts = "sd_metrics_ports"
	expectedRegex            = "^[1-9][0-9]{0,4}(;[\\s]*[1-9][0-9]{0,4})*$"
)

type SDMetricsPorts struct {
}

// Mandatory Key
func (d *SDMetricsPorts) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	im := input.(map[string]interface{})
	if val, ok := im[SectionKeySDMetricsPorts]; !ok {
		returnKey = ""
		returnVal = ""
		translator.AddErrorMessages(GetCurPath()+SectionKeySDMetricsPorts, "mandatory key: sd_metrics_ports is not defined.")
	} else {
		if !checkMetricPortString(val.(string)) {
			translator.AddErrorMessages(GetCurPath()+SectionKeySDMetricsPorts, fmt.Sprintf("sd_metrics_ports does not follow pattern: %v.", expectedRegex))
		}
		returnKey = SectionKeySDMetricsPorts
		returnVal = val
	}
	return
}

func checkMetricPortString(portsConfig string) bool {
	ret, err := regexp.MatchString(expectedRegex, portsConfig)
	if err != nil || !ret {
		return false
	}
	return true
}

func init() {
	RegisterRule(SectionKeySDMetricsPorts, new(SDMetricsPorts))
}
