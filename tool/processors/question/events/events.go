// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package events

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/tool/data"
	"github.com/aws/amazon-cloudwatch-agent/tool/data/config/logs"
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

	FilterTypeInclude = "include"
	FilterTypeExclude = "exclude"
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

		var eventIDs []int
		if util.Yes("Do you want to filter by specific Event IDs?") {
			eventIDsInput := util.Ask("Enter Event IDs (comma-separated, e.g., 1001,1002,1003):")
			if eventIDsInput != "" {
				eventIDStrings := strings.Split(eventIDsInput, ",")
				for _, idStr := range eventIDStrings {
					idStr = strings.TrimSpace(idStr)
					if id, err := strconv.Atoi(idStr); err == nil && id >= 0 && id <= 65535 {
						eventIDs = append(eventIDs, id)
					} else {
						fmt.Printf("Warning: Invalid Event ID '%s' ignored\n", idStr)
					}
				}
			}
		}
		var filters []*logs.EventFilter
		if util.Yes("Do you want to add regex filters to include/exclude specific events?") {
			for {
				filterType := util.Choice("Filter type:", 1, []string{"Include (events matching regex)", "Exclude (events matching regex)"})
				var filterTypeStr string
				if filterType == "Include (events matching regex)" {
					filterTypeStr = FilterTypeInclude
				} else {
					filterTypeStr = FilterTypeExclude
				}
				regexPattern := util.Ask("Enter regex pattern:")
				if regexPattern != "" {
					if _, err := regexp.Compile(regexPattern); err != nil {
						fmt.Printf("Error: Invalid regex pattern '%s': %v\n", regexPattern, err)
						continue
					}
					filter := &logs.EventFilter{
						Type:       filterTypeStr,
						Expression: regexPattern,
					}
					filters = append(filters, filter)
				}
				if !util.Yes("Do you want to add another regex filter?") {
					break
				}
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
		logsConf.AddWindowsEvent(eventName, logGroupName, logStreamName, eventFormat, eventLevels, eventIDs, filters, retention, logGroupClass)

		yes = util.Yes(fmt.Sprintf("Do you want to specify any additional %s to monitor?", WindowsEventLog))
		if !yes {
			return
		}
	}
}
