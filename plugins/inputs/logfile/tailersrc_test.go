// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logfile

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/internal/logscommon"
	"github.com/aws/amazon-cloudwatch-agent/internal/state"
	"github.com/aws/amazon-cloudwatch-agent/logs"
	"github.com/aws/amazon-cloudwatch-agent/plugins/inputs/logfile/constants"
	"github.com/aws/amazon-cloudwatch-agent/plugins/inputs/logfile/tail"
	"github.com/aws/amazon-cloudwatch-agent/profiler"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

type tailerTestResources struct {
	done      *chan struct{}
	consumed  *int32
	file      *os.File
	statefile *os.File
	tailer    *tail.Tail
	ts        *tailerSrc
}

func TestTailerSrc(t *testing.T) {
	original := multilineWaitPeriod
	defer resetState(original)

	file, err := createTempFile("", "tailsrctest-*.log")
	defer os.Remove(file.Name())
	require.NoError(t, err, fmt.Sprintf("Failed to create temp file: %v", err))

	statefile, err := os.CreateTemp("", "tailsrctest-state-*.log")
	defer os.Remove(statefile.Name())
	require.NoError(t, err, fmt.Sprintf("Failed to create temp file: %v", err))
	beforeCount := tail.OpenFileCount.Load()
	tailer, err := tail.TailFile(file.Name(),
		tail.Config{
			ReOpen:      false,
			Follow:      true,
			Location:    &tail.SeekInfo{Whence: io.SeekStart, Offset: 0},
			MustExist:   true,
			Pipe:        false,
			Poll:        true,
			MaxLineSize: constants.DefaultMaxEventSize,
			IsUTF16:     false,
		})

	require.NoError(t, err, fmt.Sprintf("Failed to create tailer src for file %v with error: %v", file, err))
	require.Equal(t, beforeCount+1, tail.OpenFileCount.Load())

	stateFilePath := statefile.Name()
	m := state.NewFileRangeManager(state.ManagerConfig{
		StateFileDir: filepath.Dir(stateFilePath),
		Name:         filepath.Base(stateFilePath),
	})

	ts := NewTailerSrc(
		"groupName", "streamName",
		"destination", m,
		util.InfrequentAccessLogGroupClass,
		"tailsrctest-*.log",
		tailer,
		false, // AutoRemoval
		regexp.MustCompile("^[\\S]").MatchString,
		nil,
		parseRFC3339Timestamp,
		nil, // encoding
		constants.DefaultMaxEventSize,
		1,
		"",
	)

	multilineWaitPeriod = 100 * time.Millisecond

	// Create test data with various sizes
	lines := []string{
		logLine("A", 100, time.Now()),      // Small log (100 bytes)
		logLine("B", 256*1024, time.Now()), // 256KB log
		logLine("C", 512*1024, time.Now()), // 512KB log
		// Multiline log - with our buffer changes, this might be handled differently
		// so we'll make it smaller to ensure it's processed as a single event
		logLine("M", 1023, time.Now()) + strings.Repeat("\n "+logLine("M", 100, time.Time{}), 10),
	}

	// Channel to track received events
	eventCh := make(chan logs.LogEvent, 100)

	// Set up the output function
	ts.SetOutput(func(evt logs.LogEvent) {
		if evt == nil {
			return
		}
		eventCh <- evt
	})

	// Write the test data to the file
	for _, line := range lines {
		_, err := file.WriteString(line + "\n")
		require.NoError(t, err)
	}
	file.Sync()

	// Give the tailer some time to process the file
	time.Sleep(10 * time.Second)

	// Check the received events
	close(eventCh)
	receivedEvents := make([]logs.LogEvent, 0)
	for evt := range eventCh {
		receivedEvents = append(receivedEvents, evt)
	}

	// Verify we received the expected number of events
	require.Equal(t, len(lines), len(receivedEvents), "Should have received all events")

	// Verify the content of the events
	for i, evt := range receivedEvents {
		msg := evt.Message()
		expectedMsg := lines[i]

		require.Equal(t, expectedMsg, msg, fmt.Sprintf("Log Event %d doesn't match exactly", i))
	}

	// Removal of log file should stop tailerSrc and Tail.
	err = os.Remove(file.Name())
	require.NoError(t, err, fmt.Sprintf("Failed to remove log file '%v': %v", file.Name(), err))

	// Most test functions do not wait for the Tail to close the file.
	// They rely on Tail to detect file deletion and close the file.
	// So the count might be nonzero due to previous test cases.
	assert.Eventually(t, func() bool { return tail.OpenFileCount.Load() <= beforeCount }, 3*time.Second, time.Second)
}

