package k8sapiserver

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type Interval struct {
}

func (i *Interval) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})
	if _, ok := m["metrics_collection_interval"]; !ok {
		return
	}
	_, returnVal = translator.DefaultTimeIntervalCase("metrics_collection_interval", float64(0), input)
	returnKey = "interval"
	return
}

func init() {
	i := new(Interval)
	RegisterRule("interval", i)
}
