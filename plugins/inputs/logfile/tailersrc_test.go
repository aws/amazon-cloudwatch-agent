// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logfile

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/logs"
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
			MaxLineSize: defaultMaxEventSize,
			IsUTF16:     false,
		})

	require.NoError(t, err, fmt.Sprintf("Failed to create tailer src for file %v with error: %v", file, err))
	require.Equal(t, beforeCount+1, tail.OpenFileCount.Load())
	ts := NewTailerSrc(
		"groupName", "streamName",
		"destination", statefile.Name(),
		util.InfrequentAccessLogGroupClass,
		"tailsrctest-*.log",
		tailer,
		false, // AutoRemoval
		regexp.MustCompile("^[\\S]").MatchString,
		nil,
		parseRFC3339Timestamp,
		nil, // encoding
		defaultMaxEventSize,
		defaultTruncateSuffix,
		1,
		false,
	)
	multilineWaitPeriod = 100 * time.Millisecond

	lines := []string{
		logLine("A", 100, time.Now()),
		logLine("B", 256*1024, time.Now()),
		logLine("M", 1023, time.Now()) + strings.Repeat("\n "+logLine("M", 1022, time.Time{}), 255), // 256k multiline
		logLine("C", 256*1024+64, time.Now()),
		logLine("M", 1023, time.Now()) + strings.Repeat("\n "+logLine("M", 1022, time.Time{}), 258), // 258k multiline
		logLine("m", 1023, time.Now()) + strings.Repeat("\n "+logLine("m", 1022, time.Time{}), 258), // 386k multiline split into 2 events
		strings.Repeat("\n "+logLine("m", 1022, time.Time{}), 128),
		logLine("B", 256*1024, time.Now()),
	}

	done := make(chan struct{})
	i := 0
	ts.SetOutput(func(evt logs.LogEvent) {
		if evt == nil {
			close(done)
			return
		}
		msg := evt.Message()
		switch i {
		case 0, 1, 2:
			require.Equal(t, msg, lines[i], fmt.Sprintf("Log Event %d does not match, lengths are %v != %v", i, len(msg), len(lines[i])))
		case 3:
			expected := lines[i][:256*1024]
			require.Equal(t, msg, expected, fmt.Sprintf("Log Event %d should be truncated, does not match expectation, end of the logs are '%v' != '%v'", i, msg[len(msg)-50:], expected[len(expected)-50:]))
		case 4, 5:
			// Know bug: truncated single line log event would be broken into 2n events
		case 6:
			expected := lines[4][:256*1024-len(defaultTruncateSuffix)] + defaultTruncateSuffix
			require.Equal(t, msg, expected, fmt.Sprintf("Log Event %d should be truncated, does not match expectation, end of the logs are '%v ... %v'(%v) != '%v ... %v'(%v)", i, msg[:50], msg[len(msg)-50:], len(msg), expected[:50], expected[len(expected)-50:], len(expected)))
		case 7:
			expected := lines[5][:256*1024-len(defaultTruncateSuffix)] + defaultTruncateSuffix
			require.Equal(t, msg, expected, fmt.Sprintf("Log Event %d should be truncated, does not match expectation, end of the logs are '%v ... %v'(%v) != '%v ... %v'(%v)", i, msg[:50], msg[len(msg)-50:], len(msg), expected[:50], expected[len(expected)-50:], len(expected)))

		case 8:
			expected := lines[7]
			require.Equal(t, msg, expected, fmt.Sprintf("Log Event %d does not match expectation, end of the logs are '%v ... %v'(%v) != '%v ... %v'(%v)", i, msg[:50], msg[len(msg)-50:], len(msg), expected[:50], expected[len(expected)-50:], len(expected)))
		default:
			t.Errorf("unexpected log event: %v", evt)
		}
		i++
	})

	// Slow send
	for _, l := range lines {
		fmt.Fprintln(file, l)
		time.Sleep(2 * time.Second)
	}

	// Fast send
	i = 0
	for _, l := range lines {
		fmt.Fprintln(file, l)
		time.Sleep(500 * time.Millisecond)
	}

	// Removal of log file should stop tailerSrc and Tail.
	err = os.Remove(file.Name())
	require.NoError(t, err, fmt.Sprintf("Failed to remove log file '%v': %v", file.Name(), err))

	<-done

	// Most test functions do not wait for the Tail to close the file.
	// They rely on Tail to detect file deletion and close the file.
	// So the count might be nonzero due to previous test cases.
	assert.Eventually(t, func() bool { return tail.OpenFileCount.Load() <= beforeCount }, 3*time.Second, time.Second)
}

