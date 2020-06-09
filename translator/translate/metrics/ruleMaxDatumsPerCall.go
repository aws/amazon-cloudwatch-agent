package metrics

import "github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"

type MaxDatumsPerCall struct {
}

func (obj *MaxDatumsPerCall) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	if agent.Global_Config.Internal {
		res := map[string]interface{}{"max_datums_per_call": 1000}
		returnKey = "outputs"
		returnVal = res
	}
	return
}

func init() {
	obj := new(MaxDatumsPerCall)
	RegisterRule("max_datums_per_call", obj)
}
