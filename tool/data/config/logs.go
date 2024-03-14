// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package config

import (
	"github.com/aws/amazon-cloudwatch-agent/tool/data/config/logs"
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
)

type Logs struct {
	ForceFlushInterval int `force_flush_interval`
	LogStream          string
	LogsCollect        *logs.Collection
}

func (config *Logs) ToMap(ctx *runtime.Context) (string, map[string]interface{}) {
	resultMap := make(map[string]interface{})

	if config.LogsCollect != nil {
		key, value := config.LogsCollect.ToMap(ctx)
		resultMap[key] = value
	}

	if config.ForceFlushInterval != 0 {
		resultMap["force_flush_interval"] = config.ForceFlushInterval
	}

	if config.LogStream != "" {
		resultMap["log_stream_name"] = config.LogStream
	}

	return "logs", resultMap
}

func (config *Logs) AddLogFile(filePath, logGroupName string, logStream, timestampFormat, timezone, multiLineStartPattern, encoding string, retention int, logGroupClass string) {
	if config.LogsCollect == nil {
		config.LogsCollect = &logs.Collection{}
	}

	config.LogsCollect.AddLogFile(filePath, logGroupName, logStream, timestampFormat, timezone, multiLineStartPattern, encoding, retention, logGroupClass)
}

func (config *Logs) AddWindowsEvent(eventName, logGroupName, logStream, eventFormat string, eventLevels []string, retention int, logGroupClass string) {
	if config.LogsCollect == nil {
		config.LogsCollect = &logs.Collection{}
	}
	config.LogsCollect.AddWindowsEvent(eventName, logGroupName, logStream, eventFormat, eventLevels, retention, logGroupClass)
}
