// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package wineventlog

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

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
	EVENTIDS        = []int{777}
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
	elog := newTestEventLog(t, NAME, LEVELS, EVENTIDS)
	assert.Equal(t, NAME, elog.name)
	assert.Equal(t, uint64(0), elog.eventOffset)
	assert.Zero(t, elog.eventHandle)
}

// TestOpen verifies Open() succeeds with valid inputs.
// And fails with invalid inputs.
func TestOpen(t *testing.T) {
	// Happy path.
	elog := newTestEventLog(t, NAME, LEVELS, EVENTIDS)
	assert.NoError(t, elog.Open())
	assert.NotZero(t, elog.eventHandle)
	assert.NoError(t, elog.Close())
	// Bad event log source name does not cause Open() to fail.
	// But eventHandle will be 0 and Close() will fail because of it.
	elog = newTestEventLog(t, "FakeBadElogName", LEVELS, EVENTIDS)
	assert.NoError(t, elog.Open())
	assert.Zero(t, elog.eventHandle)
	assert.Error(t, elog.Close())
	// bad LEVELS does not cause Open() to fail.
	elog = newTestEventLog(t, NAME, []string{"498"}, EVENTIDS)
	assert.NoError(t, elog.Open())
	assert.NotZero(t, elog.eventHandle)
	assert.NoError(t, elog.Close())
	elog = newTestEventLog(t, NAME, LEVELS, []int{98698})
	assert.NoError(t, elog.Open())
	assert.NotZero(t, elog.eventHandle)
	assert.NoError(t, elog.Close())
	// bad wlog.eventOffset does not cause Open() to fail.
	elog = newTestEventLog(t, NAME, []string{"498"}, EVENTIDS)
	elog.eventOffset = 9987
	assert.NoError(t, elog.Open())
	assert.NotZero(t, elog.eventHandle)
	assert.NoError(t, elog.Close())
}

