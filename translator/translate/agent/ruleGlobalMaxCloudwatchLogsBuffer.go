// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type GlobalMaxCloudwatchLogsBuffer struct {
}

const (
	MaxCloudwatchLogsBufferKey          = "max_cloudwatch_logs_buffer"
)


func (c *GlobalMaxCloudwatchLogsBuffer) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	_, v := translator.DefaultMaxCloudwatchLogsBuffer(MaxCloudwatchLogsBufferKey, float64(-1), input)
	Global_Config.MaxCloudwatchLogsBuffer = v.(int64)
	return
}

func init() {
	c := new(GlobalMaxCloudwatchLogsBuffer)
	RegisterRule(MaxCloudwatchLogsBufferKey, c)
}