func TestEventDoneCallback(t *testing.T) {
	original := multilineWaitPeriod
	defer resetState(original)

	file, err := createTempFile("", "tailsrctest-*.log")
	defer os.Remove(file.Name())
	require.NoError(t, err, fmt.Sprintf("Failed to create temp file: %v", err))

	statefile, err := os.CreateTemp("", "tailsrctest-state-*.log")
	defer os.Remove(statefile.Name())
	require.NoError(t, err, fmt.Sprintf("Failed to create temp file: %v", err))

	tailer, err := tail.TailFile(file.Name(),
		tail.Config{
			ReOpen:      false,
			Follow:      true,
			Location:    &tail.SeekInfo{Whence: io.SeekStart, Offset: 0},
			MustExist:   true,
			Pipe:        false,
			Poll:        true,
			MaxLineSize: constants.DefaultMaxEventSize,
			IsUTF16:     false,
		})

	require.NoError(t, err, fmt.Sprintf("Failed to create tailer src for file %v with error: %v", file, err))

	stateFilePath := statefile.Name()
	m := state.NewFileRangeManager(state.ManagerConfig{
		StateFileDir: filepath.Dir(stateFilePath),
		Name:         filepath.Base(stateFilePath),
	})

	ts := NewTailerSrc(
		"groupName", "streamName",
		"destination",
		m,
		util.InfrequentAccessLogGroupClass,
		"tailsrctest-*.log",
		tailer,
		false, // AutoRemoval
		regexp.MustCompile("^[\\S]").MatchString,
		nil,
		parseRFC3339Timestamp,
		nil, // encoding
		constants.DefaultMaxEventSize,
		1,
		"",
	)
	multilineWaitPeriod = 100 * time.Millisecond

	done := make(chan struct{})

	i := 0
	ts.SetOutput(func(evt logs.LogEvent) {
		if evt == nil {
			close(done)
			return
		}
		sle, ok := evt.(logs.StatefulLogEvent)
		assert.True(t, ok)
		sle.Done()
		i++
		switch i {
		case 10:
			// Test before first truncate
			time.Sleep(1 * time.Second)
			b, err := os.ReadFile(stateFilePath)
			require.NoError(t, err, fmt.Sprintf("Failed to read state file: %v", err))
			offset, err := strconv.Atoi(string(bytes.Split(b, []byte("\n"))[0]))
			require.NoError(t, err, fmt.Sprintf("Failed to parse offset: %v, from '%s'", err, b))
			require.Equal(t, offset, 1010, fmt.Sprintf("Wrong offset %v is written to state file, expecting 1010", offset))
		case 15:
			// Test after first truncate, saved offset should decrease
			time.Sleep(1 * time.Second)
			log.Println(stateFilePath)
			b, err := os.ReadFile(stateFilePath)
			require.NoError(t, err, fmt.Sprintf("Failed to read state file: %v", err))
			file_parts := bytes.Split(b, []byte("\n"))
			log.Println("file_parts: ", file_parts)
			file_string := string(file_parts[0])
			log.Println("file_string: ", file_string)
			offset, err := strconv.Atoi(file_string)
			require.NoError(t, err, fmt.Sprintf("Failed to parse offset: %v, from '%s'", err, b))
			require.Equal(t, offset, 505, fmt.Sprintf("Wrong offset %v is written to state file, after truncate and write shorter logs expecting 505", offset))
		case 35:
			time.Sleep(1 * time.Second)
			b, err := os.ReadFile(stateFilePath)
			require.NoError(t, err, fmt.Sprintf("Failed to read state file: %v", err))
			offset, err := strconv.Atoi(string(bytes.Split(b, []byte("\n"))[0]))
			require.NoError(t, err, fmt.Sprintf("Failed to parse offset: %v, from '%s'", err, b))
			require.Equal(t, offset, 2020, fmt.Sprintf("Wrong offset %v is written to state file, after truncate and write shorter logs expecting 2022", offset))
		}
	})

	// Write 100 lines (save state should record 1010 bytes)
	for i := 0; i < 10; i++ {
		fmt.Fprintln(file, logLine("A", 100, time.Now()))
	}
	log.Println("Wait 1 seconds before truncate")
	time.Sleep(1 * time.Second)

	// First truncate then write 50 lines (save state should record 505 bytes)

	log.Println("Truncate file")
	err = file.Truncate(0)
	require.NoError(t, err, fmt.Sprintf("Failed to truncate log file '%v': %v", file.Name(), err))

	log.Println("Sleep before write")
	time.Sleep(1 * time.Second)
	file.Seek(io.SeekStart, 0)
	log.Println("Write for 505")
	for i := 0; i < 5; i++ {
		fmt.Fprintln(file, logLine("B", 100, time.Now()))
	}

	log.Println("Wait 5 seconds before truncate")
	time.Sleep(5 * time.Second)
	log.Println("Truncate then write 20 lines")

	// Second truncate then write 20 lines (save state should record 2020 bytes)
	err = file.Truncate(0)
	require.NoError(t, err, fmt.Sprintf("failed to truncate log file '%v': %v", file.Name(), err))

	time.Sleep(1 * time.Second)
	file.Seek(io.SeekStart, 0)
	for i := 0; i < 20; i++ {
		fmt.Fprintln(file, logLine("C", 100, time.Now()))
	}
	time.Sleep(3 * time.Second)

	// Removal of log file should stop tailersrc
	err = os.Remove(file.Name())
	require.NoError(t, err, fmt.Sprintf("failed to remove log file '%v': %v", file.Name(), err))

	<-done
	require.GreaterOrEqual(t, i, 35, fmt.Sprintf("Not enough logs have been processed, only %v are processed", i))
}

