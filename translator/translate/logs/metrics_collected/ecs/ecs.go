// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecs

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/jsonconfig/mergeJsonRule"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/jsonconfig/mergeJsonUtil"
	parent "github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/logs/metrics_collected"
)

type ECS struct{}
type Rule translator.Rule

var ChildRule = map[string]Rule{}

const (
	SectionKey = "ecs"
)

func GetCurPath() string {
	curPath := parent.GetCurPath() + SectionKey + "/"
	return curPath
}
func RegisterRule(fieldname string, r Rule) {
	ChildRule[fieldname] = r
}

func (e *ECS) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey = ""
	returnVal = ""
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
}
