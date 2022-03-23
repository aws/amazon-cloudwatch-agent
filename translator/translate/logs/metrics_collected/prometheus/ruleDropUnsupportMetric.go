package emfprocessor

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

const (
	SectionKeyDropUnsupportedMetric = "drop_unsupported_metric"
	DefaultDropUnsupportedMetric = true
)

type DropUnsupportedMetric struct {
}

func (d *DropUnsupportedMetric) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase(SectionKeyDropUnsupportedMetric, "", input)
	if returnVal != "" {
		return
	}
	
	if returnVal == "" {
		returnVal = DefaultDropUnsupportedMetric
	}
	
	return
}

func init() {
	RegisterRule(SectionKeyDropUnsupportedMetric, new(DropUnsupportedMetric))
}