func TestTailerSrcFiltersSingleLineLogs(t *testing.T) {
	original := multilineWaitPeriod
	defer resetState(original)
	resources := setupTailer(t, nil, constants.DefaultMaxEventSize, false, "")
	defer teardown(resources)

	n := 100
	matchedLog := "ERROR: this has an error in it."
	unmatchedLog := "Some other log message"
	publishLogsToFile(resources.file, matchedLog, unmatchedLog, n, 0)

	// Removal of log file should stop tailersrc
	err := os.Remove(resources.file.Name())
	require.NoError(t, err, fmt.Sprintf("Failed to remove log file '%v': %v", resources.file.Name(), err))

	<-*resources.done
	assertExpectedLogsPublished(t, n, int(*resources.consumed))
}

func TestTailerSrcFiltersMultiLineLogs(t *testing.T) {
	original := multilineWaitPeriod
	defer resetState(original)
	resources := setupTailer(
		t,
		regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[Z+\-]\d{2}:\d{2}`).MatchString,
		constants.DefaultMaxEventSize,
		false, "",
	)
	defer teardown(resources)

	n := 20
	// create log messages ahead of time to save compute time
	buf := bytes.Buffer{}
	buf.WriteString("This has a matching log in the middle of it")
	buf.WriteString(strings.Repeat("\nfoo", 2))
	buf.WriteString("\nHere is the ERROR that should be matched")
	buf.WriteString(strings.Repeat("\nfoo", 2))
	matchedLog := buf.String()
	buf.Reset()

	unmatchedLog := "This should not be matched." + strings.Repeat("\nbar", 5)

	publishLogsToFile(resources.file, matchedLog, unmatchedLog, n, 100)

	// Removal of log file should stop tailersrc
	err := os.Remove(resources.file.Name())
	require.NoError(t, err, fmt.Sprintf("Failed to remove log file '%v': %v", resources.file.Name(), err))

	<-*resources.done
	assertExpectedLogsPublished(t, n, int(*resources.consumed))
}

func parseRFC3339Timestamp(line string) (time.Time, string) {
	// Use RFC3339 for testing `2006-01-02T15:04:05Z07:00`
	re := regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[Z+\-]\d{2}:\d{2}`)
	tstr := re.FindString(line)
	var t time.Time
	if tstr != "" {
		t, _ = time.Parse(time.RFC3339, tstr)
	}
	return t, line
}

