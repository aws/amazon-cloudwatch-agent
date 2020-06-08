package linux

import (
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

type Memory struct {
	MemUsedPercent bool `mem_used_percent`
}

func (config *Memory) ToMap(ctx *runtime.Context) (string, map[string]interface{}) {
	resultMap := make(map[string]interface{})
	if ctx.MetricsCollectionInterval != 0 {
		resultMap[util.MapKeyMetricsCollectionInterval] = ctx.MetricsCollectionInterval
	}
	measurement := []string{}
	if config.MemUsedPercent {
		measurement = append(measurement, "mem_used_percent")
	}
	resultMap[util.MapKeyMeasurement] = measurement
	return "mem", resultMap
}

func (config *Memory) Enable() {
	config.MemUsedPercent = true
}
