// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package statsd

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect"
)

//
// Need to import new rule package in src/translator/tocwconfig/totomlconfig/toTomlConfig.go
//

// SectionKey
//
//	"statsd" : {
//	    "service_address": ":8125",
//	    "metrics_collection_interval": 10,
//	    "metrics_aggregation_interval": 60
//	}
const SectionKey = "statsd"

var ChildRule = map[string]translator.Rule{}

func GetCurPath() string {
	curPath := parent.GetCurPath() + SectionKey + "/"
	return curPath
}

func RegisterRule(fieldname string, r translator.Rule) {
	ChildRule[fieldname] = r
}

type StatsD struct {
}

func (obj *StatsD) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})
	resArray := []interface{}{}
	result := map[string]interface{}{}
	//Check if this plugin exist in the input instance
	//If not, not process
	if _, ok := m[SectionKey]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		//If exists, process it
		//Check if there are some config entry with rules applied
		result = translator.ProcessRuleToApply(m[SectionKey], ChildRule, result)
		resArray = append(resArray, result)
		returnKey = SectionKey
		returnVal = resArray
	}
	return
}

func init() {
	obj := new(StatsD)
	parent.RegisterLinuxRule(SectionKey, obj)
	parent.RegisterDarwinRule(SectionKey, obj)
	parent.RegisterWindowsRule(SectionKey, obj)
}