func logLine(s string, l int, t time.Time) string {
	line := ""
	if !t.IsZero() {
		line += t.Format(time.RFC3339) + " "
	}

	if s == "" {
		s = "A"
	}

	line += strings.Repeat(s, l/len(s)+1)
	line = line[:l]
	return line
}

func logWithTimestampPrefix(s string) string {
	return fmt.Sprintf("%v - %s", time.Now().Format(time.RFC3339), s)
}

func setupTailer(t *testing.T, multiLineFn func(string) bool, maxEventSize int, autoRemoval bool, backpressureDrop logscommon.BackpressureMode) tailerTestResources {
	done := make(chan struct{})
	var consumed int32
	file, err := createTempFile("", "tailsrctest-*.log")
	require.NoError(t, err, fmt.Sprintf("Failed to create temp file: %v", err))
	statefile, err := createTempFile("", "tailsrctest-state-*.log")
	require.NoError(t, err, fmt.Sprintf("Failed to create temp file: %v", err))

	tailer, err := tail.TailFile(file.Name(),
		tail.Config{
			ReOpen:      false,
			Follow:      true,
			Location:    &tail.SeekInfo{Whence: io.SeekStart, Offset: 0},
			MustExist:   true,
			Pipe:        false,
			Poll:        true,
			MaxLineSize: maxEventSize,
			IsUTF16:     false,
		})

	require.NoError(t, err, fmt.Sprintf("Failed to create tailer src for file %v with error: %v", file, err))

	config := &FileConfig{
		LogGroupName:  t.Name(),
		LogStreamName: t.Name(),
		Filters: []*LogFilter{
			{
				Type:       includeFilterType,
				Expression: "ERROR", // only match error logs
			},
		},
	}
	err = config.init()
	assert.NoError(t, err)

	stateFilePath := statefile.Name()
	m := state.NewFileRangeManager(state.ManagerConfig{
		StateFileDir: filepath.Dir(stateFilePath),
		Name:         filepath.Base(stateFilePath),
	})

	ts := NewTailerSrc(
		t.Name(),
		t.Name(),
		"destination",
		m,
		util.InfrequentAccessLogGroupClass,
		"tailsrctest-*.log",
		tailer,
		autoRemoval,
		multiLineFn,
		config.Filters,
		parseRFC3339Timestamp,
		nil, // encoding
		maxEventSize,
		1,
		backpressureDrop,
	)

	ts.SetOutput(func(evt logs.LogEvent) {
		if evt == nil {
			close(done)
			return
		}
		atomic.AddInt32(&consumed, 1)
		evt.Done()
	})

	return tailerTestResources{
		done:      &done,
		consumed:  &consumed,
		file:      file,
		statefile: statefile,
		tailer:    tailer,
		ts:        ts,
	}
}

