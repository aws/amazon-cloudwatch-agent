// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package config

import (
	"github.com/aws/amazon-cloudwatch-agent/tool/data/config/metric"
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
)

type Metrics struct {
	AppendDimensions *metric.AppendDimensions

	AggregationDimensions *metric.AggregationDimensions

	MetricsCollect *metric.Collection
}

func (config *Metrics) ToMap(ctx *runtime.Context) (string, map[string]interface{}) {
	resultMap := make(map[string]interface{})

	//Add the global dimensions if this is for ec2 host and customer want ec2 dimensions added.
	if config.AppendDimensions == nil && ctx.WantEC2TagDimensions {
		config.AppendDimensions = new(metric.AppendDimensions)
	}

	if config.AppendDimensions != nil {
		mapKey, mapValue := config.AppendDimensions.ToMap(ctx)
		if mapValue != nil {
			resultMap[mapKey] = mapValue
		}
	}

	if config.AggregationDimensions == nil && ctx.WantAggregateDimensions {
		config.AggregationDimensions = new(metric.AggregationDimensions)
	}

	if config.AggregationDimensions != nil {
		mapKey, mapValue := config.AggregationDimensions.ToMap(ctx)
		if mapValue != nil {
			resultMap[mapKey] = mapValue
		}
	}

	if config.MetricsCollect != nil {
		mapKey, mapValue := config.MetricsCollect.ToMap(ctx)
		resultMap[mapKey] = mapValue
	}

	return "metrics", resultMap
}

func (config *Metrics) CollectAllMetrics(ctx *runtime.Context) {
	config.MetricsCollect = new(metric.Collection)
	config.MetricsCollect.EnableAll(ctx)
}

func (config *Metrics) Collection() *metric.Collection {
	if config.MetricsCollect == nil {
		config.MetricsCollect = new(metric.Collection)
	}
	return config.MetricsCollect
}
