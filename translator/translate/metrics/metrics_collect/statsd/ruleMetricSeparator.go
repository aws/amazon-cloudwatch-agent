package statsd

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type MetricSeparator struct {
}

const SectionKey_MetricSeparator = "metric_separator"

func (obj *MetricSeparator) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	key, val := translator.DefaultCase(SectionKey_MetricSeparator, "", input)
	if val != "" {
		return key, val
	}
	return
}

func init() {
	obj := new(MetricSeparator)
	RegisterRule(SectionKey_MetricSeparator, obj)
}
