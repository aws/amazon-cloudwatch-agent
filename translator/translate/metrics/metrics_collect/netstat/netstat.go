// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package netstat

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/util"
)

var ChildRule = map[string]translator.Rule{}

const SectionKey_Netstat = "netstat"

func GetCurPath() string {
	curPath := parent.GetCurPath() + SectionKey_Netstat + "/"
	return curPath
}

func RegisterRule(fieldname string, r translator.Rule) {
	ChildRule[fieldname] = r
}

type NetStat struct {
}

func (n *NetStat) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})
	//Generate the config file for monitoring system metrics on non-windows
	res := []interface{}{}
	result := map[string]interface{}{}
	//Check if this plugin exist in the input instance
	//If not, not process
	if _, ok := m[SectionKey_Netstat]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		/*
		  In JSON config file, it represents as "cpu" : {//specification config information}
		  To check the specification config entry
		*/
		//Check if there are some config entry with rules applied
		result = translator.ProcessRuleToApply(m[SectionKey_Netstat], ChildRule, result)

		//Process common config, like measurement
		hasValidMetric := util.ProcessLinuxCommonConfig(m[SectionKey_Netstat], SectionKey_Netstat, GetCurPath(), result)
		if hasValidMetric {
			res = append(res, result)
			returnKey = SectionKey_Netstat
			returnVal = res
		} else {
			returnKey = ""
		}
	}
	return
}

func init() {
	n := new(NetStat)
	parent.RegisterLinuxRule(SectionKey_Netstat, n)
}
