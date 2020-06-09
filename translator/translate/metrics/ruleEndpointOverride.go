package metrics

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type EndpointOverride struct {
}

func (r *EndpointOverride) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	res := map[string]interface{}{}
	key, val := translator.DefaultCase("endpoint_override", "", input)
	res[key] = val
	if val != "" {
		returnKey = "outputs"
		returnVal = res
	}
	return
}

func init() {
	r := new(EndpointOverride)
	RegisterRule("endpoint_override", r)
}
