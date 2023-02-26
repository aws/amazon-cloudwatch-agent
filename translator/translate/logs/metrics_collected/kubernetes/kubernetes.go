// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package kubernetes

import (
	"fmt"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"

	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonRule"
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonUtil"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected"
)

const SectionKey = "kubernetes"

type Rule translator.Rule

var ChildRule = map[string]Rule{}

type Kubernetes struct {
}

func GetCurPath() string {
	curPath := parent.GetCurPath() + SectionKey + "/"
	return curPath
}

func RegisterRule(fieldname string, r Rule) {
	ChildRule[fieldname] = r
}

func (k *Kubernetes) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	im := input.(map[string]interface{})
	result := map[string]map[string]interface{}{}
	inputs := map[string]interface{}{}
	processors := map[string]interface{}{}

	//Check if this plugin exist in the input instance
	//If not, not process
	if _, ok := im[SectionKey]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		//If yes, process it
		if !context.CurrentContext().RunInContainer() {
			translator.AddErrorMessages(GetCurPath(), fmt.Sprintf("kubernetes is configured in a non-containerized environment"))
			return
		}
		for _, rule := range ChildRule {
			key, val := rule.ApplyRule(im[SectionKey])
			if key == "cadvisor" || key == "k8sapiserver" {
				inputs[key] = []interface{}{val}
			} else if key == "k8sdecorator" {
				processors[key] = []interface{}{val}
			} else if key == "ec2tagger" {
				// Only enable ec2tagger if in ec2 mode
				if context.CurrentContext().Mode() == config.ModeEC2 {
					processors[key] = []interface{}{val}
				}
			} else if key != "" {
				translator.AddErrorMessages(GetCurPath(), fmt.Sprintf("Find unexpected key %s", key))
				return
			}
		}

		result["inputs"] = inputs
		result["processors"] = processors

		returnKey = SectionKey
		returnVal = result
	}
	return
}

var MergeRuleMap = map[string]mergeJsonRule.MergeRule{}

func (k *Kubernetes) Merge(source map[string]interface{}, result map[string]interface{}) {
	mergeJsonUtil.MergeMap(source, result, SectionKey, MergeRuleMap, GetCurPath())
}

func init() {
	k := new(Kubernetes)
	parent.MergeRuleMap[SectionKey] = k
	parent.RegisterLinuxRule(SectionKey, k)
	parent.RegisterDarwinRule(SectionKey, k)
}
