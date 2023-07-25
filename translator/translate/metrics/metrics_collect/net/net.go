// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package net

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/util"
)

var ChildRule = map[string]translator.Rule{}

const SectionKey_Net_Linux = "net"
const SectionKey_Net_Windows = "Network Interface"

func GetCurPath() string {
	curPath := parent.GetCurPath() + SectionKey_Net_Linux + "/"
	return curPath
}

func RegisterRule(fieldname string, r translator.Rule) {
	ChildRule[fieldname] = r
}

type Net struct {
}

func (n *Net) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})
	resArray := []interface{}{}
	result := map[string]interface{}{}
	//Generate the config file for monitoring system metrics on linux
	//Check if this plugin exist in the input instance
	//If not, not process
	if _, ok := m[SectionKey_Net_Linux]; !ok {
		returnKey = ""
		returnVal = ""
	} else {

		/*
		  In JSON config file, it represents as "cpu" : {//specification config information}
		  To check the specification config entry
		*/
		//Check if there are some config entry with rules applied
		result = translator.ProcessRuleToApply(m[SectionKey_Net_Linux], ChildRule, result)

		//Process common config, like measurement
		hasValidMetric := util.ProcessLinuxCommonConfig(m[SectionKey_Net_Linux], SectionKey_Net_Linux, GetCurPath(), result)
		if hasValidMetric {
			resArray = append(resArray, result)
			returnKey = SectionKey_Net_Linux
			returnVal = resArray
		} else {
			returnKey = ""
		}
	}
	return
}

func init() {
	n := new(Net)
	parent.RegisterLinuxRule(SectionKey_Net_Linux, n)
	parent.RegisterDarwinRule(SectionKey_Net_Linux, n)
}
