// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package windows

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/runtime"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/util"
)

type Memory struct {
	PercentCommittedBytesInUse bool `% Committed Bytes In Use`
}

func (config *Memory) ToMap(ctx *runtime.Context) (string, map[string]interface{}) {
	resultMap := make(map[string]interface{})
	if ctx.MetricsCollectionInterval != 0 {
		resultMap[util.MapKeyMetricsCollectionInterval] = ctx.MetricsCollectionInterval
	}
	measurement := []string{}
	if config.PercentCommittedBytesInUse {
		measurement = append(measurement, "% Committed Bytes In Use")
	}
	resultMap[util.MapKeyMeasurement] = measurement
	return "Memory", resultMap
}

func (config *Memory) Enable() {
	config.PercentCommittedBytesInUse = true
}
