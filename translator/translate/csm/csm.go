// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package csm

import (
	"github.com/aws/amazon-cloudwatch-agent/internal/csm"
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonRule"
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonUtil"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/util"
)

const (
	ConfInputPluginKey       = "awscsm_listener"
	ConfAggregationPluginKey = "aws_csm"
	ConfOutputPluginKey      = "aws_csm"
	ConfInputAddressKey      = "service_addresses"

	OutputsKey = "outputs"
)

var ChildRule = map[string]translator.Rule{}

func GetCurPath() string {
	curPath := parent.GetCurPath() + csm.JSONSectionKey + "/"
	return curPath
}

func RegisterRule(fieldname string, r translator.Rule) {
	ChildRule[fieldname] = r
}

type Csm struct {
}

func (c *Csm) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})
	result := map[string]interface{}{}
	inputConfig := map[string]interface{}{}
	outputConfig := map[string]interface{}{}

	// Check if this plugin exists in the input instance
	// If not, don't process
	if csmSection, ok := m[csm.JSONSectionKey]; !ok {
		returnKey = ""
		returnVal = ""
		translator.AddInfoMessages("", "No csm configuration found.")
	} else {
		for _, rule := range ChildRule {
			key, val := rule.ApplyRule(csmSection)
			if key == ConfOutputPluginKey {
				outputConfig = translator.MergeTwoUniqueMaps(outputConfig, val.(map[string]interface{}))
			} else if key == ConfInputPluginKey {
				inputConfig = translator.MergeTwoUniqueMaps(inputConfig, val.(map[string]interface{}))
			}
		}

		if len(inputConfig) == 0 {
			returnKey = ""
			returnVal = ""
			translator.AddInfoMessages("", "Unable to generate client-side monitoring listener configuration")
			return
		}

		csmInput := map[string]interface{}{}
		csmInput[ConfInputPluginKey] = []interface{}{inputConfig}
		result["inputs"] = csmInput

		outputConfig[agent.RegionKey] = agent.Global_Config.Region
		csmOutput := map[string]interface{}{}
		csmOutput[ConfOutputPluginKey] = []interface{}{outputConfig}
		result[OutputsKey] = csmOutput

		returnKey = csm.JSONSectionKey
		returnVal = result

	}
	return
}

var MergeRuleMap = map[string]mergeJsonRule.MergeRule{}

func (c *Csm) Merge(source map[string]interface{}, result map[string]interface{}) {
	mergeJsonUtil.MergeMap(source, result, csm.JSONSectionKey, MergeRuleMap, GetCurPath())
}

func init() {
	c := new(Csm)
	parent.RegisterLinuxRule(csm.JSONSectionKey, c)
	parent.RegisterWindowsRule(csm.JSONSectionKey, c)
	ChildRule["csmCreds"] = util.GetCredsRule(ConfOutputPluginKey)
	mergeJsonUtil.MergeRuleMap[csm.JSONSectionKey] = c
}
