// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package mem

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/util"
)

var ChildRule = map[string]translator.Rule{}

const SectionKey_Mem_Linux = "mem"
const SectionKey_Mem_Windows = "Memory"

func GetCurPath() string {
	curPath := parent.GetCurPath() + SectionKey_Mem_Linux + "/"
	return curPath
}

func RegisterRule(fieldname string, r translator.Rule) {
	ChildRule[fieldname] = r
}

type Mem struct {
}

func (m *Mem) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	im := input.(map[string]interface{})
	resArray := []interface{}{}
	result := map[string]interface{}{}
	//Check if this plugin exist in the input instance
	//If not, not process
	if _, ok := im[SectionKey_Mem_Linux]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		/*
		  In JSON config file, it represent as "mem" : {//specification config information}
		  To check the specification config entry
		*/
		//Check if there are some config entry with rules applied
		result = translator.ProcessRuleToApply(im[SectionKey_Mem_Linux], ChildRule, result)

		//Process common config, like measurement
		hasValidMetric := util.ProcessLinuxCommonConfig(im[SectionKey_Mem_Linux], SectionKey_Mem_Linux, GetCurPath(), result)
		if hasValidMetric {
			resArray = append(resArray, result)
			returnKey = SectionKey_Mem_Linux
			returnVal = resArray
		} else {
			returnKey = ""
		}
	}
	return
}

func init() {
	m := new(Mem)
	parent.RegisterLinuxRule(SectionKey_Mem_Linux, m)
}
