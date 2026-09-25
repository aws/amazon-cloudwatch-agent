// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package win_services

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/util"
)

// SectionKey is the JSON key under metrics_collected for Windows service status metrics.
//
//	"win_services": {
//	    "service_names": ["AmazonSSMAgent", "Spooler", "Win*"],
//	    "excluded_service_names": ["WinRM"],
//	    "measurement": ["state", "startup_mode"],
//	    "metrics_collection_interval": 60
//	}
const SectionKey = "win_services"

var ChildRule = map[string]translator.Rule{}

func GetCurPath() string {
	curPath := parent.GetCurPath() + SectionKey + "/"
	return curPath
}

func RegisterRule(fieldname string, r translator.Rule) {
	ChildRule[fieldname] = r
}

type WinServices struct {
}

func (obj *WinServices) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})
	resArray := []interface{}{}
	result := map[string]interface{}{}
	if _, ok := m[SectionKey]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		result = translator.ProcessRuleToApply(m[SectionKey], ChildRule, result)
		util.ProcessAppendDimensions(m[SectionKey].(map[string]interface{}), SectionKey, result)
		resArray = append(resArray, result)
		returnKey = SectionKey
		returnVal = resArray
	}
	return
}

func init() {
	obj := new(WinServices)
	parent.RegisterWindowsRule(SectionKey, obj)
}
