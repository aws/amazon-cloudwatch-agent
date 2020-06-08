package cpu

import (
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/util"
)

type CollectCpuTime struct {
}

func (c *CollectCpuTime) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	measurementNames := util.GetMeasurementName(input)
	for _, measurementName := range measurementNames {
		measurementName = strings.TrimPrefix(measurementName, "cpu_")
		if strings.HasPrefix(measurementName, "time") {
			returnKey = "collect_cpu_time"
			returnVal = true
			return
		}
	}
	return
}

func init() {
	c := new(CollectCpuTime)
	RegisterRule("collect_cpu_time", c)
}
