// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package windows

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/runtime"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/util"
)

type Processor struct {
	Instances []string

	PercentProcessorTime bool `% Processor Time`
	PercentUserTime      bool `% User Time`
	PercentIdleTime      bool `% Idle Time`
	PercentInterruptTime bool `% Interrupt Time`
}

func (config *Processor) ToMap(ctx *runtime.Context) (string, map[string]interface{}) {
	resultMap := make(map[string]interface{})

	if config.Instances != nil && len(config.Instances) > 0 {
		resultMap[util.MapKeyInstances] = config.Instances
	} else if ctx.WantPerInstanceMetrics {
		resultMap[util.MapKeyInstances] = []string{"*"}
	} else {
		resultMap[util.MapKeyInstances] = []string{"_Total"}
	}

	if ctx.MetricsCollectionInterval != 0 {
		resultMap[util.MapKeyMetricsCollectionInterval] = ctx.MetricsCollectionInterval
	}

	measurement := []string{}
	if config.PercentProcessorTime {
		measurement = append(measurement, "% Processor Time")
	}
	if config.PercentUserTime {
		measurement = append(measurement, "% User Time")
	}
	if config.PercentIdleTime {
		measurement = append(measurement, "% Idle Time")
	}
	if config.PercentInterruptTime {
		measurement = append(measurement, "% Interrupt Time")
	}
	resultMap[util.MapKeyMeasurement] = measurement

	return "Processor", resultMap
}

func (config *Processor) Enable() {
	config.PercentProcessorTime = true
	config.PercentUserTime = true
	config.PercentIdleTime = true
	config.PercentInterruptTime = true
}
