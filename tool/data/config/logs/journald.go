// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logs

import "github.com/aws/amazon-cloudwatch-agent/tool/runtime"

type Journald struct {
	JournaldConfigs []*JournaldConfig
}

func (config *Journald) ToMap(ctx *runtime.Context) (string, map[string]interface{}) {
	resultMap := map[string]interface{}{
		"collect_list": make([]map[string]interface{}, 0, len(config.JournaldConfigs)),
	}
	for i := range config.JournaldConfigs {
		_, singleJournald := config.JournaldConfigs[i].ToMap(ctx)
		resultMap["collect_list"] = append(resultMap["collect_list"].([]map[string]interface{}), singleJournald)
	}
	return "journald", resultMap
}

func (config *Journald) AddJournald(logGroupName, logStreamName string, units []string, priority string, matches []map[string]string, filters []*JournaldFilter, retention int) {
	if config.JournaldConfigs == nil {
		config.JournaldConfigs = []*JournaldConfig{}
	}

	singleJournald := &JournaldConfig{
		LogGroup:        logGroupName,
		LogStream:       logStreamName,
		Units:           units,
		Priority:        priority,
		Matches:         matches,
		Filters:         filters,
		RetentionInDays: retention,
	}

	config.JournaldConfigs = append(config.JournaldConfigs, singleJournald)
}
