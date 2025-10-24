//go:build windows

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package useragent

import (
	"github.com/influxdata/telegraf/models"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	"github.com/aws/amazon-cloudwatch-agent/plugins/inputs/windows_event_log"
)

func (ua *userAgent) detectWindowsEventLogFeatures(input *models.RunningInput, winFeatures collections.Set[string]) {
	if input.Config.Name == pluginWindowsEventLog {
		if plugin, ok := input.Input.(*windows_event_log.Plugin); ok {
			for _, eventConfig := range plugin.Events {
				if len(eventConfig.EventIDs) > 0 {
					winFeatures.Add(FlagWindowsEventIDs)
				}
				if len(eventConfig.Filters) > 0 {
					winFeatures.Add(FlagWindowsEventFilters)
				}
				if len(eventConfig.Levels) > 0 {
					winFeatures.Add(FlagWindowsEventLevels)
				}
			}
		}
	}
}
