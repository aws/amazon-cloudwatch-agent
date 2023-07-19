// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package diskio

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/util"
)

var ChildRule = map[string]translator.Rule{}

const SectionKey_DiskIO_Linux = "diskio"

func GetCurPath() string {
	curPath := parent.GetCurPath() + SectionKey_DiskIO_Linux + "/"
	return curPath
}

func RegisterRule(fieldname string, r translator.Rule) {
	ChildRule[fieldname] = r
}

type DiskIO struct {
}

func (d *DiskIO) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})
	resArray := []interface{}{}
	result := map[string]interface{}{}
	//Generate the config file for monitoring system metrics on linux
	//Check if this plugin exist in the input instance
	//If not, not process
	if _, ok := m[SectionKey_DiskIO_Linux]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		//If exists, process it
		/*
		  In JSON config file, it represents as "cpu" : {//specification config information}
		  To check the specification config entry
		*/
		//Check if there are some config entry with rules applied
		result = translator.ProcessRuleToApply(m[SectionKey_DiskIO_Linux], ChildRule, result)

		//Process common config, like measurement
		hasValidMetric := util.ProcessLinuxCommonConfig(m[SectionKey_DiskIO_Linux], SectionKey_DiskIO_Linux, GetCurPath(), result)
		if hasValidMetric {
			resArray = append(resArray, result)
			returnKey = SectionKey_DiskIO_Linux
			returnVal = resArray
		} else {
			returnKey = ""
		}
	}
	return
}

func init() {
	d := new(DiskIO)
	parent.RegisterLinuxRule(SectionKey_DiskIO_Linux, d)
	parent.RegisterDarwinRule(SectionKey_DiskIO_Linux, d)
}
