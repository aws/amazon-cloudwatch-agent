// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logs

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonRule"
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonUtil"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/util"
)

type Rule translator.Rule

var ChildRule = map[string]Rule{}

const (
	SectionKey             = "logs"
	Output_Cloudwatch_Logs = "cloudwatchlogs"
)

func GetCurPath() string {
	curPath := parent.GetCurPath() + SectionKey + "/"
	return curPath
}

func RegisterRule(fieldname string, r Rule) {
	ChildRule[fieldname] = r
}

type Logs struct {
	FileStateFolder string
	MetadataInfo    map[string]string
}

var GlobalLogConfig = Logs{}

func (l *Logs) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	im := input.(map[string]interface{})
	result := map[string]interface{}{}
	inputs := map[string]interface{}{}
	processors := map[string]interface{}{}
	cloudwatchConfig := map[string]interface{}{}
	GlobalLogConfig.MetadataInfo = util.GetMetadataInfo(util.Ec2MetadataInfoProvider)

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
				if key == "metrics_collected" {
					if metricsResult, ok := val.(map[string]map[string]interface{}); ok {
						if metricsInputs, ok := metricsResult["inputs"]; ok {
							for k, v := range metricsInputs {
								inputs[k] = v
							}
						}
						if metricsProcessors, ok := metricsResult["processors"]; ok {
							for k, v := range metricsProcessors {
								processors[k] = v
							}
						}

					}
				} else if key == "inputs" {
					// inputs here are coming from logs_collected
					inputs = translator.MergeTwoUniqueMaps(inputs, val.(map[string]interface{}))
				} else if key == Output_Cloudwatch_Logs {
					cloudwatchConfig = translator.MergeTwoUniqueMaps(cloudwatchConfig, val.(map[string]interface{}))
				}
			}
		}

		cloudwatchInfo := map[string]interface{}{}
		cloudwatchInfo["cloudwatchlogs"] = []interface{}{cloudwatchConfig}
		result["outputs"] = cloudwatchInfo

		if len(inputs) > 0 {
			result["inputs"] = inputs
		}
		if len(processors) > 0 {
			result["processors"] = processors
		}

		returnKey = SectionKey
		returnVal = result
	}
	return
}

var MergeRuleMap = map[string]mergeJsonRule.MergeRule{}

func (l *Logs) Merge(source map[string]interface{}, result map[string]interface{}) {
	mergeJsonUtil.MergeMap(source, result, SectionKey, MergeRuleMap, GetCurPath())
}

func init() {
	l := new(Logs)
	parent.RegisterLinuxRule(SectionKey, l)
	parent.RegisterDarwinRule(SectionKey, l)
	parent.RegisterWindowsRule(SectionKey, l)
	mergeJsonUtil.MergeRuleMap[SectionKey] = l
}
