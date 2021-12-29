// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package wineventlog

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/windows/svc/eventlog"
)

var (
	// common inputs for creating an EventLog.
	NAME = "Application"
	// 2 is ERROR
	LEVELS          = []string{"2"}
	GROUP_NAME      = "fake"
	STREAM_NAME     = "fake"
	RENDER_FMT      = FormatPlainText
	DEST            = "fake"
	STATE_FILE_PATH = "fake"
	BATCH_SIZE      = 99
	RETENTION       = 42
)

// TestNewEventLog verifies constructor's default values.
func TestNewEventLog(t *testing.T) {
	elog := NewEventLog(NAME, LEVELS, GROUP_NAME, STREAM_NAME, RENDER_FMT, DEST,
		STATE_FILE_PATH, BATCH_SIZE, RETENTION)
	assert.Equal(t, NAME, elog.name)
	assert.Equal(t, uint64(0), elog.eventOffset)
	assert.Zero(t, elog.eventHandle)
}

// TestOpen verifies Open() succeeds with valid inputs.
// And fails with invalid inputs.
func TestOpen(t *testing.T) {
	// happy
	elog := NewEventLog(NAME, LEVELS, GROUP_NAME, STREAM_NAME, RENDER_FMT, DEST,
		STATE_FILE_PATH, BATCH_SIZE, RETENTION)
	assert.NoError(t, elog.Open())
	assert.NotNil(t, elog.eventHandle)
	assert.NoError(t, elog.Close())
	// bad name
	elog = NewEventLog("FakeBadElogName", LEVELS, GROUP_NAME, STREAM_NAME,
		RENDER_FMT, DEST, STATE_FILE_PATH, BATCH_SIZE, RETENTION)
	assert.Error(t, elog.Open())
	assert.Zero(t, elog.eventHandle)
	// bad LEVELS does not cause Open() to fail.
	// bad wlog.eventOffset does not cause Open() to fail.
}

// TestReadGoodSource will verify we can read events written by a registered
// event log source.
func TestReadGoodSource(t *testing.T) {
	elog := NewEventLog(NAME, LEVELS, GROUP_NAME, STREAM_NAME, RENDER_FMT, DEST,
		STATE_FILE_PATH, BATCH_SIZE, RETENTION)
	assert.Equal(t, nil, elog.Open())
	seekToEnd(t, elog)
	writeEvents(t, 10, true, "CWA_UnitTest111", 777)
	// Without a sleep we do not read any events!!!
	time.Sleep(1 * time.Second)
	checkEvents(t, elog, 10, "[Application] [ERROR] [777] [CWA_UnitTest111] ")
	assert.Equal(t, nil, elog.Close())
}

// TestReadBadSource will verify that we cannot read events written by an
// unregistered event log source.
func TestReadBadSource(t *testing.T) {
	elog := NewEventLog(NAME, LEVELS, GROUP_NAME, STREAM_NAME, RENDER_FMT, DEST,
		STATE_FILE_PATH, BATCH_SIZE, RETENTION)
	assert.Equal(t, nil, elog.Open())
	seekToEnd(t, elog)
	writeEvents(t, 10, false, "CWA_UnitTest222", 888)
	time.Sleep(1 * time.Second)
	checkEvents(t, elog, 0, "[Application] [ERROR] [888] [CWA_UnitTest222] ")
	assert.Equal(t, nil, elog.Close())
}

// TestReadWithBothSources will verify we can read events written by a
// registered event log source, even if the batch contains events from an
// unregistered source too.
func TestReadWithBothSources(t *testing.T) {
	elog := NewEventLog(NAME, LEVELS, GROUP_NAME, STREAM_NAME, RENDER_FMT, DEST,
		STATE_FILE_PATH, BATCH_SIZE, RETENTION)
	assert.Equal(t, nil, elog.Open())
	seekToEnd(t, elog)
	writeEvents(t, 10, true, "CWA_UnitTest111", 777)
	writeEvents(t, 10, false, "CWA_UnitTest222", 888)
	time.Sleep(1 * time.Second)
	checkEvents(t, elog, 10, "[Application] [ERROR] [777] [CWA_UnitTest111] ")
	assert.Equal(t, nil, elog.Close())
}

// seekToEnd skips past all current events in the event log.
func seekToEnd(t *testing.T, elog *windowsEventLog) {
	// loop until we stop getting records.
	numRetrieved := 0
	for {
		records := elog.read()
		//assert.Equal(t, nil, err)
		t.Logf("seekToEnd() current count %v", len(records))
		numRetrieved += len(records)
		if len(records) == 0 {
			break
		}
	}
	t.Logf("seekToEnd() total %v", numRetrieved)
}

// writeEvents writes msgCount number of events to the Application event log.
// Optionally register the given logSrc.
// Fail the test if an error occurs.
func writeEvents(t *testing.T, msgCount int, doRegister bool, logSrc string, eventId uint32) {
	if doRegister {
		// Expected to fail if unit test previously ran and installed the event src.
		_ = eventlog.InstallAsEventCreate(logSrc, eventlog.Info|eventlog.Warning|eventlog.Error)
	}
	wlog, err := eventlog.Open(logSrc)
	assert.NoError(t, err)
	for i := 0; i < msgCount; i++ {
		msg := fmt.Sprintf("CWA_UnitTest event msg %v", i)
		wlog.Error(eventId, msg)
	}
	err = wlog.Close()
	assert.NoError(t, err)
}

// checkEvents reads all events (since last read) then checks each one for the
// given substr. Verify count == msgCount.
func checkEvents(t *testing.T, elog *windowsEventLog, msgCount int, substr string) {
	// Get all new records.
	var records []*windowsEventLogRecord
	for {
		// In the old code read() would return an error if there were events
		// from an unregistered provider.
		currentRecords := elog.read()
		//assert.Equal(t, nil, err)
		t.Logf("checkEvents() current count %v", len(currentRecords))
		if len(currentRecords) == 0 {
			break
		}
		records = append(records, currentRecords...)
	}
	t.Logf("checkEvents() total count %v", len(records))
	// Check all new records.
	found := 0
	for _, r := range records {
		eventMsg, err := r.Value()
		assert.NoError(t, err)
		if strings.Contains(eventMsg, substr) {
			found += 1
		}
	}
	assert.Equal(t, msgCount, found)
}
