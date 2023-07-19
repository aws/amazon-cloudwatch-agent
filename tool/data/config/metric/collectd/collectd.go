// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collectd

import (
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
)

type CollectD struct {
	MetricsAggregationInterval int `metrics_aggregation_interval`
}

func (config *CollectD) ToMap(ctx *runtime.Context) (string, map[string]interface{}) {
	resultMap := make(map[string]interface{})
	resultMap["metrics_aggregation_interval"] = config.MetricsAggregationInterval
	return "collectd", resultMap
}

func (config *CollectD) Enable() {
	config.MetricsAggregationInterval = 60
}
