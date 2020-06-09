package windows

import (
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

type PagingFile struct {
	Instances []string

	PercentUsage bool `% Usage`
}

func (config *PagingFile) ToMap(ctx *runtime.Context) (string, map[string]interface{}) {
	resultMap := make(map[string]interface{})

	if config.Instances != nil && len(config.Instances) > 0 {
		resultMap[util.MapKeyInstances] = config.Instances
	} else {
		resultMap[util.MapKeyInstances] = []string{"*"}
	}

	if ctx.MetricsCollectionInterval != 0 {
		resultMap[util.MapKeyMetricsCollectionInterval] = ctx.MetricsCollectionInterval
	}

	measurement := []string{}
	if config.PercentUsage {
		measurement = append(measurement, "% Usage")
	}
	resultMap[util.MapKeyMeasurement] = measurement

	return "Paging File", resultMap
}

func (config *PagingFile) Enable() {
	config.PercentUsage = true
}
