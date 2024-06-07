// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ethtool

import (
	"fmt"
	"github.com/aws/amazon-cloudwatch-agent/translator"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/util"
)

var ChildRule = map[string]translator.Rule{}

//	"ethtool" : {
//	    "interface_include": "*",
//	    "interface_exclude": "",
//	    "metrics_include": [
//	        "bw_in_allowance_exceeded",
//	        "bw_out_allowance_exceeded"
//	    ]
//		"append_dimensions":{
//			key:value
//		}
//	}
const SectionKey_Ethtool = "ethtool"

func GetCurPath() string {
	curPath := parent.GetCurPath() + SectionKey_Ethtool + "/"
	return curPath
}

func RegisterRule(fieldname string, r translator.Rule) {
	ChildRule[fieldname] = r
}

type Ethtool struct {
}

func (n *Ethtool) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	fmt.Println("_+_+_+_+__+_+_+_+__+_+_+_+__+_+_+_+__+_+_+_+__+_+_+_+__+_+_+_+__+_+_+_+__+_+_+_+__+_+_+_+_+_+__+_+_+_+__+_+_+_+__+_+_+_+_")
	fmt.Println("In Ethtool, this is the input before key val")
	fmt.Println(input)
	m := input.(map[string]interface{})
	fmt.Println("------ Below is the map -------")
	fmt.Println(m)
	res := []interface{}{}
	//Generate the config file for monitoring system metrics on non-windows
	resArr := []interface{}{}
	result := map[string]interface{}{}
	//Check if this plugin exist in the input instance
	//If not, not process
	if _, ok := m[SectionKey_Ethtool]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		//If exists, process it
		//Check if there are some config entry with rules applied
		result = translator.ProcessRuleToApply(m[SectionKey_Ethtool], ChildRule, result)
		resArr = append(resArr, result)
		returnKey = SectionKey_Ethtool
		returnVal = resArr

		//Process tags
		hasValidMetric := util.ProcessLinuxCommonConfig(m[SectionKey_Ethtool], SectionKey_Ethtool, GetCurPath(), result)
		if hasValidMetric {
			res = append(res, result)
			returnKey = SectionKey_Ethtool
			returnVal = res
		} else {
			returnKey = ""
		}
	}
	return
}

func init() {
	n := new(Ethtool)
	parent.RegisterLinuxRule(SectionKey_Ethtool, n)
}
