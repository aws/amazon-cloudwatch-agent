// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsdecorator

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected/ecs"
)

type Rule translator.Rule

var ChildRule = map[string]Rule{}

const (
	SubSectionKey = "ecsdecorator"
)

func GetCurPath() string {
	curPath := parent.GetCurPath() + SubSectionKey + "/"
	return curPath
}

func RegisterRule(fieldname string, r Rule) {
	ChildRule[fieldname] = r
}

type ECSDecorator struct {
}

func (e *ECSDecorator) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	im := input.(map[string]interface{})
	result := map[string]interface{}{"order": 1}
	returnKey = SubSectionKey
	for _, rule := range ChildRule {
		key, val := rule.ApplyRule(im)
		if key != "" {
			result[key] = val
		}
	}
	returnVal = result
	return
}

func init() {
	k := new(ECSDecorator)
	parent.RegisterRule(SubSectionKey, k)
}
