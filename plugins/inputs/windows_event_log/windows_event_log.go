// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package windows_event_log

import (
	"errors"
	"os"
	"sync"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"

	"github.com/aws/amazon-cloudwatch-agent/internal/logscommon"
	"github.com/aws/amazon-cloudwatch-agent/internal/state"
	"github.com/aws/amazon-cloudwatch-agent/logs"
	"github.com/aws/amazon-cloudwatch-agent/plugins/inputs/windows_event_log/wineventlog"
)

const (
	forcePullInterval = 250 * time.Millisecond
	stateQueueSize    = 100
)

var startOnlyOnce sync.Once

type EventConfig struct {
	Name          string   `toml:"event_name"`
	Levels        []string `toml:"event_levels"`
	EventIDs      []int    `toml:"event_ids"`
	RenderFormat  string   `toml:"event_format"`
	BatchReadSize int      `toml:"batch_read_size"`
	LogGroupName  string   `toml:"log_group_name"`
	LogStreamName string   `toml:"log_stream_name"`
	LogGroupClass string   `toml:"log_group_class"`
	Destination   string   `toml:"destination"`
	Retention     int      `toml:"retention_in_days"`
}
type Plugin struct {
	FileStateFolder string          `toml:"file_state_folder"`
	Events          []EventConfig   `toml:"event_config"`
	Destination     string          `toml:"destination"`
	MaxPersistState int             `toml:"max_persist_state"`
	Log             telegraf.Logger `toml:"-"`

	newEvents []logs.LogSrc
}

func (s *Plugin) Description() string {
	return "A plugin to collect Windows event logs"
}

func (s *Plugin) SampleConfig() string {
	return `
	file_state_folder = "c:\\path\\to\\state\\folder"

	[[inputs.windows_event_log.event_config]]
	event_name = "System"
	event_levels = ["2", "3"]
	event_ids = [1001, 1002]
	batch_read_size = 1
	log_group_name = "System"
	log_stream_name = "STREAM_NAME"
	destination = "cloudwatchlogs"
	`
}

func (s *Plugin) Gather(acc telegraf.Accumulator) (err error) {
	return nil
}

func (s *Plugin) FindLogSrc() []logs.LogSrc {
	events := s.newEvents
	s.newEvents = nil
	return events
}

/**
 * We can do any initialization in this method.
 */
func (s *Plugin) Start(acc telegraf.Accumulator) error {
	alreadyRan := true
	startOnlyOnce.Do(func() {
		alreadyRan = false
	})
	if alreadyRan {
		return nil
	}

	monitor := newServiceMonitor()
	for _, eventConfig := range s.Events {
		// Assume no 2 EventConfigs have the same combination of:
		// LogGroupName, LogStreamName, Name.
		stateManagerCfg, err := getStateManagerConfig(s, &eventConfig)
		if err != nil {
			return err
		}
		stateManager := state.NewFileRangeManager(stateManagerCfg)
		destination := eventConfig.Destination
		if destination == "" {
			destination = s.Destination
		}
		eventLog := wineventlog.NewEventLog(
			eventConfig.Name,
			eventConfig.Levels,
			eventConfig.EventIDs,
			eventConfig.LogGroupName,
			eventConfig.LogStreamName,
			eventConfig.RenderFormat,
			destination,
			stateManager,
			eventConfig.BatchReadSize,
			eventConfig.Retention,
			eventConfig.LogGroupClass,
		)
		err = eventLog.Init()
		if err != nil {
			return err
		}
		monitor.addListener(eventLog.ResubscribeCh())
		s.newEvents = append(s.newEvents, eventLog)
	}
	go monitor.start()
	return nil
}

// getStateManagerConfig returns a state.ManagerConfig with a unique name for a given EventConfig and windows event
// log specific prefix.
func getStateManagerConfig(plugin *Plugin, ec *EventConfig) (state.ManagerConfig, error) {
	var cfg state.ManagerConfig
	if plugin.FileStateFolder == "" {
		return cfg, errors.New("empty FileStateFolder")
	}
	err := os.MkdirAll(plugin.FileStateFolder, 0755)
	if err != nil {
		return cfg, err
	}
	return state.ManagerConfig{
		StateFileDir:      plugin.FileStateFolder,
		StateFilePrefix:   logscommon.WindowsEventLogPrefix,
		Name:              ec.LogGroupName + "_" + ec.LogStreamName + "_" + ec.Name,
		QueueSize:         stateQueueSize,
		MaxPersistedItems: max(1, plugin.MaxPersistState),
	}, nil
}

func (s *Plugin) Stop() {
}

func init() {
	inputs.Add("windows_event_log", func() telegraf.Input { return &Plugin{} })
}