// TestReadGoodSource will verify we can read events written by a registered
// event log source.
func TestReadGoodSource(t *testing.T) {
	elog := newTestEventLog(t, NAME, LEVELS, EVENTIDS)
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
	elog := newTestEventLog(t, NAME, LEVELS, EVENTIDS)
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
	elog := newTestEventLog(t, NAME, LEVELS, EVENTIDS)
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
	originalWinEventApi := winEventAPI
	defer func() {
		winEventAPI = originalWinEventApi
	}()

	t.Run("BasicGapReading", func(t *testing.T) {
		rl := state.RangeList{
			// Inverted range: 0-5, 10-15, 20-25, 30-inf
			state.NewRange(5, 10),
			state.NewRange(15, 20),
			state.NewRange(25, 30),
		}
		elog, stateFileName := newTestEventLogWithState(t, NAME, LEVELS, rl)
		mockAPI := NewMockWindowsEventAPI()
		winEventAPI = mockAPI

		mockAPI.AddMockEventsForQuery(createMockEventRecordsRange(0, 5))
		mockAPI.AddMockEventsForQuery(createMockEventRecordsRange(10, 15))
		mockAPI.AddMockEventsForQuery(createMockEventRecordsRange(20, 25))

		elog.Init()

		var records []logs.LogEvent
		// SetOutput calls run as well hence the omission of elog.run()
		elog.SetOutput(func(e logs.LogEvent) {
			records = append(records, e)
			e.Done()
		})
		time.Sleep(8 * time.Second)
		elog.Stop()

		expectedRecords := []int{
			1, 2, 3, 4, 5, 11, 12, 13, 14, 15, 21, 22, 23, 24, 25,
		}

		assert.Empty(t, elog.gapsToRead, "Gaps should be cleared after reading")
		assert.Len(t, records, len(expectedRecords), "Should return correct number of mock events")
		assert.Len(t, mockAPI.QueryCalls, 3, "Should make three query calls")
		assert.Equal(t, NAME, mockAPI.QueryCalls[0].Path, "Should query correct path")
		assert.Contains(t, mockAPI.QueryCalls[0].Query, "EventRecordID &gt; 0", "Query should contain start range")
		assert.Contains(t, mockAPI.QueryCalls[0].Query, "EventRecordID &lt;= 5", "Query should contain end range")
		assert.Contains(t, mockAPI.QueryCalls[1].Query, "EventRecordID &gt; 10", "Query should contain start range")
		assert.Contains(t, mockAPI.QueryCalls[1].Query, "EventRecordID &lt;= 15", "Query should contain end range")
		assert.Contains(t, mockAPI.QueryCalls[2].Query, "EventRecordID &gt; 20", "Query should contain start range")
		assert.Contains(t, mockAPI.QueryCalls[2].Query, "EventRecordID &lt;= 25", "Query should contain end range")
		assert.Greater(t, len(mockAPI.CloseCalls), 0, "Should make close calls")

		for i, record := range records {
			assert.Contains(t, record.Message(), fmt.Sprintf("Event %d", expectedRecords[i]))
		}

		assertStateFileRange(t, stateFileName, state.RangeList{
			state.NewRange(0, 30),
		})
	})
	t.Run("ReadGapThenSubscribe", func(t *testing.T) {
		rl := state.RangeList{
			// Inverted range: 2-4
			state.NewRange(0, 2),
			state.NewRange(4, 5),
		}
		elog, stateFileName := newTestEventLogWithState(t, NAME, LEVELS, rl)
		mockAPI := NewMockWindowsEventAPI()
		winEventAPI = mockAPI

		// This is per EvtHandle hence the necessity to break up these calls
		// 1, 2, 5 were "sent" previously (should be skipped)
		mockAPI.AddMockEventsForQuery(createMockEventRecords(1, 2, 5))
		// Gap records (should be read by gap reading)
		mockAPI.AddMockEventsForQuery(createMockEventRecords(3, 4))

		elog.Init()

		// Simulate new subscription events arriving
		mockAPI.SimulateSubscriptionEvents(createMockEventRecords(6, 7, 8))

		var records []logs.LogEvent
		// SetOutput calls run as well hence the omission of elog.run()
		elog.SetOutput(func(e logs.LogEvent) {
			records = append(records, e)
			e.Done()
		})
		time.Sleep(5 * time.Second)
		elog.Stop()

		expectedRecords := []int{
			3, 4, 6, 7, 8,
		}

		assert.Empty(t, elog.gapsToRead, "Gaps should be cleared after reading")
		assert.Len(t, records, len(expectedRecords), "Should return correct number of mock events")
		assert.Len(t, mockAPI.QueryCalls, 1, "Should make one query call")
		assert.Equal(t, NAME, mockAPI.QueryCalls[0].Path, "Should query correct path")
		assert.Contains(t, mockAPI.QueryCalls[0].Query, "EventRecordID &gt; 2", "Query should contain start range")
		assert.Contains(t, mockAPI.QueryCalls[0].Query, "EventRecordID &lt;= 4", "Query should contain end range")
		assert.Len(t, mockAPI.SubscribeCalls, 1, "Should make one subscribe call")
		assert.Greater(t, len(mockAPI.CloseCalls), 0, "Should make close calls")

		for i, record := range records {
			assert.Contains(t, record.Message(), fmt.Sprintf("Event %d", expectedRecords[i]))
		}

		// Surprisingly, the end offset will be 8 instead of 9. This is because Windows record IDs start at 1
		assertStateFileRange(t, stateFileName, state.RangeList{
			state.NewRange(0, 8),
		})
	})
	t.Run("NoGaps", func(t *testing.T) {
		rl := state.RangeList{
			state.NewRange(0, 5),
		}
		elog, stateFileName := newTestEventLogWithState(t, NAME, LEVELS, rl)
		mockAPI := NewMockWindowsEventAPI()
		winEventAPI = mockAPI

		mockAPI.AddMockEventsForQuery(createMockEventRecordsRange(0, 5))

		elog.Init()

		mockAPI.SimulateSubscriptionEvents(createMockEventRecordsRange(6, 9))

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
		assert.Len(t, mockAPI.QueryCalls, 0, "Should not make a query call")
		assert.Len(t, mockAPI.SubscribeCalls, 1, "Should make one subscribe call")
		assert.Greater(t, len(mockAPI.CloseCalls), 0, "Should make close calls")

		for i, record := range records {
			// Offset by 5 since we "already read" events 0-4
			assert.Contains(t, record.Message(), fmt.Sprintf("Event %d", 6+i))
		}

		assertStateFileRange(t, stateFileName, state.RangeList{
			state.NewRange(0, 9),
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
		err = wlog.Error(eventId, fmt.Sprintf("CWA_UnitTest event msg %v", i))
		assert.NoError(t, err)
	}
	err = wlog.Close()
	assert.NoError(t, err)
	// Must sleep after wlog.Error() otherwise elog.read() will not see results.
	time.Sleep(3 * time.Second)
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

// Start and end are both inclusive
func createMockEventRecordsRange(start, end int) []*MockEventRecord {
	var records []*MockEventRecord
	for i := start; i <= end; i++ {
		records = append(records, &MockEventRecord{
			EventRecordID: fmt.Sprintf("%d", i),
			TimeCreated:   time.Now(),
			Level:         "2",
			EventID:       100,
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
			EventID:       100,
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
	return NewEventLog(name, levels, EVENTIDS, GROUP_NAME, STREAM_NAME, RENDER_FMT, DEST,
		manager, BATCH_SIZE, RETENTION, LOG_GROUP_CLASS), file.Name()
}

func newTestEventLog(t *testing.T, name string, levels []string, eventids []int) *windowsEventLog {
	t.Helper()
	manager := state.NewFileRangeManager(state.ManagerConfig{
		StateFileDir:    t.TempDir(),
		StateFilePrefix: logscommon.WindowsEventLogPrefix,
		Name:            GROUP_NAME + "_" + STREAM_NAME + "_" + name,
	})
	return NewEventLog(name, levels, eventids, GROUP_NAME, STREAM_NAME, RENDER_FMT, DEST,
		manager, BATCH_SIZE, RETENTION, LOG_GROUP_CLASS)
}

// MockWindowsEventAPI provides a mock implementation for testing
type MockWindowsEventAPI struct {
	// Mock data storage - separated by purpose
	subscriptionEvents map[EvtHandle][]*MockEventRecord // Events for subscription handles (EvtSubscribe)
	queryEvents        map[EvtHandle][]*MockEventRecord // Events for query handles (EvtQuery)
	nextHandle         EvtHandle

	// Behavior configuration
	shouldFailSubscribe     bool
	shouldFailQuery         bool
	shouldFailNext          bool
	shouldFailClose         bool
	shouldFailRender        bool
	shouldFailBookmark      bool
	shouldFailFormatMessage bool
	shouldFailOpenPublisher bool
	subscribeFailError      error
	queryFailError          error
	nextFailError           error
	closeFailError          error
	renderFailError         error
	bookmarkFailError       error
	formatMessageFailError  error
	openPublisherFailError  error

	// Call tracking
	SubscribeCalls     []SubscribeCall
	QueryCalls         []QueryCall
	NextCalls          []NextCall
	CloseCalls         []CloseCall
	RenderCalls        []RenderCall
	BookmarkCalls      []BookmarkCall
	FormatMessageCalls []FormatMessageCall
	OpenPublisherCalls []OpenPublisherCall
}

type SubscribeCall struct {
	Session     EvtHandle
	SignalEvent uintptr
	ChannelPath string
	Query       string
	Bookmark    EvtHandle
	Context     uintptr
	Callback    syscall.Handle
	Flags       EvtSubscribeFlag
}

type BookmarkCall struct {
	Path   string
	Offset uint64
}

type QueryCall struct {
	Session EvtHandle
	Path    string
	Query   string
	Flags   EvtQueryFlag
	Range   state.Range // Extracted from query
}

type NextCall struct {
	ResultSet      EvtHandle
	EventArraySize uint32
	Timeout        uint32
	Flags          uint32
}

type CloseCall struct {
	Handle EvtHandle
}

type RenderCall struct {
	Context       EvtHandle
	Fragment      EvtHandle
	Flags         EvtRenderFlag
	BufferSize    uint32
	BufferUsed    uint32
	PropertyCount uint32
}

type FormatMessageCall struct {
	PublisherMetadata EvtHandle
	Event             EvtHandle
	MessageID         uint32
	ValueCount        uint32
	Values            uintptr
	Flags             EvtFormatMessageFlag
	BufferSize        uint32
	BufferUsed        uint32
}

type OpenPublisherCall struct {
	Session     EvtHandle
	PublisherID string
	LogFilePath string
	Locale      uint32
	Flags       uint32
}

type MockEventRecord struct {
	EventRecordID string
	TimeCreated   time.Time
	Level         string
	EventID       int
	Provider      string
	Message       string
	Channel       string
}

func NewMockWindowsEventAPI() *MockWindowsEventAPI {
	return &MockWindowsEventAPI{
		subscriptionEvents: make(map[EvtHandle][]*MockEventRecord),
		queryEvents:        make(map[EvtHandle][]*MockEventRecord),
		nextHandle:         1000, // Start with a high number to avoid conflicts
	}
}

// AddMockEventsForSubscription adds events that will be returned by subscription reading
func (m *MockWindowsEventAPI) AddMockEventsForSubscription(events []*MockEventRecord) {
	handle := m.nextHandle
	m.nextHandle++
	m.subscriptionEvents[handle] = events
}

// SimulateSubscriptionEvents simulates new events arriving on the subscription
// This should be called after Init() to trigger the subscription callback
func (m *MockWindowsEventAPI) SimulateSubscriptionEvents(events []*MockEventRecord) {
	// Find the subscription handle (the one created by EvtSubscribe)
	for handle, existingEvents := range m.subscriptionEvents {
		if len(existingEvents) == 0 {
			// This is the subscription handle, add events to it
			m.subscriptionEvents[handle] = events
			return
		}
	}

	// If no empty subscription handle found, create a new one
	handle := m.nextHandle
	m.nextHandle++
	m.subscriptionEvents[handle] = events
}

// AddMockEventsForQuery adds events that will be returned by query reading (gaps)
func (m *MockWindowsEventAPI) AddMockEventsForQuery(events []*MockEventRecord) {
	handle := m.nextHandle
	m.nextHandle++
	m.queryEvents[handle] = events
}

// Configuration methods
func (m *MockWindowsEventAPI) SetSubscribeFailure(shouldFail bool, err error) {
	m.shouldFailSubscribe = shouldFail
	m.subscribeFailError = err
}

func (m *MockWindowsEventAPI) SetQueryFailure(shouldFail bool, err error) {
	m.shouldFailQuery = shouldFail
	m.queryFailError = err
}

func (m *MockWindowsEventAPI) SetNextFailure(shouldFail bool, err error) {
	m.shouldFailNext = shouldFail
	m.nextFailError = err
}

func (m *MockWindowsEventAPI) SetCloseFailure(shouldFail bool, err error) {
	m.shouldFailClose = shouldFail
	m.closeFailError = err
}

func (m *MockWindowsEventAPI) SetRenderFailure(shouldFail bool, err error) {
	m.shouldFailRender = shouldFail
	m.renderFailError = err
}

func (m *MockWindowsEventAPI) SetBookmarkFailure(shouldFail bool, err error) {
	m.shouldFailBookmark = shouldFail
	m.bookmarkFailError = err
}

func (m *MockWindowsEventAPI) SetFormatMessageFailure(shouldFail bool, err error) {
	m.shouldFailFormatMessage = shouldFail
	m.formatMessageFailError = err
}

func (m *MockWindowsEventAPI) SetOpenPublisherFailure(shouldFail bool, err error) {
	m.shouldFailOpenPublisher = shouldFail
	m.openPublisherFailError = err
}

// Interface implementation
func (m *MockWindowsEventAPI) EvtSubscribe(session EvtHandle, signalEvent uintptr, channelPath *uint16, query *uint16, bookmark EvtHandle, context uintptr, callback syscall.Handle, flags EvtSubscribeFlag) (EvtHandle, error) {
	pathStr := ""
	if channelPath != nil {
		pathStr = utf16PtrToString(channelPath)
	}

	queryStr := ""
	if query != nil {
		queryStr = utf16PtrToString(query)
	}

	call := SubscribeCall{
		Session:     session,
		SignalEvent: signalEvent,
		ChannelPath: pathStr,
		Query:       queryStr,
		Bookmark:    bookmark,
		Context:     context,
		Callback:    callback,
		Flags:       flags,
	}
	m.SubscribeCalls = append(m.SubscribeCalls, call)

	if m.shouldFailSubscribe {
		return 0, m.subscribeFailError
	}

	// Return a mock subscription handle
	handle := m.nextHandle
	m.nextHandle++

	// Initialize with empty events for subscription (events will be added later)
	m.subscriptionEvents[handle] = []*MockEventRecord{}

	return handle, nil
}

func (m *MockWindowsEventAPI) EvtQuery(session EvtHandle, path *uint16, query *uint16, flags EvtQueryFlag) (EvtHandle, error) {
	pathStr := ""
	if path != nil {
		pathStr = utf16PtrToString(path)
	}

	queryStr := ""
	if query != nil {
		queryStr = utf16PtrToString(query)
	}

	// Extract range from query string for tracking
	r := m.extractRangeFromQuery(queryStr)

	call := QueryCall{
		Session: session,
		Path:    pathStr,
		Query:   queryStr,
		Flags:   flags,
		Range:   r,
	}
	m.QueryCalls = append(m.QueryCalls, call)

	if m.shouldFailQuery {
		return 0, m.queryFailError
	}

	// Find or create a handle with events for this range
	handle := m.findOrCreateHandleForRange(r)
	return handle, nil
}

func (m *MockWindowsEventAPI) EvtNext(resultSet EvtHandle, eventArraySize uint32, eventArray *EvtHandle, timeout uint32, flags uint32, numReturned *uint32) error {
	call := NextCall{
		ResultSet:      resultSet,
		EventArraySize: eventArraySize,
		Timeout:        timeout,
		Flags:          flags,
	}
	m.NextCalls = append(m.NextCalls, call)

	if m.shouldFailNext {
		return m.nextFailError
	}

	// Get mock events for this handle
	events, exists := m.getEventsForHandle(resultSet)
	if !exists || len(events) == 0 {
		*numReturned = 0
		return nil
	}

	// Return up to eventArraySize events
	numToReturn := uint32(len(events))
	if numToReturn > eventArraySize {
		numToReturn = eventArraySize
	}

	// Create event handles for the events
	eventHandles := (*[1024]EvtHandle)(unsafe.Pointer(eventArray))[:numToReturn]
	for i := uint32(0); i < numToReturn; i++ {
		eventHandles[i] = m.nextHandle
		m.nextHandle++
		// Store the event data associated with this handle
		m.setEventsForHandle(eventHandles[i], []*MockEventRecord{events[i]})
	}

	*numReturned = numToReturn

	// Remove returned events from the handle's event list
	m.setEventsForHandle(resultSet, events[numToReturn:])

	return nil
}

func (m *MockWindowsEventAPI) EvtClose(handle EvtHandle) error {
	call := CloseCall{Handle: handle}
	m.CloseCalls = append(m.CloseCalls, call)

	if m.shouldFailClose {
		return m.closeFailError
	}

	// Clean up mock data for this handle
	m.deleteHandle(handle)
	return nil
}

func (m *MockWindowsEventAPI) EvtRender(context EvtHandle, fragment EvtHandle, flags EvtRenderFlag, bufferSize uint32, buffer *byte, bufferUsed *uint32, propertyCount *uint32) error {
	call := RenderCall{
		Context:       context,
		Fragment:      fragment,
		Flags:         flags,
		BufferSize:    bufferSize,
		BufferUsed:    0, // Will be set below
		PropertyCount: 0, // Will be set below
	}
	m.RenderCalls = append(m.RenderCalls, call)

	if m.shouldFailRender {
		return m.renderFailError
	}

	// Get the mock event for this handle
	events, exists := m.getEventsForHandle(fragment)
	if !exists || len(events) == 0 {
		*bufferUsed = 0
		*propertyCount = 0
		return fmt.Errorf("no mock event for handle %d", fragment)
	}

	event := events[0]

	// Create a much simpler XML that matches the expected structure
	var xmlContent string
	if flags == EvtRenderEventXml {
		xmlContent = fmt.Sprintf(`<Event xmlns='http://schemas.microsoft.com/win/2004/08/events/event'><System><Provider Name='%s'/><EventID>4625</EventID><Version>0</Version><Level>%s</Level><Task>12544</Task><Opcode>0</Opcode><Keywords>0x8010000000000000</Keywords><TimeCreated SystemTime='%s'/><EventRecordID>%s</EventRecordID><Channel>%s</Channel><Computer>TEST-COMPUTER</Computer></System><EventData><Data Name='Message'>%s</Data></EventData></Event>`,
			event.Provider,
			event.Level,
			event.TimeCreated.Format("2006-01-02T15:04:05.000000000Z"),
			event.EventRecordID,
			event.Channel,
			event.Message)
	} else {
		// For other render flags, return simple content
		xmlContent = fmt.Sprintf("Event %s: %s", event.EventRecordID, event.Message)
	}

	// Convert to UTF-16 as Windows APIs return UTF-16 encoded data
	utf16Data, err := syscall.UTF16FromString(xmlContent)
	if err != nil {
		return fmt.Errorf("failed to convert to UTF-16: %v", err)
	}

	// Convert UTF-16 slice to bytes (little-endian)
	contentBytes := make([]byte, len(utf16Data)*2)
	for i, r := range utf16Data {
		contentBytes[i*2] = byte(r)
		contentBytes[i*2+1] = byte(r >> 8)
	}

	// Set the buffer used size (in bytes, not UTF-16 characters)
	*bufferUsed = uint32(len(contentBytes))
	*propertyCount = 1

	if len(contentBytes) > int(bufferSize) {
		// Return error indicating buffer too small, but still set bufferUsed
		return fmt.Errorf("buffer too small, need %d bytes", len(contentBytes))
	}

	// Create a slice from the buffer pointer with the actual buffer size
	maxSize := int(bufferSize)
	if maxSize > 1<<20 { // Cap at 1MB for safety
		maxSize = 1 << 20
	}
	bufferSlice := (*[1 << 20]byte)(unsafe.Pointer(buffer))[:maxSize]
	copy(bufferSlice, contentBytes)

	return nil
}

func (m *MockWindowsEventAPI) EvtCreateBookmark(bookmarkXML *uint16) (EvtHandle, error) {
	bookmarkStr := ""
	if bookmarkXML != nil {
		bookmarkStr = utf16PtrToString(bookmarkXML)
	}

	call := BookmarkCall{
		Path:   bookmarkStr, // Store the XML string in Path field for tracking
		Offset: 0,           // Not used for EvtCreateBookmark
	}
	m.BookmarkCalls = append(m.BookmarkCalls, call)

	if m.shouldFailBookmark {
		return 0, m.bookmarkFailError
	}

	// Return a mock bookmark handle
	handle := m.nextHandle
	m.nextHandle++
	return handle, nil
}

func (m *MockWindowsEventAPI) EvtFormatMessage(publisherMetadata EvtHandle, event EvtHandle, messageId uint32, valueCount uint32, values uintptr, flags EvtFormatMessageFlag, bufferSize uint32, buffer *byte, bufferUsed *uint32) error {
	call := FormatMessageCall{
		PublisherMetadata: publisherMetadata,
		Event:             event,
		MessageID:         messageId,
		ValueCount:        valueCount,
		Values:            values,
		Flags:             flags,
		BufferSize:        bufferSize,
		BufferUsed:        0, // Will be set below
	}
	m.FormatMessageCalls = append(m.FormatMessageCalls, call)

	if m.shouldFailFormatMessage {
		return m.formatMessageFailError
	}

	// Get the mock event
	events, exists := m.getEventsForHandle(event)
	if !exists || len(events) == 0 {
		return fmt.Errorf("no mock event for handle %d", event)
	}

	mockEvent := events[0]

	// Create proper XML message format that matches what Windows EvtFormatMessage returns
	// This should be a complete XML document that can be unmarshaled
	message := fmt.Sprintf(`<Event xmlns='http://schemas.microsoft.com/win/2004/08/events/event'>
		<System>
			<Provider Name='%s'/>
			<EventID>4625</EventID>
			<Level>%s</Level>
			<EventRecordID>%s</EventRecordID>
		</System>
		<RenderingInfo Culture='en-US'>
			<Message>%s</Message>
		</RenderingInfo>
	</Event>`, mockEvent.Provider, mockEvent.Level, mockEvent.EventRecordID, mockEvent.Message)

	// Convert to UTF-16 as Windows APIs return UTF-16 encoded data
	utf16Data, err := syscall.UTF16FromString(message)
	if err != nil {
		return fmt.Errorf("failed to convert to UTF-16: %v", err)
	}

	// Convert UTF-16 slice to bytes (little-endian)
	messageBytes := make([]byte, len(utf16Data)*2)
	for i, r := range utf16Data {
		messageBytes[i*2] = byte(r)
		messageBytes[i*2+1] = byte(r >> 8)
	}

	// Set the buffer used size (in bytes, not UTF-16 characters)
	*bufferUsed = uint32(len(messageBytes))

	if len(messageBytes) > int(bufferSize) {
		// Return error indicating buffer too small, but still set bufferUsed
		return fmt.Errorf("buffer too small, need %d bytes", len(messageBytes))
	}

	// Create a slice from the buffer pointer with the actual buffer size
	maxSize := int(bufferSize)
	if maxSize > 1<<20 { // Cap at 1MB for safety
		maxSize = 1 << 20
	}
	bufferSlice := (*[1 << 20]byte)(unsafe.Pointer(buffer))[:maxSize]
	copy(bufferSlice, messageBytes)

	return nil
}

func (m *MockWindowsEventAPI) EvtOpenPublisherMetadata(session EvtHandle, publisherId *uint16, logFilePath *uint16, locale uint32, flags uint32) (EvtHandle, error) {
	publisherStr := ""
	if publisherId != nil {
		publisherStr = utf16PtrToString(publisherId)
	}

	logFileStr := ""
	if logFilePath != nil {
		logFileStr = utf16PtrToString(logFilePath)
	}

	call := OpenPublisherCall{
		Session:     session,
		PublisherID: publisherStr,
		LogFilePath: logFileStr,
		Locale:      locale,
		Flags:       flags,
	}
	m.OpenPublisherCalls = append(m.OpenPublisherCalls, call)

	if m.shouldFailOpenPublisher {
		return 0, m.openPublisherFailError
	}

	// Return a mock handle for publisher metadata
	handle := m.nextHandle
	m.nextHandle++
	return handle, nil
}

// Helper methods
func (m *MockWindowsEventAPI) extractRangeFromQuery(query string) state.Range {
	// Parse the XML query to extract EventRecordID constraints using a single regex
	// Look for pattern like "EventRecordID &gt; 2 and EventRecordID &lt;= 4"

	var start, end uint64 = 0, 1000 // Default range

	if query != "" {
		// Extract both start and end in one regex
		rangeRegex := regexp.MustCompile(`EventRecordID &gt; (\d+) and EventRecordID &lt;= (\d+)`)
		if matches := rangeRegex.FindStringSubmatch(query); len(matches) > 2 {
			if parsedStart, err := strconv.ParseUint(matches[1], 10, 64); err == nil {
				start = parsedStart
			}
			if parsedEnd, err := strconv.ParseUint(matches[2], 10, 64); err == nil {
				end = parsedEnd
			}
		}
	}

	return state.NewRange(start, end)
}

func (m *MockWindowsEventAPI) findOrCreateHandleForRange(r state.Range) EvtHandle {
	// Look for existing query events in this range
	for _, events := range m.queryEvents {
		if len(events) > 0 {
			// First, collect all events that fall in the range
			filteredEvents := []*MockEventRecord{}
			for _, event := range events {
				eventID, _ := strconv.ParseUint(event.EventRecordID, 10, 64)
				inRange := eventID > r.StartOffset() && eventID <= r.EndOffset()
				if inRange {
					filteredEvents = append(filteredEvents, event)
				}
			}

			// If we found any events in this range, create a new handle with them
			if len(filteredEvents) > 0 {
				newHandle := m.nextHandle
				m.nextHandle++
				m.queryEvents[newHandle] = filteredEvents
				return newHandle
			}
		}
	}

	// Create new handle with empty events
	handle := m.nextHandle
	m.nextHandle++
	m.queryEvents[handle] = []*MockEventRecord{}
	return handle
}

// Helper method to get events from either subscription or query handles
func (m *MockWindowsEventAPI) getEventsForHandle(handle EvtHandle) ([]*MockEventRecord, bool) {
	// Check subscription events first
	if events, exists := m.subscriptionEvents[handle]; exists {
		return events, true
	}
	// Check query events
	if events, exists := m.queryEvents[handle]; exists {
		return events, true
	}
	return nil, false
}

// Helper method to set events for a handle (updates the correct map)
func (m *MockWindowsEventAPI) setEventsForHandle(handle EvtHandle, events []*MockEventRecord) {
	// Check which map this handle belongs to
	if _, exists := m.subscriptionEvents[handle]; exists {
		m.subscriptionEvents[handle] = events
		return
	}
	if _, exists := m.queryEvents[handle]; exists {
		m.queryEvents[handle] = events
		return
	}
	// If handle doesn't exist in either, default to query events
	m.queryEvents[handle] = events
}

// Helper method to delete handle from both maps
func (m *MockWindowsEventAPI) deleteHandle(handle EvtHandle) {
	delete(m.subscriptionEvents, handle)
	delete(m.queryEvents, handle)
}
func (m *MockWindowsEventAPI) Reset() {
	m.subscriptionEvents = make(map[EvtHandle][]*MockEventRecord)
	m.queryEvents = make(map[EvtHandle][]*MockEventRecord)
	m.nextHandle = 1000
	m.SubscribeCalls = nil
	m.QueryCalls = nil
	m.NextCalls = nil
	m.CloseCalls = nil
	m.RenderCalls = nil
	m.BookmarkCalls = nil
	m.FormatMessageCalls = nil
	m.OpenPublisherCalls = nil
	m.shouldFailSubscribe = false
	m.shouldFailQuery = false
	m.shouldFailNext = false
	m.shouldFailClose = false
	m.shouldFailRender = false
	m.shouldFailBookmark = false
	m.shouldFailFormatMessage = false
	m.shouldFailOpenPublisher = false
}
