// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logs

import "github.com/aws/amazon-cloudwatch-agent/tool/runtime"

type Journald struct {
	JournaldConfigs []*JournaldConfig
}

func (config *Journald) ToMap(ctx *runtime.Context) (string, map[string]interface{}) {
	resultMap := make(map[string]interface{})
	
	collectList := []map[string]interface{}{}
	for i := range config.JournaldConfigs {
		_, singleJournald := config.JournaldConfigs[i].ToMap(ctx)
		collectList = append(collectList, singleJournald)
	}
	resultMap["collect_list"] = collectList
	
	return "journald", resultMap
}

func (config *Journald) AddJournald(units []string, logGroupName, logStreamName string, filters []*JournaldFilter, retention int) {
	if config.JournaldConfigs == nil {
		config.JournaldConfigs = []*JournaldConfig{}
	}
	
	singleJournald := &JournaldConfig{
		LogGroup:        logGroupName,
		LogStream:       logStreamName,
		Units:           units,
		Filters:         filters,
		RetentionInDays: retention,
	}
	
	config.JournaldConfigs = append(config.JournaldConfigs, singleJournald)
}