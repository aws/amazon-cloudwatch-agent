// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package windows

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/runtime"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/util"
)

type LogicalDisk struct {
	Instances []string

	PercentFreeSpace bool `% Free Space`
}

func (config *LogicalDisk) ToMap(ctx *runtime.Context) (string, map[string]interface{}) {
	resultMap := make(map[string]interface{})

	if config.Instances != nil && len(config.Instances) > 0 {
		resultMap[util.MapKeyInstances] = config.Instances
	} else {
		resultMap[util.MapKeyInstances] = []string{"*"}
	}

	if config.Instances != nil && len(config.Instances) > 0 {
		resultMap[util.MapKeyInstances] = config.Instances
	}

	if ctx.MetricsCollectionInterval != 0 {
		resultMap[util.MapKeyMetricsCollectionInterval] = ctx.MetricsCollectionInterval
	}

	measurement := []string{}
	if config.PercentFreeSpace {
		measurement = append(measurement, "% Free Space")
	}
	resultMap[util.MapKeyMeasurement] = measurement

	return "LogicalDisk", resultMap
}

func (config *LogicalDisk) Enable() {
	config.PercentFreeSpace = true
}
