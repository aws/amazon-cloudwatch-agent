// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package translate

import (
	"fmt"
	"log"
	"sort"

	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
)

type Rule translator.Rule

func GetCurPath() string {
	curPath := "/"
	return curPath
}

var (
	linuxTranslateRule   = map[string]Rule{}
	darwinTranslateRule  = map[string]Rule{}
	windowsTranslateRule = map[string]Rule{}
)

func RegisterLinuxRule(fieldname string, r Rule) {
	linuxTranslateRule[fieldname] = r
}

func RegisterDarwinRule(fieldname string, r Rule) {
	darwinTranslateRule[fieldname] = r
}

func RegisterWindowsRule(fieldname string, r Rule) {
	windowsTranslateRule[fieldname] = r
}

type Translator struct {
}

func (t *Translator) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})
	result := map[string]interface{}{}
	allInputPlugin := map[string]interface{}{}
	allOutputPlugin := map[string]interface{}{}
	var allProcessorPlugin map[string]interface{}
	var allAggregatorPlugin map[string]interface{}

	var targetRuleMap map[string]Rule
	switch translator.GetTargetPlatform() {
	case config.OS_TYPE_LINUX:
		targetRuleMap = linuxTranslateRule
	case config.OS_TYPE_DARWIN:
		targetRuleMap = darwinTranslateRule
	case config.OS_TYPE_WINDOWS:
		targetRuleMap = windowsTranslateRule
	default:
		log.Panicf("E! Unknown target platform %s", translator.GetTargetPlatform())
	}

	fmt.Println("_+_+_+_+__+_+_+_+__+_+_+_+__+_+_+_+__+_+_+_+__+_+_+_+__+_+_+_+__+_+_+_+__+_+_+_+__+_+_+_+_+_+__+_+_+_+__+_+_+_+__+_+_+_+_")
	fmt.Println("Below is the input before key val")
	fmt.Println(input)
	//We need to apply agent rule first, since global setting lies there, which will impact the override logic
	key, val := agent.Global_Config.ApplyRule(input)
	result[key] = val

	// sort rule here so that we could get the output plugin instance in a stable order
	sortedRuleKey := make([]string, 0, len(targetRuleMap))
	for k := range targetRuleMap {
		sortedRuleKey = append(sortedRuleKey, k)
	}
	sort.Strings(sortedRuleKey)
	for _, key = range sortedRuleKey {
		rule := targetRuleMap[key]
		key, val = rule.ApplyRule(m)
		fmt.Printf("This is the key %v and this is the value: %v\n", key, val)
		//Only output the result that the class instance is processed
		//If it is not processed, it key will return ""
		if key != "" {
			if key == "agent" || key == "global_tags" {
				result[key] = val
			} else {
				valMap := val.(map[string]interface{})
				fmt.Println("This is the valMap Inputs", valMap["inputs"])
				fmt.Println("This is the valMap outputs", valMap["outputs"])
				fmt.Println("This is the valMap processors", valMap["processors"])
				fmt.Println("This is the valMap aggregators", valMap["aggregators"])

				if inputs, ok := valMap["inputs"]; ok {
					allInputPlugin = translator.MergePlugins(allInputPlugin, inputs.(map[string]interface{}))
					fmt.Println("This is the allinput plugin: ", allInputPlugin)
				}
				if outputs, ok := valMap["outputs"]; ok {
					allOutputPlugin = translator.MergePlugins(allOutputPlugin, outputs.(map[string]interface{}))
					fmt.Println("This is the all output plugin: ", allOutputPlugin)

				}
				if processors, ok := valMap["processors"]; ok {
					allProcessorPlugin = translator.MergePlugins(allProcessorPlugin, processors.(map[string]interface{}))
					fmt.Println("This is the all output plugin: ", allProcessorPlugin)

				}
				if aggregators, ok := valMap["aggregators"]; ok {
					allAggregatorPlugin = translator.MergeTwoUniqueMaps(allAggregatorPlugin, aggregators.(map[string]interface{}))
					fmt.Println("This is the all output plugin: ", allAggregatorPlugin)
				}
			}
		}
	}
	if len(allInputPlugin) != 0 {
		result["inputs"] = allInputPlugin
	}
	if len(allOutputPlugin) != 0 {
		result["outputs"] = allOutputPlugin
	}
	if allProcessorPlugin != nil {
		result["processors"] = allProcessorPlugin
	}
	if allAggregatorPlugin != nil {
		result["aggregators"] = allAggregatorPlugin
	}
	returnKey = "root"
	returnVal = result
	return
}
