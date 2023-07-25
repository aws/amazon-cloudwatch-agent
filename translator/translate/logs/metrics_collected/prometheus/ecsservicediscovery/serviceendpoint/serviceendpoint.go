// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package serviceendpoint

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected/prometheus/ecsservicediscovery"
)

type Rule translator.Rule

var ChildRule = map[string]Rule{}

const (
	SubSectionKey = "service_name_list_for_tasks"
)

func GetCurPath() string {
	curPath := parent.GetCurPath() + SubSectionKey + "/"
	return curPath
}

func RegisterRule(fieldname string, r Rule) {
	ChildRule[fieldname] = r
}

type ServiceEndpoint struct {
}

func (e *ServiceEndpoint) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	im := input.(map[string]interface{})
	returnKey = SubSectionKey

	if _, ok := im[SubSectionKey]; !ok {
		returnKey = ""
		returnVal = ""
		return
	}

	configArr := im[SubSectionKey].([]interface{})
	res := []interface{}{}
	for i := 0; i < len(configArr); i++ {
		result := map[string]interface{}{}
		for _, ruleArr := range ChildRule {
			key, val := ruleArr.ApplyRule(configArr[i])
			if key != "" {
				result[key] = val
			}
		}
		res = append(res, result)
	}

	returnKey = SubSectionKey
	returnVal = res

	return
}

func init() {
	e := new(ServiceEndpoint)
	parent.RegisterRule(SubSectionKey, e)
}
