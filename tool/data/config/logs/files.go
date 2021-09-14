// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logs

import (
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
)

type Files struct {
	FileConfigs []*Config
}

func (config *Files) ToMap(ctx *runtime.Context) (string, map[string]interface{}) {
	resultMap := make(map[string]interface{})

	collectList := []map[string]interface{}{}
	for i := range config.FileConfigs {
		_, singleFile := config.FileConfigs[i].ToMap(ctx)
		collectList = append(collectList, singleFile)
	}
	resultMap["collect_list"] = collectList

	return "files", resultMap
}

func (config *Files) AddLogFile(filePath, logGroupName, logStreamName string, timestampFormat, timezone, multiLineStartPattern, encoding string, includePattern string) {
	if config.FileConfigs == nil {
		config.FileConfigs = []*Config{}
	}
	singleFile := &Config{
		FilePath: filePath,
		LogGroup: logGroupName,
	}
	if timestampFormat != "" {
		singleFile.TimestampFormat = timestampFormat
	}
	if timezone != "" {
		singleFile.Timezone = timezone
	}
	if multiLineStartPattern != "" {
		singleFile.MultiLineStartPattern = multiLineStartPattern
	}
	if logStreamName != "" {
		singleFile.LogStream = logStreamName
	}
	if encoding != "" {
		singleFile.Encoding = encoding
	}
	if includePattern != "" {
		singleFile.IncludePattern = includePattern
	}
	config.FileConfigs = append(config.FileConfigs, singleFile)
}
