package emfprocessor

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected/prometheus"
)

type Rule translator.Rule

var ChildRule = map[string]Rule{}

const (
	SubSectionKey = "emf_processor"
)

func GetCurPath() string {
	curPath := parent.GetCurPath() + SubSectionKey + "/"
	return curPath
}

func RegisterRule(fieldname string, r Rule) {
	ChildRule[fieldname] = r
}

type EMFProcessor struct {
}

func (e *EMFProcessor) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	im := input.(map[string]interface{})
	// add a arbitary order 10 for the EMF processor as the EMF processor should be used as the last processor before output plugins.
	result := map[string]interface{}{"order": 10}
	returnKey = SubSectionKey

	if _, ok := im[SubSectionKey]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		for _, rule := range ChildRule {
			key, val := rule.ApplyRule(im[SubSectionKey])
			result[key] = val
		}

		returnKey = SubSectionKey
		returnVal = result
	}
	return
}

func init() {
	e := new(EMFProcessor)
	parent.RegisterRule(SubSectionKey, e)
}
