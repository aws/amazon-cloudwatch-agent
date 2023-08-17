// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonRule"
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonUtil"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/logs_collected"
)

var ChildRule = map[string]translator.Rule{}

type Files struct {
}

const (
	SectionKey       = "files"
	SectionMappedKey = "logfile"
)

func GetCurPath() string {
	return parent.GetCurPath() + SectionKey + "/"
}

func RegisterRule(ruleName string, r translator.Rule) {
	ChildRule[ruleName] = r
}

func (f *Files) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	im := input.(map[string]interface{})
	tailConfig := map[string]interface{}{}
	if _, ok := im[SectionKey]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		for _, rule := range ChildRule {
			key, val := rule.ApplyRule(im[SectionKey])
			if key == "fixedTailConfig" {
				tailConfig = translator.MergeTwoUniqueMaps(tailConfig, val.(map[string]interface{}))
			} else if key != "" {
				tailConfig[key] = val
			}

		}

		// generate tail config only if file_config exists
		tailInfo := map[string]interface{}{}
		if _, ok = tailConfig["file_config"]; ok {
			tailInfo[SectionMappedKey] = []interface{}{tailConfig}
			returnKey = "inputs"
			returnVal = tailInfo
		}
	}
	return
}

var MergeRuleMap = map[string]mergeJsonRule.MergeRule{}

func (f *Files) Merge(source map[string]interface{}, result map[string]interface{}) {
	mergeJsonUtil.MergeMap(source, result, SectionKey, MergeRuleMap, GetCurPath())
}

func init() {
	f := new(Files)
	parent.RegisterLinuxRule(SectionKey, f)
	parent.RegisterDarwinRule(SectionKey, f)
	parent.RegisterWindowsRule(SectionKey, f)
	parent.MergeRuleMap[SectionKey] = f
}
