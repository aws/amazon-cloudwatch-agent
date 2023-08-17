// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package csm

import (
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonRule"
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonUtil"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate"
)

const JSONSectionKey = "csm"

func GetCurPath() string {
	curPath := parent.GetCurPath() + JSONSectionKey + "/"
	return curPath
}

type Csm struct {
}

func (c *Csm) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey = ""
	returnVal = ""
	return
}

var MergeRuleMap = map[string]mergeJsonRule.MergeRule{}

func (c *Csm) Merge(source map[string]interface{}, result map[string]interface{}) {
	mergeJsonUtil.MergeMap(source, result, JSONSectionKey, MergeRuleMap, GetCurPath())
}

func init() {
	c := new(Csm)
	parent.RegisterLinuxRule(JSONSectionKey, c)
	parent.RegisterDarwinRule(JSONSectionKey, c)
	parent.RegisterWindowsRule(JSONSectionKey, c)
	mergeJsonUtil.MergeRuleMap[JSONSectionKey] = c
}
