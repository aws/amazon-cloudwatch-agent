// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ethtool

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect"
)

var ChildRule = map[string]translator.Rule{}

//	"ethtool" : {
//	    "interface_include": "*",
//	    "interface_exclude": "",
//	    "metrics_include": [
//	        "bw_in_allowance_exceeded",
//	        "bw_out_allowance_exceeded"
//	    ]
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
	m := input.(map[string]interface{})
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
	}
	return
}

func init() {
	n := new(Ethtool)
	parent.RegisterLinuxRule(SectionKey_Ethtool, n)
}
