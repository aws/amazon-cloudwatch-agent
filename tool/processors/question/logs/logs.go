// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logs

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/tool/data"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/question/events"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/tracesconfig"
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

var Processor processors.Processor = &processor{}

type processor struct{}

func (p *processor) Process(ctx *runtime.Context, config *data.Config) {
	monitorLogs(ctx, config)
}

func (p *processor) NextProcessor(ctx *runtime.Context, config *data.Config) interface{} {
	if ctx.OsParameter == util.OsTypeWindows {
		return events.Processor
	}
	return tracesconfig.Processor
}

func monitorLogs(ctx *runtime.Context, config *data.Config) {
	var question string
	//feedback from Windows customer that log files easier to mix with the Windows event log.
	if ctx.OsParameter == util.OsTypeWindows {
		question = "Do you want to monitor any customized log files?"
	} else {
		question = "Do you want to monitor any log files?"
	}
	yes := util.Yes(question)
	if !yes {
		return
	}
	for {
		logsConf := config.LogsConf()
		logFilePath := util.Ask("Log file path:")
		logGroupNameHint := strings.Replace(filepath.Base(logFilePath), " ", "_", -1)
		logGroupName := util.AskWithDefault("Log group name:", logGroupNameHint)
		logGroupClass := util.Choice("Log group class:", 1, []string{util.StandardLogGroupClass, util.InfrequentAccessLogGroupClass})
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
		logsConf.AddLogFile(logFilePath, logGroupName, logStreamName, "", "", "", "", retention, logGroupClass)
		yes = util.Yes("Do you want to specify any additional log files to monitor?")
		if !yes {
			return
		}
	}
}
