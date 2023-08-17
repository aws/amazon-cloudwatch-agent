// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package rollup_dimensions

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics"
)

type rollupDimensions struct {
}

const SectionKey = "aggregation_dimensions"

var ChildRule = map[string]translator.Rule{}

func GetCurPath() string {
	curPath := parent.GetCurPath() + SectionKey + "/"
	return curPath
}

func (ad *rollupDimensions) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	im := input.(map[string]interface{})

	result := map[string]interface{}{}

	if _, ok := im[SectionKey]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		returnKey = metrics.OutputsKey
		if !IsValidRollupList(im[SectionKey]) {
			returnKey = ""
		}
		result["rollup_dimensions"] = im[SectionKey]
		returnVal = result
	}
	return
}

func init() {
	rd := new(rollupDimensions)
	parent.RegisterRule(SectionKey, rd)
}

// Strict type check for [][]string
func IsValidRollupList(input interface{}) bool {
	if inputList, ok := input.([]interface{}); ok {
		if len(inputList) == 0 {
			return fail()
		}
		for _, item := range inputList {
			if elementList, ok := item.([]interface{}); ok {
				if len(elementList) != 0 {
					for _, element := range elementList {
						if _, ok := element.(string); !ok {
							return fail()
						}
					}
				}
			} else {
				return fail()
			}
		}
	} else {
		return fail()
	}
	return true
}

func fail() bool {
	translator.AddErrorMessages(GetCurPath(), "Invalid format, Expected Value is [][]string, e.g. [[\"ImageId\"], [\"InstanceId\", \"InstanceType\"],[]]")
	return false
}
