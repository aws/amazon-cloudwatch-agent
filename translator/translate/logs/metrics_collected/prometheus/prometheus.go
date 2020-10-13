package emfprocessor

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonRule"
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonUtil"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected"
)

const SectionKey = "prometheus"

type Rule translator.Rule

var ChildRule = map[string]Rule{}

type Prometheus struct {
}

func GetCurPath() string {
	curPath := parent.GetCurPath() + SectionKey + "/"
	return curPath
}

func RegisterRule(fieldname string, r Rule) {
	ChildRule[fieldname] = r
}

func (p *Prometheus) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	im := input.(map[string]interface{})
	result := map[string]map[string]interface{}{}
	inputs := map[string]interface{}{}
	processors := map[string]interface{}{}
	promScaper := map[string]interface{}{}

	//Check if this plugin exist in the input instance
	//If not, not process
	if _, ok := im[SectionKey]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		for _, rule := range ChildRule {
			key, val := rule.ApplyRule(im[SectionKey])
			if key == "emf_processor" {
				processors["emfProcessor"] = []interface{}{val}
			} else if key == SectionKeyLogGroupName {
				if v, ok := promScaper["tags"]; ok {
					m := v.(map[string]interface{})
					m[key] = val
				} else {
					promScaper["tags"] = map[string]interface{}{key: val}
				}
			} else if key != "" {
				promScaper[key] = val
			}
		}

		inputs["prometheus_scraper"] = []interface{}{promScaper}

		result["inputs"] = inputs
		result["processors"] = processors

		returnKey = SectionKey
		returnVal = result
	}
	return
}

var MergeRuleMap = map[string]mergeJsonRule.MergeRule{}

func (p *Prometheus) Merge(source map[string]interface{}, result map[string]interface{}) {
	mergeJsonUtil.MergeMap(source, result, SectionKey, MergeRuleMap, GetCurPath())
}

func init() {
	k := new(Prometheus)
	parent.MergeRuleMap[SectionKey] = k
	parent.RegisterLinuxRule(SectionKey, k)
}
