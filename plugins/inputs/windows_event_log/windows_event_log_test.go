// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package windows_event_log

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/useragent"
	"github.com/aws/amazon-cloudwatch-agent/plugins/inputs/windows_event_log/wineventlog"
)

// TestGetStateFilePathGood tests getStateFilePath with good input.
func TestGetStateFilePathGood(t *testing.T) {
	fileStateFolder := t.TempDir()
	plugin := Plugin{
		FileStateFolder: fileStateFolder,
	}
	ec := EventConfig{
		LogGroupName:  "MyGroup",
		LogStreamName: "MyStream",
		Name:          "SystemEventLog",
	}
	stateManagerCfg, err := getStateManagerConfig(&plugin, &ec)
	assert.NoError(t, err)
	t.Log(stateManagerCfg.StateFilePath())
	expected := filepath.Join(fileStateFolder,
		"Amazon_CloudWatch_WindowsEventLog_MyGroup_MyStream_SystemEventLog")
	assert.Equal(t, expected, stateManagerCfg.StateFilePath())
	_, err = os.Stat(fileStateFolder)
	assert.False(t, os.IsNotExist(err))
}

// TestGetStateFilePathEscape tests getStateFilePath() with special characters.
func TestGetStateFilePathEscape(t *testing.T) {
	fileStateFolder := t.TempDir()
	plugin := Plugin{
		FileStateFolder: fileStateFolder,
	}
	ec := EventConfig{
		LogGroupName:  "My  Group/:::",
		LogStreamName: "My::Stream//  ",
		Name:          "System  Event//Log::",
	}
	stateManagerCfg, err := getStateManagerConfig(&plugin, &ec)
	assert.NoError(t, err)
	t.Log(stateManagerCfg.StateFilePath())
	expected := filepath.Join(fileStateFolder,
		"Amazon_CloudWatch_WindowsEventLog_My__Group_____My__Stream_____System__Event__Log__")
	assert.Equal(t, expected, stateManagerCfg.StateFilePath())
}

// TestGetStateFilePathEmpty tests getStateFilePath() with empty folder.
func TestGetStateFilePathEmpty(t *testing.T) {
	fileStateFolder := ""
	plugin := Plugin{
		FileStateFolder: fileStateFolder,
	}
	ec := EventConfig{
		LogGroupName:  "MyGroup",
		LogStreamName: "MyStream",
		Name:          "SystemEventLog",
	}
	stateManagerCfg, err := getStateManagerConfig(&plugin, &ec)
	t.Log(stateManagerCfg.StateFilePath())
	assert.Error(t, err)
}

// TestGetStateFilePathSpecialChars tests getStateFilePath() with bad folder.
func TestGetStateFilePathSpecialChars(t *testing.T) {
	fileStateFolder := "F:\\\\bin!@#$%^&*)(\\CloudWatchAgentTest"
	// cleanup
	plugin := Plugin{
		FileStateFolder: fileStateFolder,
	}
	ec := EventConfig{
		LogGroupName:  "MyGroup",
		LogStreamName: "MyStream",
		Name:          "SystemEventLog",
	}
	stateManagerCfg, err := getStateManagerConfig(&plugin, &ec)
	t.Log(stateManagerCfg.StateFilePath())
	assert.Error(t, err)
}

func TestWindowsDuplicateStart(t *testing.T) {
	fileStateFolder := filepath.Join(t.TempDir(), "CloudWatchAgentTest")
	plugin := Plugin{
		FileStateFolder: fileStateFolder,
	}
	ec := EventConfig{
		LogGroupName:  "My  Group/:::",
		LogStreamName: "My::Stream//  ",
		Name:          "System  Event//Log::",
	}
	plugin.Events = append(plugin.Events, ec)
	require.Equal(t, 0, len(plugin.newEvents), "Start should be ran only once so there should be only 1 new event")
	plugin.Start(nil)
	require.Equal(t, 1, len(plugin.newEvents), "Start should be ran only once so there should be only 1 new event")
	plugin.Start(nil)
	require.Equal(t, 1, len(plugin.newEvents), "Start should be ran only once so there should be only 1 new event")
}

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
