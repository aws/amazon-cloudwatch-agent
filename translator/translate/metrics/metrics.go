// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metrics

import (
	"sort"

	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonRule"
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonUtil"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/util"
)

type Rule translator.Rule

var ChildRule = map[string]Rule{}

const (
	SectionKey = "metrics"
	OutputsKey = "outputs"
)

func GetCurPath() string {
	curPath := parent.GetCurPath() + SectionKey + "/"
	return curPath
}

func RegisterRule(fieldName string, r Rule) {
	ChildRule[fieldName] = r
}

type Metrics struct {
}

func (m *Metrics) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	im := input.(map[string]interface{})
	result := map[string]interface{}{}
	outputPlugInfo := map[string]interface{}{}

	//Check if this plugin exist in the input instance
	//If not, not process
	if _, ok := im[SectionKey]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		//If yes, process it
		for _, rule := range ChildRule {
			key, val := rule.ApplyRule(im[SectionKey])
			//If key == "", then no instance of this class in input
			if key != "" {
				if key == OutputsKey {
					outputPlugInfo = translator.MergeTwoUniqueMaps(outputPlugInfo, val.(map[string]interface{}))
				} else if config.ContainsKey(key) {
					addCloudWatchOutputConfig(key, val, outputPlugInfo)
				} else {
					result[key] = val
				}
			}
		}

		cloudwatchInfo := map[string]interface{}{}
		cloudwatchInfo["cloudwatch"] = []interface{}{map[string]interface{}{}}
		result["outputs"] = cloudwatchInfo
		returnKey = SectionKey
		returnVal = result
	}
	return
}

func addCloudWatchOutputConfig(key string, val interface{}, outputPlugInfo map[string]interface{}) {
	if val1, isSlice := val.([]interface{}); isSlice && len(val1) > 0 {
		outputPlugInfo[key] = val
	} else if val2, isMap := val.(map[string][]string); isMap && len(val2) > 0 {
		sortItems(val2)
		outputPlugInfo[key] = val2
	}

}

var MergeRuleMap = map[string]mergeJsonRule.MergeRule{}

func (m *Metrics) Merge(source map[string]interface{}, result map[string]interface{}) {
	mergeJsonUtil.MergeMap(source, result, SectionKey, MergeRuleMap, GetCurPath())
}

// Sort items in map alphabetically to temporarily avoid unstable tests before introduce Toml-to-Toml comparison.
func sortItems(vals map[string][]string) {
	for _, val := range vals {
		sort.Strings(val)
	}
}

func init() {
	m := new(Metrics)
	parent.RegisterLinuxRule(SectionKey, m)
	parent.RegisterDarwinRule(SectionKey, m)
	parent.RegisterWindowsRule(SectionKey, m)
	ChildRule["globalcredentials"] = util.GetCredsRule(OutputsKey)
	ChildRule["region"] = util.GetRegionRule(OutputsKey)

	mergeJsonUtil.MergeRuleMap[SectionKey] = m
}
