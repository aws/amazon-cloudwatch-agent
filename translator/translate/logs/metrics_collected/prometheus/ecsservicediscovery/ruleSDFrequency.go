package ecsservicediscovery

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

const (
	SectionKeySDFrequency = "sd_frequency"
)

type SDFrequency struct {
}

func (d *SDFrequency) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase(SectionKeySDFrequency, "1m", input)
	return
}

func init() {
	RegisterRule(SectionKeySDFrequency, new(SDFrequency))
}
