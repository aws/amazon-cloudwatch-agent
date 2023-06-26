// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package windows

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/runtime"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/util"
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
