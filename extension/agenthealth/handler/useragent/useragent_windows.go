//go:build windows

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package useragent

import (
	"fmt"
	"github.com/influxdata/telegraf/models"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	"github.com/aws/amazon-cloudwatch-agent/plugins/inputs/windows_event_log"
)

func (ua *userAgent) detectWindowsEventLogFeatures(input *models.RunningInput, winFeatures collections.Set[string]) {
	fmt.Printf("DEBUG: detectWindowsEventLogFeatures called for plugin: %s\n", input.Config.Name)
	if input.Config.Name == pluginWindowsEventLog {
		fmt.Printf("DEBUG: Found windows_event_log plugin\n")
		if plugin, ok := input.Input.(*windows_event_log.Plugin); ok {
			fmt.Printf("DEBUG: Type assertion successful, found %d events\n", len(plugin.Events))
			for i, eventConfig := range plugin.Events {
				fmt.Printf("DEBUG: Event %d - EventIDs: %d, Filters: %d, Levels: %d\n", 
					i, len(eventConfig.EventIDs), len(eventConfig.Filters), len(eventConfig.Levels))
				if len(eventConfig.EventIDs) > 0 {
					fmt.Printf("DEBUG: Adding win_event_ids\n")
					winFeatures.Add(FlagWindowsEventIDs)
				}
				if len(eventConfig.Filters) > 0 {
					fmt.Printf("DEBUG: Adding win_event_filters\n")
					winFeatures.Add(FlagWindowsEventFilters)
				}
				if len(eventConfig.Levels) > 0 {
					fmt.Printf("DEBUG: Adding win_event_levels\n")
					winFeatures.Add(FlagWindowsEventLevels)
				}
			}
		} else {
			fmt.Printf("DEBUG: Type assertion failed\n")
		}
	}
}
