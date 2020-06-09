package agent

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type Interval struct {
}

func (i *Interval) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	_, returnVal = translator.DefaultTimeIntervalCase("metrics_collection_interval", float64(60), input)
	returnKey = "interval"
	// Set global collection interval
	Global_Config.Interval = returnVal.(string)
	return
}

func init() {
	i := new(Interval)
	RegisterRule("interval", i)
}
