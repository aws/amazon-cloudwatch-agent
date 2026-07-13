//go:build windows

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package useragent

import (
	"github.com/influxdata/telegraf/models"

	"github.com/aws/amazon-cloudwatch-agent/plugins/inputs/windows_event_log"
)

const (
	flagWindowsEventIDs     = "win_event_ids"
	flagWindowsEventFilters = "win_event_filters"
	flagWindowsEventLevels  = "win_event_levels"
	pluginWindowsEventLog   = "windows_event_log"
)

func (ua *userAgent) setWindowsEventLogFeatureFlags(input *models.RunningInput) {
	if input.Config.Name == pluginWindowsEventLog {
		if plugin, ok := input.Input.(*windows_event_log.Plugin); ok {
			for _, eventConfig := range plugin.Events {
				if len(eventConfig.EventIDs) > 0 {
					ua.feature.Add(flagWindowsEventIDs)
				}
				if len(eventConfig.Filters) > 0 {
					ua.feature.Add(flagWindowsEventFilters)
				}
				if len(eventConfig.Levels) > 0 {
					ua.feature.Add(flagWindowsEventLevels)
				}
			}
		}
	}
}
