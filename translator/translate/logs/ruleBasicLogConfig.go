// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logs

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
)

type BasicLogConfig struct {
}

func (f *BasicLogConfig) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	cloudwatchlogsConfig := map[string]interface{}{}
	// add creds
	cloudwatchlogsConfig = translator.MergeTwoUniqueMaps(cloudwatchlogsConfig, agent.Global_Config.Credentials)
	cloudwatchlogsConfig[agent.RegionKey] = agent.Global_Config.Region
	cloudwatchlogsConfig[agent.RegionType] = agent.Global_Config.RegionType
	cloudwatchlogsConfig[agent.Mode] = context.CurrentContext().ShortMode()

	returnKey = Output_Cloudwatch_Logs
	returnVal = cloudwatchlogsConfig
	return
}

func init() {
	RegisterRule("basic_log_config", new(BasicLogConfig))
}
