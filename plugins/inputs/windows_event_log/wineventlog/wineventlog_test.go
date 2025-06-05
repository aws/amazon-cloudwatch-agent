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

	"github.com/aws/amazon-cloudwatch-agent/internal/logscommon"
	"github.com/aws/amazon-cloudwatch-agent/internal/state"
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
	LOG_GROUP_CLASS = "standard"
)

// TestNewEventLog verifies constructor's default values.
func TestNewEventLog(t *testing.T) {
	elog := newTestEventLog(t, NAME, LEVELS)
	assert.Equal(t, NAME, elog.name)
	assert.Equal(t, uint64(0), elog.eventOffset)
	assert.Zero(t, elog.eventHandle)
}

// TestOpen verifies Open() succeeds with valid inputs.
// And fails with invalid inputs.
func TestOpen(t *testing.T) {
	// Happy path.
	elog := newTestEventLog(t, NAME, LEVELS)
	assert.NoError(t, elog.Open())
	assert.NotZero(t, elog.eventHandle)
	assert.NoError(t, elog.Close())
	// Bad event log source name does not cause Open() to fail.
	// But eventHandle will be 0 and Close() will fail because of it.
	elog = newTestEventLog(t, "FakeBadElogName", LEVELS)
	assert.NoError(t, elog.Open())
	assert.Zero(t, elog.eventHandle)
	assert.Error(t, elog.Close())
	// bad LEVELS does not cause Open() to fail.
	elog = newTestEventLog(t, NAME, []string{"498"})
	assert.NoError(t, elog.Open())
	assert.NotZero(t, elog.eventHandle)
	assert.NoError(t, elog.Close())
	// bad wlog.eventOffset does not cause Open() to fail.
	elog = newTestEventLog(t, NAME, []string{"498"})
	elog.eventOffset = 9987
	assert.NoError(t, elog.Open())
	assert.NotZero(t, elog.eventHandle)
	assert.NoError(t, elog.Close())
}

// TestReadGoodSource will verify we can read events written by a registered
// event log source.
func TestReadGoodSource(t *testing.T) {
	elog := newTestEventLog(t, NAME, LEVELS)
	assert.NoError(t, elog.Open())
	seekToEnd(t, elog)
	writeEvents(t, 10, true, "CWA_UnitTest111", 777)
	records := readHelper(elog)
	checkEvents(t, records, "[Application] [ERROR] [777] [CWA_UnitTest111] ", 10)
	assert.NoError(t, elog.Close())
}

// TestReadBadSource will verify that we cannot read events written by an
// unregistered event log source.
func TestReadBadSource(t *testing.T) {
	elog := newTestEventLog(t, NAME, LEVELS)
	assert.NoError(t, elog.Open())
	seekToEnd(t, elog)
	writeEvents(t, 10, false, "CWA_UnitTest222", 888)
	records := readHelper(elog)
	checkEvents(t, records, "CWA_UnitTest222", 0)
	assert.NoError(t, elog.Close())
}

// TestReadWithBothSources will verify we can read events written by a
// registered event log source, even if the batch contains events from an
// unregistered source too.
func TestReadWithBothSources(t *testing.T) {
	elog := newTestEventLog(t, NAME, LEVELS)
	assert.NoError(t, elog.Open())
	seekToEnd(t, elog)
	writeEvents(t, 10, true, "CWA_UnitTest111", 777)
	writeEvents(t, 10, false, "CWA_UnitTest222", 888)
	records := readHelper(elog)
	checkEvents(t, records, "[Application] [ERROR] [777] [CWA_UnitTest111] ", 10)
	checkEvents(t, records, "CWA_UnitTest222", 0)
	assert.NoError(t, elog.Close())
}

// seekToEnd skips past all current events in the event log.
func seekToEnd(t *testing.T, elog *windowsEventLog) {
	// loop until we stop getting records.
	numRetrieved := 0
	// Max 100 loops as a safety check.
	for i := 0; i < 100; i++ {
		records := elog.read()
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
		wlog.Error(eventId, fmt.Sprintf("CWA_UnitTest event msg %v", i))
	}
	err = wlog.Close()
	assert.NoError(t, err)
	// Must sleep after wlog.Error() otherwise elog.read() will not see results.
	time.Sleep(1 * time.Second)
}

// readHelper reads all events (since last read).
func readHelper(elog *windowsEventLog) []*windowsEventLogRecord {
	var records []*windowsEventLogRecord
	// MAX 100 loops as a safety check.
	for i := 0; i < 100; i++ {
		currentRecords := elog.read()
		if len(currentRecords) == 0 {
			break
		}
		records = append(records, currentRecords...)
	}
	return records
}

// checkEvents counts the records matching substring and verifies against count.
func checkEvents(t *testing.T, records []*windowsEventLogRecord, substring string, count int) {
	// For each expected value, verify the count of matching events.
	found := 0
	for _, r := range records {
		eventMsg, err := r.Value()
		assert.NoError(t, err)
		if strings.Contains(eventMsg, substring) {
			found += 1
		}
	}
	assert.Equal(t, count, found, "expected %v, %v, actual %v", substring, count, found)
}

func newTestEventLog(t *testing.T, name string, levels []string) *windowsEventLog {
	t.Helper()
	manager := state.NewFileOffsetManager(state.ManagerConfig{
		StateFileDir:    t.TempDir(),
		StateFilePrefix: logscommon.WindowsEventLogPrefix,
		Name:            GROUP_NAME + "_" + STREAM_NAME + "_" + name,
	})
	return NewEventLog(name, levels, GROUP_NAME, STREAM_NAME, RENDER_FMT, DEST,
		manager, BATCH_SIZE, RETENTION, LOG_GROUP_CLASS)
}
