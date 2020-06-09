package logs

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type ForceFlushInterval struct {
}

func (f *ForceFlushInterval) ApplyRule(input interface{}) (string, interface{}) {
	key, val := translator.DefaultTimeIntervalCase("force_flush_interval", float64(5), input)
	return "cloudwatchlogs", map[string]interface{}{key: val}
}

func init() {
	RegisterRule("forceFlushInterval", new(ForceFlushInterval))
}
