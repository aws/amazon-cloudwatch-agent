// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package events

import (
	"fmt"
	"strconv"

	"github.com/aws/amazon-cloudwatch-agent/tool/data"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/tracesconfig"
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

const (
	VERBOSE     = "VERBOSE"     //5
	INFORMATION = "INFORMATION" //4
	WARNING     = "WARNING"     //3
	ERROR       = "ERROR"       //2
	CRITICAL    = "CRITICAL"    //1

	WindowsEventLog = "Windows event log"

	EventFormatXMLDescription       = "XML: XML format in Windows Event Viewer"
	EventFormatPlainTextDescription = "Plain Text: Legacy CloudWatch Windows Agent (SSM Plugin) Format"

	EventFormatXML       = "xml"
	EventFormatPlainText = "text"
)

var Processor processors.Processor = &processor{}

type processor struct{}

func (p *processor) Process(ctx *runtime.Context, config *data.Config) {
	monitorEvents(ctx, config)
}

func (p *processor) NextProcessor(ctx *runtime.Context, config *data.Config) interface{} {
	return tracesconfig.Processor
}

func monitorEvents(ctx *runtime.Context, config *data.Config) {
	yes := util.Yes(fmt.Sprintf("Do you want to monitor any %s?", WindowsEventLog))
	if !yes {
		return
	}

	eventFormatDefaultOption := 1
	for {
		logsConf := config.LogsConf()
		eventName := util.AskWithDefault(fmt.Sprintf("%s name:", WindowsEventLog), "System")

		availableEventLevels := []string{VERBOSE, INFORMATION, WARNING, ERROR, CRITICAL}
		eventLevels := []string{}
		for _, eventLevel := range availableEventLevels {
			yes = util.Yes(
				fmt.Sprintf("Do you want to monitor %s level events for %s %s ?",
					eventLevel,
					WindowsEventLog,
					eventName))
			if yes {
				eventLevels = append(eventLevels, eventLevel)
			}
		}

		logGroupName := util.AskWithDefault("Log group name:", eventName)

		logStreamNameHint := "{instance_id}"
		if ctx.IsOnPrem {
			logStreamNameHint = "{hostname}"
		}

		logStreamName := util.AskWithDefault("Log stream name:", logStreamNameHint)

		logGroupDefaultOption := 1
		logGroupClass := util.Choice("Which log group class would you like to have for this log group?", logGroupDefaultOption, []string{util.StandardLogGroupClass, util.InfrequentAccessLogGroupClass})
		if logGroupClass == util.StandardLogGroupClass {
			logGroupClass = util.StandardLogGroupClass
			logGroupDefaultOption = 1
		} else {
			logGroupClass = util.InfrequentAccessLogGroupClass
			logGroupDefaultOption = 2
		}

		eventFormat := util.Choice("In which format do you want to store windows event to CloudWatch Logs?", eventFormatDefaultOption, []string{EventFormatXMLDescription, EventFormatPlainTextDescription})
		if eventFormat == EventFormatXMLDescription {
			eventFormat = EventFormatXML
			eventFormatDefaultOption = 1
		} else {
			eventFormat = EventFormatPlainText
			eventFormatDefaultOption = 2
		}
		keys := translator.ValidRetentionInDays
		retentionInDays := util.Choice("Log Group Retention in days", 1, keys)
		retention := -1

		i, err := strconv.Atoi(retentionInDays)
		if err == nil {
			retention = i
		}
		logsConf.AddWindowsEvent(eventName, logGroupName, logStreamName, eventFormat, eventLevels, retention, logGroupClass)

		yes = util.Yes(fmt.Sprintf("Do you want to specify any additional %s to monitor?", WindowsEventLog))
		if !yes {
			return
		}
	}
}
