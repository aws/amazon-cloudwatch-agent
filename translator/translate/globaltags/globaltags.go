package globaltags

import (
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate"
)

const SectionKey = "global_tags"

func GetCurPath() string {
	curPath := parent.GetCurPath() + SectionKey + "/"
	return curPath
}

type GlobalTags struct {
}

func (g *GlobalTags) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})
	result := map[string]interface{}{}
	//Check if user specifies global_tags
	if _, ok := m[SectionKey]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		for k, v := range m[SectionKey].(map[string]interface{}) {
			result[k] = v
		}
		returnKey = SectionKey
		returnVal = result
	}
	return
}

func init() {
	g := new(GlobalTags)
	parent.RegisterLinuxRule(SectionKey, g)
	parent.RegisterWindowsRule(SectionKey, g)
}