func publishLogsToFile(file *os.File, matchedLog, unmatchedLog string, n, multiLineWaitMs int) {
	var sleepDuration time.Duration
	if multiLineWaitMs > 0 {
		multilineWaitPeriod = time.Duration(multiLineWaitMs) * time.Millisecond
		sleepDuration = time.Duration(multiLineWaitMs*10) * time.Millisecond
	}

	for i := 0; i < n; i++ {
		mod := i % 2
		if mod == 0 {
			fmt.Fprintln(file, logWithTimestampPrefix(unmatchedLog))
		} else {
			fmt.Fprintln(file, logWithTimestampPrefix(matchedLog))
		}
		if multiLineWaitMs > 0 {
			time.Sleep(sleepDuration)
		} else {
			time.Sleep(multilineWaitPeriod)
		}
	}
}

func assertExpectedLogsPublished(t *testing.T, total, numConsumed int) {
	// Atomic recommends synchronization functions is better done with channels or the facilities of the sync package
	// Therefore, the count will fluctuate and not equal with the expect consumed.
	assert.LessOrEqual(t, numConsumed, total/2)
	stats := profiler.Profiler.GetStats()
	statKey := fmt.Sprintf("logfile_%s_%s_messages_dropped", t.Name(), t.Name())
	if val, ok := stats[statKey]; !ok {
		t.Error("Missing profiled stat")
	} else {
		assert.LessOrEqual(t, int(val), total/2)
	}
}

func resetState(originWaitDuration time.Duration) {
	multilineWaitPeriod = originWaitDuration
	profiler.Profiler.ReportAndClear()
}

func teardown(resources tailerTestResources) {
	if resources.file != nil {
		resources.file.Close()
	}
	if resources.statefile != nil {
		resources.statefile.Close()
	}
	os.Remove(resources.file.Name())
	os.Remove(resources.statefile.Name())

	// Wait for file operations to complete
	time.Sleep(100 * time.Millisecond)
}

func TestTailerSrcCloseFileDescriptorOnBufferBlock(t *testing.T) {
	resources := setupTailer(t, nil, constants.DefaultMaxEventSize, false, logscommon.LogBackpressureModeFDRelease)

	doneCh := make(chan struct{})
	var consumed int32
	blockCh := make(chan struct{})

	defer func() {
		close(blockCh)
		time.Sleep(100 * time.Millisecond)
		resources.ts.Stop()
		<-doneCh
		teardown(resources)
	}()

	initialCount := tail.OpenFileCount.Load()
	resources.ts.SetOutput(func(evt logs.LogEvent) {
		if evt == nil {
			close(doneCh)
			return
		}
		atomic.AddInt32(&consumed, 1)
		t.Logf("Processed log: %s", evt.Message())
		select {
		case <-blockCh:
			return
		default:
			<-blockCh
		}
	})

	// Write logs to fill the buffer
	logCount := defaultBufferSize + 5
	for i := 0; i < logCount; i++ {
		_, err := fmt.Fprintf(resources.file, "ERROR: Test log line %d\n", i)
		require.NoError(t, err)
	}
	resources.file.Sync()

	// Wait for buffer to fill and file operations to complete
	maxRetries := 10
	var currentCount int64
	for i := 0; i < maxRetries; i++ {
		time.Sleep(100 * time.Millisecond)
		currentCount = tail.OpenFileCount.Load()
		if currentCount <= initialCount {
			break
		}
	}

	t.Logf("OpenFileCount after buffer full: %d", currentCount)
	assert.LessOrEqual(t, currentCount, initialCount, "File count should not increase when buffer is full")

	// Allow processing to continue
	for i := 0; i < 3; i++ {
		blockCh <- struct{}{}
		time.Sleep(50 * time.Millisecond)
		currentCount = tail.OpenFileCount.Load()
		t.Logf("OpenFileCount during processing %d: %d", i, currentCount)
	}

	// Verify that some logs were processed
	processedCount := atomic.LoadInt32(&consumed)
	t.Logf("Processed %d logs", processedCount)
	assert.Greater(t, processedCount, int32(0), "Should have processed some logs")

	// Final check of OpenFileCount
	finalCount := tail.OpenFileCount.Load()
	assert.LessOrEqual(t, finalCount, initialCount, "File count should not increase")
}
