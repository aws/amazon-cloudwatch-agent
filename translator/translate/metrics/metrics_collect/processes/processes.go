// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package processes

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/util"
)

var ChildRule = map[string]translator.Rule{}

const SectionKey_Processes_Linux = "processes"
const SectionKey_Processes_Windows = "Processor"

func GetCurPath() string {
	curPath := parent.GetCurPath() + SectionKey_Processes_Linux + "/"
	return curPath
}

func RegisterRule(fieldname string, r translator.Rule) {
	ChildRule[fieldname] = r
}

type Processes struct {
}

func (p *Processes) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})
	resArray := []interface{}{}
	result := map[string]interface{}{}
	//Generate the config file for monitoring system metrics on linux
	//Check if this plugin exist in the input instance
	//If not, not process
	if _, ok := m[SectionKey_Processes_Linux]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		/*
		  In JSON config file, it represents as "cpu" : {//specification config information}
		  To check the specification config entry
		*/
		//Check if there are some config entry with rules applied
		result = translator.ProcessRuleToApply(m[SectionKey_Processes_Linux], ChildRule, result)

		//Process common config, like measurement
		hasValidMetric := util.ProcessLinuxCommonConfig(m[SectionKey_Processes_Linux], SectionKey_Processes_Linux, GetCurPath(), result)
		if hasValidMetric {
			resArray = append(resArray, result)
			returnKey = SectionKey_Processes_Linux
			returnVal = resArray
		} else {
			returnKey = ""
		}

	}
	return
}

func init() {
	p := new(Processes)
	parent.RegisterLinuxRule(SectionKey_Processes_Linux, p)
	parent.RegisterDarwinRule(SectionKey_Processes_Linux, p)
}
