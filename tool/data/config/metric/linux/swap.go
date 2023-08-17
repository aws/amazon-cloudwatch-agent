// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package linux

import (
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

type Swap struct {
	UsedPercent bool `swap_used_percent`
}

func (config *Swap) ToMap(ctx *runtime.Context) (string, map[string]interface{}) {
	resultMap := make(map[string]interface{})
	if ctx.MetricsCollectionInterval != 0 {
		resultMap[util.MapKeyMetricsCollectionInterval] = ctx.MetricsCollectionInterval
	}
	measurement := []string{}
	if config.UsedPercent {
		measurement = append(measurement, "swap_used_percent")
	}
	resultMap[util.MapKeyMeasurement] = measurement
	return "swap", resultMap
}

func (config *Swap) Enable() {
	config.UsedPercent = true
}
