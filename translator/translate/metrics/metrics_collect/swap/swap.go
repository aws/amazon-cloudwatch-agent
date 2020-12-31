// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package swap

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/util"
)

var ChildRule = map[string]translator.Rule{}

const SectionKey_Swap = "swap"

func GetCurPath() string {
	curPath := parent.GetCurPath() + SectionKey_Swap + "/"
	return curPath
}

func RegisterRule(fieldname string, r translator.Rule) {
	ChildRule[fieldname] = r
}

type Swap struct {
}

func (s *Swap) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})
	result := map[string]interface{}{}
	res := []interface{}{}
	//Check if this plugin exist in the input instance
	//If not, not process
	if _, ok := m[SectionKey_Swap]; !ok {
		returnKey = ""
		returnVal = ""
	} else {

		/*
		  In JSON config file, it represent as "swap" : {//specification config information}
		  To check the specification config entry
		*/
		//Check if there are some config entry with rules applied
		result = translator.ProcessRuleToApply(m[SectionKey_Swap], ChildRule, result)

		//Process common config, like measurement
		hasValidMetric := util.ProcessLinuxCommonConfig(m[SectionKey_Swap], SectionKey_Swap, GetCurPath(), result)
		if hasValidMetric {
			res = append(res, result)
			returnKey = SectionKey_Swap
			returnVal = res
		} else {
			returnKey = ""
		}
	}
	return
}

func init() {
	s := new(Swap)
	parent.RegisterLinuxRule(SectionKey_Swap, s)
	parent.RegisterDarwinRule(SectionKey_Swap, s)
}
