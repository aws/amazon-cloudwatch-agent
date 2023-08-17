// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metric

import "github.com/aws/amazon-cloudwatch-agent/tool/runtime"

type AppendDimensions struct {
	Dimensions map[string]interface{}
}

func (config *AppendDimensions) ToMap(ctx *runtime.Context) (string, map[string]interface{}) {
	config.SetDefaultDimensions()
	return "append_dimensions", config.Dimensions
}

func (config *AppendDimensions) SetDefaultDimensions() {
	config.Dimensions = map[string]interface{}{
		"ImageId":              "${aws:ImageId}",
		"InstanceId":           "${aws:InstanceId}",
		"InstanceType":         "${aws:InstanceType}",
		"AutoScalingGroupName": "${aws:AutoScalingGroupName}",
	}
}
