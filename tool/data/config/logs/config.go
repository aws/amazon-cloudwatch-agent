// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logs

import "github.com/aws/amazon-cloudwatch-agent/tool/runtime"

type Config struct {
	FilePath              string `file_path`
	LogGroup              string `log_group_name`
	LogStream             string `log_stream_name`
	LogGroupClass         string `log_group_class`
	TimestampFormat       string `timestamp_format`
	Timezone              string `timezone`
	MultiLineStartPattern string `multi_line_start_pattern`
	Encoding              string `encoding`
	Retention             int    `retention_in_days`
}

func (config *Config) ToMap(ctx *runtime.Context) (string, map[string]interface{}) {
	resultMap := make(map[string]interface{})
	resultMap["file_path"] = config.FilePath
	resultMap["log_group_name"] = config.LogGroup
	if config.TimestampFormat != "" {
		resultMap["timestamp_format"] = config.TimestampFormat
	}
	if config.Timezone != "" {
		resultMap["timezone"] = config.Timezone
	}
	if config.MultiLineStartPattern != "" {
		resultMap["multi_line_start_pattern"] = config.MultiLineStartPattern
	}
	if config.LogStream != "" {
		resultMap["log_stream_name"] = config.LogStream
	}
	if config.Encoding != "" {
		resultMap["encoding"] = config.Encoding
	}
	if config.Retention != 0 {
		resultMap["retention_in_days"] = config.Retention
	}
	if config.LogGroupClass != "" {
		resultMap["log_group_class"] = config.LogGroupClass
	}
	return "", resultMap
}
