// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package gpu

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/util"
)

var ChildRule = map[string]translator.Rule{}

//
//	"nvidia_gpu": {
//		"measurement": [
//			"utilization_gpu",
//			"temperature_gpu"
//		],
//      "metrics_collection_interval": 60
//	}
//

// SectionKey metrics name in user config to opt in Nvidia GPU metrics
const (
	SectionKey       = "nvidia_gpu"
	SectionMappedKey = "nvidia_smi"
)

func GetCurPath() string {
	curPath := parent.GetCurPath() + SectionKey + "/"
	return curPath
}

func RegisterRule(fieldname string, r translator.Rule) {
	ChildRule[fieldname] = r
}

type NvidiaSmi struct {
}

func (n *NvidiaSmi) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})
	resArr := []interface{}{}
	result := map[string]interface{}{}
	//Check if this plugin exist in the input instance
	//If not, not process
	if _, ok := m[SectionKey]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		/*
		   In JSON config file, it represent as "nvidia_gpu" : {//specification config information}
		   To check the specification config entry
		*/
		//Check if there are any config entry with rules applied
		result = translator.ProcessRuleToApply(m[SectionKey], ChildRule, result)
		//Process common config, like measurement
		hasValidMetric := util.ProcessLinuxCommonConfig(m[SectionKey], SectionMappedKey, GetCurPath(), result)
		if hasValidMetric {
			resArr = append(resArr, result)
			returnKey = SectionMappedKey
			returnVal = resArr
		} else {
			returnKey = ""
		}
	}
	return
}

func init() {
	n := new(NvidiaSmi)
	parent.RegisterLinuxRule(SectionKey, n)
	parent.RegisterDarwinRule(SectionKey, n)
	//parent.RegisterWindowsRule(SectionKey, n)
}
