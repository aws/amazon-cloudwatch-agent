package metrics

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type TagExclude struct {
}

func (t *TagExclude) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	res := map[string]interface{}{}
	m := input.(map[string]interface{})
	if _, ok := m["append_dimensions"]; ok {
		key, val := translator.DefaultCase("tagexclude", "host", input)
		res[key] = []string{val.(string)}
	}
	returnKey = "outputs"
	returnVal = res
	return
}

func init() {
	t := new(TagExclude)
	RegisterRule("tagexclude", t)
}
