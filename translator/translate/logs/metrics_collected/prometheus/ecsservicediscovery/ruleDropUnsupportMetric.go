package ecsservicediscovery

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
)

const (
	SectionKeyDropUnsupportedMetric = "drop_unsupported_metric"
	DefaultDropUnsupportedMetric = false
)

type DropUnsupportedMetric struct {
}

func (d *MetricNamespace) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
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