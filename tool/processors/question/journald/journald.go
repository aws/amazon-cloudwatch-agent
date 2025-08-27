// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package journald

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/aws/amazon-cloudwatch-agent/tool/data"
	"github.com/aws/amazon-cloudwatch-agent/tool/data/config/logs"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/question/events"
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

const (
	JournaldLog = "systemd journal log"

	FilterTypeInclude = "include"
	FilterTypeExclude = "exclude"
)

var Processor processors.Processor = &processor{}

type processor struct{}

func (p *processor) Process(ctx *runtime.Context, config *data.Config) {
	if ctx.OsParameter == util.OsTypeLinux {
		monitorJournald(ctx, config)
	}
}

func (p *processor) NextProcessor(ctx *runtime.Context, config *data.Config) interface{} {
	return events.Processor
}

func monitorJournald(ctx *runtime.Context, config *data.Config) {
	yes := util.Yes(fmt.Sprintf("Do you want to monitor any %s?", JournaldLog))
	if !yes {
		return
	}

	for {
		logsConf := config.LogsConf()
		journaldName := util.AskWithDefault(fmt.Sprintf("%s name:", JournaldLog), "journald")

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

		logGroupName := util.AskWithDefault("Log group name:", journaldName)

		logStreamNameHint := "{instance_id}"
		if ctx.IsOnPrem {
			logStreamNameHint = "{hostname}"
		}

		logStreamName := util.AskWithDefault("Log stream name:", logStreamNameHint)

		keys := translator.ValidRetentionInDays
		retentionInDays := util.Choice("Log Group Retention in days", 1, keys)
		retention := -1

		i, err := strconv.Atoi(retentionInDays)
		if err == nil {
			retention = i
		}
		logsConf.AddJournald(logGroupName, logStreamName, filters, retention)

		yes = util.Yes(fmt.Sprintf("Do you want to specify any additional %s to monitor?", JournaldLog))
		if !yes {
			return
		}
	}
}