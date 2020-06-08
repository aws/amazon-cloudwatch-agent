package k8sapiserver

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected/kubernetes"
)

type Rule translator.Rule

var ChildRule = map[string]Rule{}

const (
	SubSectionKey = "k8sapiserver"
)

func GetCurPath() string {
	curPath := parent.GetCurPath() + SubSectionKey + "/"
	return curPath
}

func RegisterRule(fieldname string, r Rule) {
	ChildRule[fieldname] = r
}

type ApiServer struct {
}

func (a *ApiServer) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	im := input.(map[string]interface{})
	result := map[string]interface{}{}
	for _, rule := range ChildRule {
		key, val := rule.ApplyRule(im)
		if key != "" {
			result[key] = val
		}
	}
	returnKey = SubSectionKey
	returnVal = result
	return
}

func init() {
	a := new(ApiServer)
	parent.RegisterRule(SubSectionKey, a)
}
