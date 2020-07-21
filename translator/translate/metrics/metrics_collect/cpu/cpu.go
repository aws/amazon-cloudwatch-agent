// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cpu

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/util"
)

var CPU_ChildRule = map[string]translator.Rule{}

const SectionKey_CPU = "cpu"

func GetCurPath() string {
	curPath := parent.GetCurPath() + SectionKey_CPU + "/"
	return curPath
}

func RegisterRule(fieldname string, r translator.Rule) {
	CPU_ChildRule[fieldname] = r
}

type Cpu struct {
}

func (c *Cpu) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})

	//Generate the config file for monitoring system metrics on linux
	res := []interface{}{}
	result := map[string]interface{}{}
	//Check if this plugin exist in the input instance
	//If not, not process
	if _, ok := m[SectionKey_CPU]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		/*
		  In JSON config file, it represents as "cpu" : {//specification config information}
		  To check the specification config entry
		*/
		//Check if there are some config entry with rules applied
		result = translator.ProcessRuleToApply(m[SectionKey_CPU], CPU_ChildRule, result)

		//Process common config, like measurement
		hasValidMetric := util.ProcessLinuxCommonConfig(m[SectionKey_CPU], SectionKey_CPU, GetCurPath(), result)
		if hasValidMetric {
			res = append(res, result)
			returnKey = SectionKey_CPU
			returnVal = res
		} else {
			returnKey = ""
		}
	}
	return
}

func init() {
	c := new(Cpu)
	parent.RegisterLinuxRule(SectionKey_CPU, c)
}
