// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metrics_collected

import (
	"sort"

	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonRule"
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonUtil"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs"
)

type Rule translator.Rule

var ChildRule = map[string]Rule{}

var (
	windowsMetricCollectRule = map[string]Rule{}
	linuxMetricCollectRule   = map[string]Rule{}
	darwinMetricCollectRule  = map[string]Rule{}
)

const SectionKey = "metrics_collected"

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

type CollectMetrics struct {
}

func (c *CollectMetrics) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	im := input.(map[string]interface{})
	result := map[string]map[string]interface{}{}
	inputs := map[string]interface{}{}
	processors := map[string]interface{}{}

	var targetRuleMap map[string]Rule
	switch translator.GetTargetPlatform() {
	case config.OS_TYPE_LINUX:
		targetRuleMap = linuxMetricCollectRule
	case config.OS_TYPE_DARWIN:
		targetRuleMap = darwinMetricCollectRule
	case config.OS_TYPE_WINDOWS:
		targetRuleMap = windowsMetricCollectRule
	default:
		// NOTE: we should panic but now there are many unit tests not setting the global var
		targetRuleMap = linuxMetricCollectRule
		//panic("unknown target platform " + translator.GetTargetPlatform())
	}

	//Check if this plugin exist in the input instance
	//If not, not process
	if _, ok := im[SectionKey]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		//If yes, process it

		featureInited := false // kubernetes, ecs, prometheus are mutually exclusive, use this flag to ensure only one feature could be turned on.
		for _, rule := range getOrderedRules(targetRuleMap) {
			key, val := rule.ApplyRule(im[SectionKey])
			if key == "kubernetes" || key == "ecs" || key == "prometheus" {
				if featureInited {
					translator.AddErrorMessages(GetCurPath(), "Feature kubernetes, ecs, prometheus are mutually exclusive")
					return
				} else {
					featureInited = true
				}
				if result, ok := val.(map[string]map[string]interface{}); ok {
					if tmpInputs, ok := result["inputs"]; ok {
						for k, v := range tmpInputs {
							inputs[k] = v
						}
					}
					if tmpProcessors, ok := result["processors"]; ok {
						for k, v := range tmpProcessors {
							processors[k] = v
						}
					}
				}
			} else {
				if key != "" {
					inputs[key] = val
				}
			}
		}
	}
	result["inputs"] = inputs
	result["processors"] = processors
	returnKey = SectionKey
	returnVal = result
	return
}

// Adding alphabet order to the Rules
func getOrderedRules(ruleMap map[string]Rule) []Rule {
	var orderedRules []Rule
	var orderedRuleNames []string
	for ruleName := range ruleMap {
		orderedRuleNames = append(orderedRuleNames, ruleName)
	}
	sort.Strings(orderedRuleNames)
	for _, orderedRuleName := range orderedRuleNames {
		orderedRules = append(orderedRules, ruleMap[orderedRuleName])
	}
	return orderedRules
}

var MergeRuleMap = map[string]mergeJsonRule.MergeRule{}

func (c *CollectMetrics) Merge(source map[string]interface{}, result map[string]interface{}) {
	mergeJsonUtil.MergeMap(source, result, SectionKey, MergeRuleMap, GetCurPath())
}

func init() {
	c := new(CollectMetrics)
	parent.RegisterRule(SectionKey, c)
	parent.MergeRuleMap[SectionKey] = c
}
