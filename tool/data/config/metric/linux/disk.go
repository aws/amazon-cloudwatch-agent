// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package linux

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/runtime"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/util"
)

type Disk struct {
	Instances []string

	UsedPercent bool `used_percent`
	InodesFree  bool `inodes_free`
}

func (config *Disk) ToMap(ctx *runtime.Context) (string, map[string]interface{}) {
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
	if config.UsedPercent {
		measurement = append(measurement, "used_percent")
	}
	if config.InodesFree {
		measurement = append(measurement, "inodes_free")
	}
	resultMap[util.MapKeyMeasurement] = measurement
	return "disk", resultMap
}

func (config *Disk) Enable() {
	config.UsedPercent = true
	config.InodesFree = true
}
