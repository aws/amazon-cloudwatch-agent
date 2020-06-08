package metrics

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type ForceFlushInterval struct {
}

func (f *ForceFlushInterval) ApplyRule(input interface{}) (string, interface{}) {
	key, val := translator.DefaultTimeIntervalCase("force_flush_interval", float64(60), input)
	return "outputs", map[string]interface{}{key: val}
}

func init() {
	RegisterRule("forceFlushInterval", new(ForceFlushInterval))
}
