// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonRule"
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonUtil"
)

var ChildRule = map[string]translator.Rule{}

const (
	SectionKey = "agent"
	Mode       = "mode"
)

func GetCurPath() string {
	return "/" + SectionKey + "/"
}

func RegisterRule(fieldname string, r translator.Rule) {
	ChildRule[fieldname] = r
}

type Agent struct {
	Interval    string
	Credentials map[string]interface{}
	Region      string
	RegionType  string
	Mode        string
	Internal    bool
	Role_arn    string
}

var Global_Config Agent = *new(Agent)

func (a *Agent) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})
	result := map[string]interface{}{}
	/*
	  In JSON config file, it represent as "agent" : {//specification config information}
	  To check the specification config entry
	*/
	//For agent configuration, specification configuration should only override default config.
	var agentMap interface{}
	if tempMap, ok := m[SectionKey]; ok {
		agentMap = tempMap
	} else {
		agentMap = map[string]interface{}{}
	}
	result = translator.ProcessRuleToApply(agentMap, ChildRule, result)

	returnKey = SectionKey
	returnVal = result
	return
}

var MergeRuleMap = map[string]mergeJsonRule.MergeRule{}

func (a *Agent) Merge(source map[string]interface{}, result map[string]interface{}) {
	mergeJsonUtil.MergeMap(source, result, SectionKey, MergeRuleMap, GetCurPath())
}

func init() {
	// Not registering agent translator rule, we handle it differently in translate.go

	// Register merge json rule
	obj := new(Agent)
	mergeJsonUtil.MergeRuleMap[SectionKey] = obj
}
