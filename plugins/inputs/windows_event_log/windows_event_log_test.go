// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package windows_event_log

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/useragent"
	"github.com/aws/amazon-cloudwatch-agent/plugins/inputs/windows_event_log/wineventlog"
)

func TestDetectFeatures(t *testing.T) {
	plugin := &Plugin{
		Events: []EventConfig{
			{
				EventIDs: []int{1000, 1001},
			},
			{
				Filters: []*wineventlog.EventFilter{{Expression: "test"}},
				Levels:  []string{"ERROR"},
			},
		},
	}

	ua := useragent.Get()
	plugin.detectFeatures()

	header := ua.Header(true)
	assert.Contains(t, header, useragent.FlagWindowsEventIDs)
	assert.Contains(t, header, useragent.FlagWindowsEventFilters)
	assert.Contains(t, header, useragent.FlagWindowsEventLevels)

	// Test that only configured features are detected
	plugin = &Plugin{
		Events: []EventConfig{{
			EventIDs: []int{1000},
		}},
	}
	ua = useragent.Get()
	ua.Reset()
	plugin.detectFeatures()

	header = ua.Header(true)
	assert.Contains(t, header, useragent.FlagWindowsEventIDs)
	assert.NotContains(t, header, useragent.FlagWindowsEventFilters)
	assert.NotContains(t, header, useragent.FlagWindowsEventLevels)
}
