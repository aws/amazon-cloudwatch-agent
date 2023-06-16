// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logs

import (
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
)

const key = "max_cloudwatch_logs_buffer"

type MaxCloudwatchLogsBuffer struct {
}

func (m *MaxCloudwatchLogsBuffer) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey = Output_Cloudwatch_Logs
	res := map[string]interface{}{}
	res[key] = agent.Global_Config.MaxCloudwatchLogsBuffer
	returnVal = res
	return
}
func init() {
	m := new(MaxCloudwatchLogsBuffer)
	RegisterRule(key, m)
}
