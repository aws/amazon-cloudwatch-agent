// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package procstat

import (
	"github.com/aws/amazon-cloudwatch-agent/internal/util/hash"
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonRule"
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonUtil"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/util"
)

var ChildRule = map[string]translator.Rule{}

const SectionKey = "procstat"

func GetCurPath() string {
	curPath := parent.GetCurPath() + SectionKey + "/"
	return curPath
}

func RegisterRule(fieldname string, r translator.Rule) {
	ChildRule[fieldname] = r
}

type Procstat struct {
}

func (p *Procstat) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	im := input.(map[string]interface{})
	//Check if this plugin exist in the input instance
	//If not, not process
	returnKey = ""
	returnVal = ""
	if _, ok := im[SectionKey]; !ok {
		return
	}

	resArray := []interface{}{}
	configArray := im[SectionKey].([]interface{})
	for _, processConfig := range configArray {
		result := map[string]interface{}{}
		// common config
		if !util.ProcessLinuxCommonConfig(processConfig, SectionKey, GetCurPath(), result) {
			return
		}

		for _, rule := range ChildRule {
			if key, val := rule.ApplyRule(processConfig); key != "" {
				result[key] = val
			}
		}
		/*  Generate a alias name for each procstat monitored process  since every monitored process  plugin will generate
		a duplicate plugin but with different configuration. Moreover, we want to order by PidFile, ExeKey, Pattern Key
		according to the public documents if multiple configuration is specified
		https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-Agent-procstat-process-metrics.html#CloudWatch-Agent-procstat-configuration
		*/
		for _, procstatMonitored := range []string{PatternKey, ExeKey, PidFileKey} {
			for _, rule := range ChildRule {
				if key, val := rule.ApplyRule(processConfig); key != "" && key == procstatMonitored {
					result[util.Alias_Key] = hash.HashName(val.(string))
					break
				}
			}

		}
		resArray = append(resArray, result)
	}

	returnKey = SectionKey
	returnVal = resArray
	return
}

var MergeRuleMap = map[string]mergeJsonRule.MergeRule{}

func (c *Procstat) Merge(source map[string]interface{}, result map[string]interface{}) {
	mergeJsonUtil.MergeList(source, result, SectionKey)
}

func init() {
	m := new(Procstat)
	parent.RegisterLinuxRule(SectionKey, m)
	parent.RegisterDarwinRule(SectionKey, m)
	parent.RegisterWindowsRule(SectionKey, m)
	parent.MergeRuleMap[SectionKey] = m
}
