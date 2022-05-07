// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecs

import (
	"fmt"

	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonRule"
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonUtil"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected"
)

type ECS struct{}
type Rule translator.Rule

var ChildRule = map[string]Rule{}

const (
	SectionKey             = "ecs"
	SectionKeyCadvisor     = "cadvisor"
	SectionKeyECSDecorator = "ecsdecorator"
	SectionKeyEC2Tagger    = "ec2tagger"
)

func GetCurPath() string {
	curPath := parent.GetCurPath() + SectionKey + "/"
	return curPath
}
func RegisterRule(fieldname string, r Rule) {
	ChildRule[fieldname] = r
}

func (e *ECS) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	im := input.(map[string]interface{})
	result := map[string]map[string]interface{}{}
	inputs := map[string]interface{}{}
	processors := map[string]interface{}{}

	if _, ok := im[SectionKey]; !ok {
		returnKey = ""
		returnVal = ""
		return
	}

	if !context.CurrentContext().RunInContainer() {
		translator.AddErrorMessages(GetCurPath(), fmt.Sprintf("ecs is configured in a non-containerized environment"))
		return
	}
	for _, rule := range ChildRule {
		key, val := rule.ApplyRule(im[SectionKey])

		if key == SectionKeyCadvisor {
			inputs[key] = []interface{}{val}
		} else if key == SectionKeyEC2Tagger || key == SectionKeyECSDecorator {
			processors[key] = []interface{}{val}
		} else if key != "" {
			translator.AddErrorMessages(GetCurPath(), fmt.Sprintf("Find unexpected key %s", key))
			return
		}
	}

	result["inputs"] = inputs
	result["processors"] = processors

	returnKey = SectionKey
	returnVal = result
	return
}

var MergeRuleMap = map[string]mergeJsonRule.MergeRule{}

func (e *ECS) Merge(source map[string]interface{}, result map[string]interface{}) {
	mergeJsonUtil.MergeMap(source, result, SectionKey, MergeRuleMap, GetCurPath())
}

func init() {
	e := new(ECS)
	parent.MergeRuleMap[SectionKey] = e
	parent.RegisterLinuxRule(SectionKey, e)
	parent.RegisterDarwinRule(SectionKey, e)
}
