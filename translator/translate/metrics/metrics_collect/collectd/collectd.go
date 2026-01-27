// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collected

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/util"
)

//
// Need to import new rule package in src/translator/tocwconfig/totomlconfig/toTomlConfig.go
//

//	"collectd" : {
//	    "service_address": "udp://127.0.0.1:25826",
//	    "name_prefix": "collectd_",
//	    "collectd_auth_file": "/etc/collectd/auth_file",
//	    "collectd_security_level": "encrypt",
//	    "collectd_typesdb": ["/usr/share/collectd/types.db"],
//	    "metrics_aggregation_interval": 60
//	}
const (
	SectionKey       = "collectd"
	SectionMappedKey = "socket_listener"
)

var ChildRule = map[string]translator.Rule{}

func GetCurPath() string {
	curPath := parent.GetCurPath() + SectionKey + "/"
	return curPath
}

func RegisterRule(fieldname string, r translator.Rule) {
	ChildRule[fieldname] = r
}

type CollectD struct {
}

func (obj *CollectD) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})
	resArray := []interface{}{}
	result := map[string]interface{}{}
	//Check if this plugin exist in the input instance
	//If not, not process
	if _, ok := m[SectionKey]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		util.ProcessAppendDimensions(m[SectionKey].(map[string]interface{}), SectionKey, result)
		result = translator.ProcessRuleToMergeAndApply(m[SectionKey], ChildRule, result)
		resArray = append(resArray, result)
		returnKey = SectionMappedKey
		returnVal = resArray
	}
	return
}

func init() {
	obj := new(CollectD)
	parent.RegisterLinuxRule(SectionKey, obj)
	parent.RegisterDarwinRule(SectionKey, obj)
}
