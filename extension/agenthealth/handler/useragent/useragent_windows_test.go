//go:build windows

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package useragent

import (
	"testing"

	"github.com/influxdata/telegraf/models"
	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/plugins/inputs/windows_event_log"
)

func TestSetWindowsEventLogFeatureFlags(t *testing.T) {
	tests := []struct {
		name          string
		inputName     string
		plugin        *windows_event_log.Plugin
		expectedFlags []string
	}{
		{
			name:          "non-windows input",
			inputName:     "cpu",
			plugin:        &windows_event_log.Plugin{},
			expectedFlags: []string{},
		},
		{
			name:      "no features",
			inputName: pluginWindowsEventLog,
			plugin: &windows_event_log.Plugin{
				Events: []windows_event_log.EventConfig{{Name: "System"}},
			},
			expectedFlags: []string{},
		},
		{
			name:      "win_event_ids",
			inputName: pluginWindowsEventLog,
			plugin: &windows_event_log.Plugin{
				Events: []windows_event_log.EventConfig{{
					Name:     "System",
					EventIDs: []int{1000, 1001},
				}},
			},
			expectedFlags: []string{flagWindowsEventIDs},
		},
		{
			name:      "win_event_filters",
			inputName: pluginWindowsEventLog,
			plugin: &windows_event_log.Plugin{
				Events: []windows_event_log.EventConfig{{
					Name:    "System",
					Filters: []*windows_event_log.EventFilter{{Expression: "test"}},
				}},
			},
			expectedFlags: []string{flagWindowsEventFilters},
		},
		{
			name:      "win_event_levels",
			inputName: pluginWindowsEventLog,
			plugin: &windows_event_log.Plugin{
				Events: []windows_event_log.EventConfig{{
					Name:   "System",
					Levels: []string{"ERROR", "WARNING"},
				}},
			},
			expectedFlags: []string{flagWindowsEventLevels},
		},
		{
			name:      "all flags",
			inputName: pluginWindowsEventLog,
			plugin: &windows_event_log.Plugin{
				Events: []windows_event_log.EventConfig{{
					Name:     "System",
					EventIDs: []int{1000},
					Filters:  []*windows_event_log.EventFilter{{Expression: "test"}},
					Levels:   []string{"ERROR"},
				}},
			},
			expectedFlags: []string{flagWindowsEventIDs, flagWindowsEventFilters, flagWindowsEventLevels},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ua := newUserAgent()
			input := &models.RunningInput{
				Config: &models.InputConfig{Name: tt.inputName},
				Input:  tt.plugin,
			}

			ua.setWindowsEventLogFeatureFlags(input)

			for _, flag := range tt.expectedFlags {
				assert.Contains(t, ua.feature, flag)
			}
			assert.Len(t, ua.feature, len(tt.expectedFlags))
		})
	}
}
