package metrics

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
	SectionKey = "metrics"
	OutputsKey = "outputs"
)

func GetCurPath() string {
	curPath := parent.GetCurPath() + SectionKey + "/"
	return curPath
}

func RegisterRule(fieldname string, r Rule) {
	ChildRule[fieldname] = r
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
		translator.AddInfoMessages("", "No metric configuration found.")
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
				} else if key == "metric_decoration" {
					addDecorations(key, val, outputPlugInfo)
				} else {
					result[key] = val
				}
			}
		}

		cloudwatchInfo := map[string]interface{}{}
		cloudwatchInfo["cloudwatch"] = []interface{}{outputPlugInfo}
		result["outputs"] = cloudwatchInfo
		translator.SetMetricPath(result, SectionKey)
		returnKey = SectionKey
		returnVal = result
	}
	return
}

func addDecorations(key string, val interface{}, outputPlugInfo map[string]interface{}) {
	if len(val.([]interface{})) > 0 {
		outputPlugInfo[key] = val
	}
}

var MergeRuleMap = map[string]mergeJsonRule.MergeRule{}

func (m *Metrics) Merge(source map[string]interface{}, result map[string]interface{}) {
	mergeJsonUtil.MergeMap(source, result, SectionKey, MergeRuleMap, GetCurPath())
}

func init() {
	m := new(Metrics)
	parent.RegisterLinuxRule(SectionKey, m)
	parent.RegisterWindowsRule(SectionKey, m)
	ChildRule["globalcredentials"] = util.GetCredsRule(OutputsKey)
	ChildRule["region"] = util.GetRegionRule(OutputsKey)

	mergeJsonUtil.MergeRuleMap[SectionKey] = m
}
