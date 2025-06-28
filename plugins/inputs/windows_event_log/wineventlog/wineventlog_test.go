// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package wineventlog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/windows/svc/eventlog"

	"github.com/aws/amazon-cloudwatch-agent/internal/logscommon"
	"github.com/aws/amazon-cloudwatch-agent/internal/state"
	"github.com/aws/amazon-cloudwatch-agent/logs"
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

func TestReadGaps(t *testing.T) {
	t.Run("BasicGapReading", func(t *testing.T) {
		t.Parallel()

		rl := state.RangeList{
			state.NewRange(0, 100),
			state.NewRange(105, 107),
		}
		elog, stateFileName := newTestEventLogWithState(t, NAME, LEVELS, rl)
		mockAPI := NewMockWindowsEventAPI()
		elog.SetEventAPI(mockAPI)

		mockAPI.AddMockEvents(createMockEventRecordsRange(100, 105))

		elog.Init()

		var records []logs.LogEvent
		// SetOutput calls run as well hence the omission of elog.run()
		elog.SetOutput(func(e logs.LogEvent) {
			records = append(records, e)
			e.Done()
		})
		time.Sleep(5 * time.Second)
		elog.Stop()

		assert.Empty(t, elog.gapsToRead, "Gaps should be cleared after reading")
		assert.Len(t, records, 5, "Should return 5 mock events")
		assert.Len(t, mockAPI.QueryCalls, 1, "Should make one query call")
		assert.Equal(t, NAME, mockAPI.QueryCalls[0].Path, "Should query correct path")
		assert.Contains(t, mockAPI.QueryCalls[0].Query, "EventRecordID &gt;= 100", "Query should contain start range")
		assert.Contains(t, mockAPI.QueryCalls[0].Query, "EventRecordID &lt; 105", "Query should contain end range")
		assert.Greater(t, len(mockAPI.CloseCalls), 0, "Should make close calls")

		for i, record := range records {
			assert.Contains(t, record.Message(), fmt.Sprintf("Event %d", 100+i))
		}

		assertStateFileRange(t, stateFileName, state.RangeList{
			state.NewRange(0, 107),
		})
	})
	t.Run("ReadGapThenSubscribe", func(t *testing.T) {
		t.Parallel()

		rl := state.RangeList{
			state.NewRange(0, 2),
			state.NewRange(4, 5),
		}
		elog, stateFileName := newTestEventLogWithState(t, NAME, LEVELS, rl)
		mockAPI := NewMockWindowsEventAPI()
		elog.SetEventAPI(mockAPI)

		// This is per EvtHandle hence the necessity to break up these calls
		// 0, 1, 4 were "sent" previously (should be skipped)
		mockAPI.AddMockEventsForQuery(createMockEventRecords(0, 1, 4))
		// Gap records (should be read by gap reading)
		mockAPI.AddMockEventsForQuery(createMockEventRecords(2, 3))

		elog.Init()

		// Simulate new subscription events arriving
		mockAPI.SimulateSubscriptionEvents(createMockEventRecords(5, 6, 7, 8))

		var records []logs.LogEvent
		// SetOutput calls run as well hence the omission of elog.run()
		elog.SetOutput(func(e logs.LogEvent) {
			records = append(records, e)
			e.Done()
		})
		time.Sleep(5 * time.Second)
		elog.Stop()

		assert.Empty(t, elog.gapsToRead, "Gaps should be cleared after reading")
		assert.Len(t, records, 6, "Should return 2 mock events")
		assert.Len(t, mockAPI.QueryCalls, 1, "Should make one query call")
		assert.Equal(t, NAME, mockAPI.QueryCalls[0].Path, "Should query correct path")
		assert.Contains(t, mockAPI.QueryCalls[0].Query, "EventRecordID &gt;= 2", "Query should contain start range")
		assert.Contains(t, mockAPI.QueryCalls[0].Query, "EventRecordID &lt; 4", "Query should contain end range")
		assert.Len(t, mockAPI.SubscribeCalls, 1, "Should make one subscribe call")
		assert.Greater(t, len(mockAPI.CloseCalls), 0, "Should make close calls")

		expectedRecords := []int{
			2, 3, 5, 6, 7, 8,
		}
		for i, record := range records {
			assert.Contains(t, record.Message(), fmt.Sprintf("Event %d", expectedRecords[i]))
		}

		// Surprisingly, the end offset will be 8 instead of 9. This is because Windows record IDs start at 1
		assertStateFileRange(t, stateFileName, state.RangeList{
			state.NewRange(0, 8),
		})
	})
	t.Run("NoGaps", func(t *testing.T) {
		t.Parallel()

		rl := state.RangeList{
			state.NewRange(0, 5),
		}
		elog, stateFileName := newTestEventLogWithState(t, NAME, LEVELS, rl)
		mockAPI := NewMockWindowsEventAPI()
		elog.SetEventAPI(mockAPI)

		mockAPI.AddMockEvents(createMockEventRecordsRange(0, 5))

		elog.Init()

		mockAPI.SimulateSubscriptionEvents(createMockEventRecordsRange(5, 9))

		var records []logs.LogEvent
		// SetOutput calls run as well hence the omission of elog.run()
		elog.SetOutput(func(e logs.LogEvent) {
			records = append(records, e)
			e.Done()
		})
		time.Sleep(5 * time.Second)
		elog.Stop()

		assert.Empty(t, elog.gapsToRead, "There should be no gaps")
		assert.Len(t, records, 4, "Should return 4 mock events")
		assert.Len(t, mockAPI.QueryCalls, 0, "Should make one query call")
		assert.Len(t, mockAPI.SubscribeCalls, 1, "Should make one subscribe call")
		assert.Greater(t, len(mockAPI.CloseCalls), 0, "Should make close calls")

		for i, record := range records {
			// Offset by 5 since we "already read" events 0-4
			assert.Contains(t, record.Message(), fmt.Sprintf("Event %d", 5+i))
		}

		assertStateFileRange(t, stateFileName, state.RangeList{
			state.NewRange(0, 8),
		})
	})
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

func marshalRangeList(rl state.RangeList) string {
	var marshalledRanges []string
	for _, r := range rl {
		text, _ := r.MarshalText()
		marshalledRanges = append(marshalledRanges, string(text))
	}
	return strings.Join(marshalledRanges, ",")
}

func assertStateFileRange(t *testing.T, fileName string, rl state.RangeList) {
	content, _ := os.ReadFile(fileName)
	assert.Contains(t, string(content), marshalRangeList(rl))
}

func createMockEventRecordsRange(start, end int) []*MockEventRecord {
	var records []*MockEventRecord
	for i := start; i < end; i++ {
		records = append(records, &MockEventRecord{
			EventRecordID: fmt.Sprintf("%d", i),
			TimeCreated:   time.Now(),
			Level:         "2",
			Provider:      "TestProvider",
			Message:       fmt.Sprintf("Event %d", i),
			Channel:       "Application",
		})
	}
	return records
}

func createMockEventRecords(recordIds ...int) []*MockEventRecord {
	var records []*MockEventRecord
	for _, id := range recordIds {
		records = append(records, &MockEventRecord{
			EventRecordID: fmt.Sprintf("%d", id),
			TimeCreated:   time.Now(),
			Level:         "2",
			Provider:      "TestProvider",
			Message:       fmt.Sprintf("Event %d", id),
			Channel:       "Application",
		})
	}
	return records
}

func newTestEventLogWithState(t *testing.T, name string, levels []string, rl state.RangeList) (*windowsEventLog, string) {
	t.Helper()
	tempDir := t.TempDir()
	file, _ := os.CreateTemp(tempDir, "")
	content := fmt.Sprintf("%d\n%s\n%s", rl.Last().EndOffset(), name, marshalRangeList(rl))
	file.WriteString(content)
	file.Close()

	manager := state.NewFileRangeManager(state.ManagerConfig{
		StateFileDir:      tempDir,
		StateFilePrefix:   "",
		Name:              filepath.Base(file.Name()),
		MaxPersistedItems: 10,
	})
	return NewEventLog(name, levels, GROUP_NAME, STREAM_NAME, RENDER_FMT, DEST,
		manager, BATCH_SIZE, RETENTION, LOG_GROUP_CLASS), file.Name()
}

func newTestEventLog(t *testing.T, name string, levels []string) *windowsEventLog {
	t.Helper()
	manager := state.NewFileRangeManager(state.ManagerConfig{
		StateFileDir:    t.TempDir(),
		StateFilePrefix: logscommon.WindowsEventLogPrefix,
		Name:            GROUP_NAME + "_" + STREAM_NAME + "_" + name,
	})
	return NewEventLog(name, levels, GROUP_NAME, STREAM_NAME, RENDER_FMT, DEST,
		manager, BATCH_SIZE, RETENTION, LOG_GROUP_CLASS)
}
