// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cadvisor

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
	parent "github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/logs/metrics_collected/ecs"
)

type Rule translator.Rule

var ChildRule = map[string]Rule{}

const (
	SubSectionKey = "cadvisor"
)

func GetCurPath() string {
	curPath := parent.GetCurPath() + SubSectionKey + "/"
	return curPath
}

func RegisterRule(fieldname string, r Rule) {
	ChildRule[fieldname] = r
}

type Cadvisor struct {
}

func (c *Cadvisor) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	im := input.(map[string]interface{})
	result := map[string]interface{}{}
	for _, rule := range ChildRule {
		key, val := rule.ApplyRule(im)
		if key != "" {
			result[key] = val
		}
	}
	returnKey = SubSectionKey
	returnVal = result
	return
}

func init() {
	c := new(Cadvisor)
	parent.RegisterRule(SubSectionKey, c)
}
