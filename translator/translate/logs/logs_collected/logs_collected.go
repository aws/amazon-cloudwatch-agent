// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logs_collected

import (
	"log"

	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonRule"
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonUtil"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs"
)

type Rule translator.Rule
type LogsCollected struct {
}

var ChildRule = map[string]Rule{}

var (
	windowsMetricCollectRule = map[string]Rule{}
	linuxMetricCollectRule   = map[string]Rule{}
	darwinMetricCollectRule  = map[string]Rule{}
)

const SectionKey = "logs_collected"

func GetCurPath() string {
	curPath := parent.GetCurPath() + SectionKey + "/"
	return curPath
}

func RegisterLinuxRule(ruleName string, r Rule) {
	linuxMetricCollectRule[ruleName] = r
}

func RegisterDarwinRule(ruleName string, r Rule) {
	darwinMetricCollectRule[ruleName] = r
}

func RegisterWindowsRule(ruleName string, r Rule) {
	windowsMetricCollectRule[ruleName] = r
}

func (l *LogsCollected) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	im := input.(map[string]interface{})
	var targetRuleMap map[string]Rule
	result := map[string]interface{}{}

	switch translator.GetTargetPlatform() {
	case config.OS_TYPE_LINUX:
		targetRuleMap = linuxMetricCollectRule
	case config.OS_TYPE_DARWIN:
		targetRuleMap = darwinMetricCollectRule
	case config.OS_TYPE_WINDOWS:
		targetRuleMap = windowsMetricCollectRule
	default:
		log.Panicf("E! Unknown target platform: %s ", translator.GetTargetPlatform())
	}

	if _, ok := im[SectionKey]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		for _, rule := range targetRuleMap {
			key, val := rule.ApplyRule(im[SectionKey])
			if key == "inputs" {
				result = translator.MergeTwoUniqueMaps(result, val.(map[string]interface{}))
			}
		}
		returnKey = "inputs"
		returnVal = result
	}
	return
}

var MergeRuleMap = map[string]mergeJsonRule.MergeRule{}

func (l *LogsCollected) Merge(source map[string]interface{}, result map[string]interface{}) {
	mergeJsonUtil.MergeMap(source, result, SectionKey, MergeRuleMap, GetCurPath())
}

func init() {
	obj := new(LogsCollected)
	parent.RegisterRule(SectionKey, obj)
	parent.MergeRuleMap[SectionKey] = obj
}