func TestOffsetDoneCallBack(t *testing.T) {
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
			MaxLineSize: defaultMaxEventSize,
			IsUTF16:     false,
		})

	require.NoError(t, err, fmt.Sprintf("Failed to create tailer src for file %v with error: %v", file, err))

	ts := NewTailerSrc(
		"groupName", "streamName",
		"destination",
		statefile.Name(),
		util.InfrequentAccessLogGroupClass,
		"tailsrctest-*.log",
		tailer,
		false, // AutoRemoval
		regexp.MustCompile("^[\\S]").MatchString,
		nil,
		parseRFC3339Timestamp,
		nil, // encoding
		defaultMaxEventSize,
		defaultTruncateSuffix,
		1,
		false,
	)
	multilineWaitPeriod = 100 * time.Millisecond

	done := make(chan struct{})

	i := 0
	ts.SetOutput(func(evt logs.LogEvent) {
		if evt == nil {
			close(done)
			return
		}
		evt.Done()
		i++
		switch i {
		case 10:
			// Test before first truncate
			time.Sleep(1 * time.Second)
			b, err := os.ReadFile(statefile.Name())
			require.NoError(t, err, fmt.Sprintf("Failed to read state file: %v", err))
			offset, err := strconv.Atoi(string(bytes.Split(b, []byte("\n"))[0]))
			require.NoError(t, err, fmt.Sprintf("Failed to parse offset: %v, from '%s'", err, b))
			require.Equal(t, offset, 1010, fmt.Sprintf("Wrong offset %v is written to state file, expecting 1010", offset))
		case 15:
			// Test after first truncate, saved offset should decrease
			time.Sleep(1 * time.Second)
			log.Println(statefile.Name())
			b, err := os.ReadFile(statefile.Name())
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
			b, err := os.ReadFile(statefile.Name())
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
	resources := setupTailer(t, nil, defaultMaxEventSize, false, false)
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
		defaultMaxEventSize,
		false, false,
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

func setupTailer(t *testing.T, multiLineFn func(string) bool, maxEventSize int, autoRemoval bool, handleRotation bool) tailerTestResources {
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
	ts := NewTailerSrc(
		t.Name(),
		t.Name(),
		"destination",
		util.InfrequentAccessLogGroupClass,
		"tailsrctest-*.log",
		statefile.Name(),
		tailer,
		autoRemoval,
		multiLineFn,
		config.Filters,
		parseRFC3339Timestamp,
		nil, // encoding
		maxEventSize,
		defaultTruncateSuffix,
		1,
		handleRotation,
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
	os.Remove(resources.file.Name())
	os.Remove(resources.statefile.Name())
}

func TestTailerSrcFileDescriptorHandling(t *testing.T) {
	// Reset at start of test
	tail.OpenFileCount.Store(0)

	testCases := []struct {
		name           string
		handleRotation bool
		autoRemoval    bool
		expectedLogs   []string
	}{
		{
			name:           "handle_rotation=true",
			handleRotation: true,
			autoRemoval:    false,
			expectedLogs:   []string{"old-log1", "old-log2", "new-log1", "new-log2"},
		},
		{
			name:           "auto_removal=true",
			handleRotation: false,
			autoRemoval:    true,
			expectedLogs:   []string{"old-log1", "old-log2"},
		},
		{
			name:           "handle_rotation=true_and_auto_removal=true",
			handleRotation: true,
			autoRemoval:    true,
			expectedLogs:   []string{"old-log1", "old-log2"}, // Same behavior as auto_removal=true
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			beforeCount := tail.OpenFileCount.Load()
			t.Logf("Before tailer creation: OpenFileCount=%d", beforeCount)

			tmpfile, err := createTempFile("", "tailsrctest-*.log")
			require.NoError(t, err)
			defer os.Remove(tmpfile.Name())

			statefile, err := os.CreateTemp("", "tailsrctest-state-*.log")
			require.NoError(t, err)
			defer os.Remove(statefile.Name())

			tailer, err := tail.TailFile(tmpfile.Name(),
				tail.Config{
					ReOpen:      tc.handleRotation,
					Follow:      true,
					Location:    &tail.SeekInfo{Whence: io.SeekStart, Offset: 0},
					MustExist:   true,
					Poll:        true,
					MaxLineSize: defaultMaxEventSize,
				})
			require.NoError(t, err)

			ts := NewTailerSrc(
				"test-group",
				"test-stream",
				"destination",
				statefile.Name(),
				util.InfrequentAccessLogGroupClass,
				tmpfile.Name(),
				tailer,
				tc.autoRemoval,
				regexp.MustCompile("^[\\S]").MatchString,
				nil,
				parseRFC3339Timestamp,
				nil,
				defaultMaxEventSize,
				defaultTruncateSuffix,
				1,
				tc.handleRotation,
			)

			var processedLogs []string
			var mu sync.Mutex
			done := make(chan struct{})
			logCount := int32(0)

			ts.SetOutput(func(evt logs.LogEvent) {
				if evt == nil {
					close(done)
					return
				}
				mu.Lock()
				processedLogs = append(processedLogs, evt.Message())
				mu.Unlock()
				atomic.AddInt32(&logCount, 1)
			})

			fmt.Fprintln(tmpfile, "old-log1")
			fmt.Fprintln(tmpfile, "old-log2")
			tmpfile.Sync()

			// wait for initial logs
			for atomic.LoadInt32(&logCount) < 2 {
				time.Sleep(100 * time.Millisecond)
			}

			if !tc.autoRemoval && tc.handleRotation {
				oldName := tmpfile.Name() + ".1"
				tmpfile.Close()
				err = os.Rename(tmpfile.Name(), oldName)
				require.NoError(t, err)
				defer os.Remove(oldName)

				newFile, err := os.Create(tmpfile.Name())
				require.NoError(t, err)

				fmt.Fprintln(newFile, "new-log1")
				fmt.Fprintln(newFile, "new-log2")
				newFile.Sync()
				newFile.Close()

				// wait for all logs
				startTime := time.Now()
				for atomic.LoadInt32(&logCount) < 4 && time.Since(startTime) < 5*time.Second {
					time.Sleep(100 * time.Millisecond)
				}

				// check all logs were processed before stopping
				require.Equal(t, int32(4), atomic.LoadInt32(&logCount), "Not all logs were processed")
			}

			ts.Stop()
			tailer.Stop()
			tailer.Cleanup()
			<-done

			mu.Lock()
			t.Logf("Processed logs: %v", processedLogs)
			assert.Equal(t, tc.expectedLogs, processedLogs, "Logs should be processed in order")
			mu.Unlock()

			// for auto_removal, verify file is removed
			if tc.autoRemoval {
				assert.Eventually(t, func() bool {
					_, err := os.Stat(tmpfile.Name())
					return os.IsNotExist(err)
				}, 5*time.Second, 100*time.Millisecond, "File should be removed with auto_removal")
			}

			// verify file descriptors
			assert.Eventually(t, func() bool {
				count := tail.OpenFileCount.Load()
				t.Logf("Current OpenFileCount: %d", count)
				return count == beforeCount
			}, 5*time.Second, 100*time.Millisecond, "File descriptors should be released")
		})
	}
}
