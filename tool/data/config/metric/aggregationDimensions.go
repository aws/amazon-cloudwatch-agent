// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metric

import "github.com/aws/amazon-cloudwatch-agent/tool/runtime"

type AggregationDimensions struct {
	Dimensions [][]string
}

func (config *AggregationDimensions) ToMap(ctx *runtime.Context) (string, [][]string) {
	config.SetDefaultDimensions()
	return "aggregation_dimensions", config.Dimensions
}

func (config *AggregationDimensions) SetDefaultDimensions() {
	config.Dimensions = [][]string{{"InstanceId"}}
}
