// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package journald

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
	FilterTypeInclude = "include"
	FilterTypeExclude = "exclude"
)

var Processor processors.Processor = &processor{}

type processor struct{}

func (p *processor) Process(ctx *runtime.Context, config *data.Config) {
	monitorJournald(ctx, config)
}

func (p *processor) NextProcessor(ctx *runtime.Context, config *data.Config) interface{} {
	return tracesconfig.Processor
}

func monitorJournald(ctx *runtime.Context, config *data.Config) {
	yes := util.Yes("Do you want to monitor journald logs?")
	if !yes {
		return
	}

	for {
		logsConf := config.LogsConf()
		
		// Ask if they want all units or specific ones
		collectAllUnits := util.Yes("Do you want to collect logs from all units?")
		
		var units []string
		if !collectAllUnits {
			// Ask about common units individually
			commonUnits := []string{
				"systemd",
				"kernel", 
				"sshd",
				"docker",
				"nginx",
				"apache2",
				"NetworkManager",
				"firewalld",
				"audit",
			}
			
			for _, unit := range commonUnits {
				if util.Yes(fmt.Sprintf("Do you want to monitor %s unit logs?", unit)) {
					units = append(units, unit)
				}
			}
			
			// Ask about custom units
			if util.Yes("Do you want to monitor custom units?") {
				customUnitsInput := util.Ask("Enter custom unit names (comma-separated):")
				if customUnitsInput != "" {
					customUnits := strings.Split(customUnitsInput, ",")
					for _, unit := range customUnits {
						unit = strings.TrimSpace(unit)
						if unit != "" {
							units = append(units, unit)
						}
					}
				}
			}
		}
		// If collectAllUnits is true, units remains empty (which means all units)
		
		// Log group name
		logGroupName := util.AskWithDefault("Log group name:", "journald")
		
		// Log stream name
		logStreamNameHint := "{instance_id}"
		if ctx.IsOnPrem {
			logStreamNameHint = "{hostname}"
		}
		logStreamName := util.AskWithDefault("Log stream name:", logStreamNameHint)
		
		// Filters (optional)
		var filters []*logs.JournaldFilter
		if util.Yes("Do you want to add regex filters to include/exclude specific log entries?") {
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
					filter := &logs.JournaldFilter{
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
		
		// Retention
		keys := translator.ValidRetentionInDays
		retentionInDays := util.Choice("Log Group Retention in days", 1, keys)
		retention := -1
		i, err := strconv.Atoi(retentionInDays)
		if err == nil {
			retention = i
		}
		
		logsConf.AddJournald(units, logGroupName, logStreamName, filters, retention)
		
		yes = util.Yes("Do you want to specify any additional journald configurations to monitor?")
		if !yes {
			return
		}
	}
}