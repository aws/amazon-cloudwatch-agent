// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agentInternal

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/util"
)

var ChildRule = map[string]translator.Rule{}

const SectionKey_Internal = "internal"

func GetCurPath() string {
	curPath := parent.GetCurPath() + SectionKey_Internal + "/"
	return curPath
}

func RegisterRule(fieldname string, r translator.Rule) {
	ChildRule[fieldname] = r
}

type Internal struct {
}

func (i *Internal) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})
	res := []interface{}{}
	result := map[string]interface{}{}
	//Check if this plugin exist in the input instance
	//If not, not process
	if _, ok := m[SectionKey_Internal]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		/*
		  In JSON config file, it represents as "cpu" : {//specification config information}
		  To check the specification config entry
		*/
		//Check if there are some config entry with rules applied
		result = translator.ProcessRuleToApply(m[SectionKey_Internal], ChildRule, result)

		//Process common config, like measurement
		hasValidMetric := util.ProcessLinuxCommonConfig(m[SectionKey_Internal], SectionKey_Internal, GetCurPath(), result)
		if hasValidMetric {
			res = append(res, result)
			returnKey = SectionKey_Internal
			returnVal = res
		} else {
			returnKey = ""
		}
	}

	return
}

func init() {
	i := new(Internal)
	parent.RegisterLinuxRule(SectionKey_Internal, i)
}
