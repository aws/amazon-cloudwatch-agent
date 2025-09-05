// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logs

import (
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

type Collection struct {
	// file based logs
	Files *Files
	// windows events
	WinEvents *Events
	// journald logs
	Journald *Journald
}

func (config *Collection) ToMap(ctx *runtime.Context) (string, map[string]interface{}) {
	resultMap := make(map[string]interface{})

	if config.Files != nil {
		util.AddToMap(ctx, resultMap, config.Files)
	}

	// WinEvents should be nil if the target Os is windows
	if config.WinEvents != nil {
		util.AddToMap(ctx, resultMap, config.WinEvents)
	}

	// Journald should be nil if the target Os is not Linux
	if config.Journald != nil {
		util.AddToMap(ctx, resultMap, config.Journald)
	}

	return "logs_collected", resultMap
}

func (config *Collection) AddWindowsEvent(eventName, logGroupName, logStreamName, eventFormat string, eventLevels []string, eventIDs []int, filters []*EventFilter, retention int, logGroupClass string) {
	if config.WinEvents == nil {
		config.WinEvents = &Events{}
	}
	config.WinEvents.AddWindowsEvent(eventName, logGroupName, logStreamName, eventFormat, eventLevels, eventIDs, filters, retention, logGroupClass)
}

func (config *Collection) AddLogFile(filePath, logGroupName, logStreamName string, timestampFormat, timezone, multiLineStartPattern, encoding string, retention int, logGroupClass string) {
	if config.Files == nil {
		config.Files = &Files{}
	}
	config.Files.AddLogFile(filePath, logGroupName, logStreamName, timestampFormat, timezone, multiLineStartPattern, encoding, retention, logGroupClass)
}

func (config *Collection) AddJournald(units []string, logGroupName, logStreamName string, filters []*JournaldFilter, retention int) {
	if config.Journald == nil {
		config.Journald = &Journald{}
	}
	config.Journald.AddJournald(units, logGroupName, logStreamName, filters, retention)
}
