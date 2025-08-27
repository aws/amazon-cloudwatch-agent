// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logs

import "github.com/aws/amazon-cloudwatch-agent/tool/runtime"

type EventFilter struct {
	Type       string `json:"type"`
	Expression string `json:"expression"`
}
type EventConfig struct {
	EventName     string         `json:"event_name"`
	EventLevels   []string       `json:"event_levels"`
	EventIDs      []int          `json:"event_ids"`
	Filters       []*EventFilter `json:"filters"`
	EventFormat   string         `json:"event_format"`
	LogGroup      string         `json:"log_group_name"`
	LogStream     string         `json:"log_stream_name"`
	LogGroupClass string         `json:"log_group_class"`
	Retention     int            `json:"retention_in_days"`
}

func (config *EventConfig) ToMap(ctx *runtime.Context) (string, map[string]interface{}) {
	resultMap := make(map[string]interface{})
	resultMap["event_name"] = config.EventName
	if len(config.EventLevels) > 0 {
		resultMap["event_levels"] = config.EventLevels
	}
	if len(config.EventIDs) > 0 {
		resultMap["event_ids"] = config.EventIDs
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
	if config.EventFormat != "" {
		resultMap["event_format"] = config.EventFormat
	}
	resultMap["log_group_name"] = config.LogGroup
	resultMap["log_stream_name"] = config.LogStream
	if config.LogGroupClass != "" {
		resultMap["log_group_class"] = config.LogGroupClass
	}
	if config.Retention != 0 {
		resultMap["retention_in_days"] = config.Retention
	}
	return "", resultMap
}
