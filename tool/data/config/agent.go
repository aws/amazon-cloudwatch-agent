// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package config

import (
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

const RUNASUSER = "run_as_user"

type AgentConfig struct {
	MetricsCollectInterval string `metrics_collection_interval`
	Runasuser              string `run_as_user`
}

func (config *AgentConfig) ToMap(ctx *runtime.Context) (string, map[string]interface{}) {
	resultMap := make(map[string]interface{})
	if ctx.MetricsCollectionInterval != 0 {
		resultMap[util.MapKeyMetricsCollectionInterval] = ctx.MetricsCollectionInterval
	}

	if config.Runasuser != "" {
		resultMap[RUNASUSER] = config.Runasuser
	}

	return "agent", resultMap
}
