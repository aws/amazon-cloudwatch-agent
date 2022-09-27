// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logs

import (
	"fmt"
	"os"
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/util"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
)

type LogStreamName struct {
}

func (l *LogStreamName) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	ctx := context.CurrentContext()
	var defaultVal string
	if ctx.RunInContainer() {
		if ecsutil.GetECSUtilSingleton().IsECS() {
			// https://docs.aws.amazon.com/AmazonECS/latest/userguide/ecs-account-settings.html#ecs-resource-ids
			// https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_PutLogEvents.html#API_PutLogEvents_RequestParameters
			// Log stream name cannot have ":" or "*". Replace ":" with "_". ECS task arn won't have "*".
			defaultVal = strings.ReplaceAll(ecsutil.GetECSUtilSingleton().TaskARN, ":", "_")
		} else if podName, ok := os.LookupEnv(config.POD_NAME); ok {
			defaultVal = podName
		} else if hostName, ok := os.LookupEnv(config.HOST_NAME); ok {
			defaultVal = hostName
		} else if hostName, err := os.Hostname(); err == nil {
			defaultVal = hostName
		}
	} else {
		var err error
		if (ctx.Mode() == config.ModeOnPrem) || (ctx.Mode() == config.ModeOnPremise) {
			if _, inputVal := translator.DefaultCase("log_stream_name", "", input); inputVal == "" {
				if defaultVal, err = os.Hostname(); err != nil {
					translator.AddErrorMessages(GetCurPath(), "Failed to get hostName for log_stream_name field, please specify value for log_stream_name field")
					return
				} else {
					translator.AddInfoMessages(GetCurPath(), fmt.Sprintf("Got hostname %s as log_stream_name", defaultVal))
				}
			}
		} else {
			defaultVal = "{instance_id}"
		}
	}

	key, val := translator.DefaultCase("log_stream_name", defaultVal, input)
	val = util.ResolvePlaceholder(val.(string), GlobalLogConfig.MetadataInfo)
	res := map[string]interface{}{}
	res[key] = val
	returnKey = Output_Cloudwatch_Logs
	returnVal = res
	return
}

func init() {
	l := new(LogStreamName)
	RegisterRule("log_stream_name", l)
}
