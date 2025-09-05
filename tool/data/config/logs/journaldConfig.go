// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logs

import "github.com/aws/amazon-cloudwatch-agent/tool/runtime"

type JournaldFilter struct {
	Type       string `json:"type"`
	Expression string `json:"expression"`
}

type JournaldConfig struct {
	LogGroup        string            `json:"log_group_name"`
	LogStream       string            `json:"log_stream_name"`
	Units           []string          `json:"units"`
	Filters         []*JournaldFilter `json:"filters"`
	RetentionInDays int               `json:"retention_in_days"`
}

func (config *JournaldConfig) ToMap(ctx *runtime.Context) (string, map[string]interface{}) {
	resultMap := make(map[string]interface{})
	
	resultMap["log_group_name"] = config.LogGroup
	resultMap["log_stream_name"] = config.LogStream
	
	if len(config.Units) > 0 {
		resultMap["units"] = config.Units
	}
	
	if len(config.Filters) > 0 {
		filters := make([]map[string]interface{}, len(config.Filters))
		for i, filter := range config.Filters {
			filters[i] = map[string]interface{}{
				"type":       filter.Type,
				"expression": filter.Expression,
			}
		}
		resultMap["filters"] = filters
	}
	
	if config.RetentionInDays != 0 {
		resultMap["retention_in_days"] = config.RetentionInDays
	}
	
	return "", resultMap
}