// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logfile

import (
	"bytes"
	"fmt"
	"github.com/aws/amazon-cloudwatch-agent/profiler"
	"github.com/stretchr/testify/assert"
	"io"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/logs"
	"github.com/aws/amazon-cloudwatch-agent/plugins/inputs/logfile/tail"
)

func TestTailerSrc(t *testing.T) {

	file, err := createTempFile("", "tailsrctest-*.log")
	defer os.Remove(file.Name())
	if err != nil {
		t.Errorf("Failed to create temp file: %v", err)
	}

	statefile, err := ioutil.TempFile("", "tailsrctest-state-*.log")
	defer os.Remove(statefile.Name())
	if err != nil {
		t.Errorf("Failed to create temp file: %v", err)
	}

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

	if err != nil {
		t.Errorf("Failed to create tailer src for file %v with error: %v", file, err)
		return
	}

	ts := NewTailerSrc(
		"groupName", "streamName",
		"destination",
		statefile.Name(),
		tailer,
		false, // AutoRemoval
		regexp.MustCompile("^[\\S]").MatchString,
		nil,
		parseRFC3339Timestamp,
		nil, // encoding
		defaultMaxEventSize,
		defaultTruncateSuffix,
		1,
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
			if msg != lines[i] {
				t.Errorf("Log Event %d does not match, lengths are %v != %v", i, len(msg), len(lines[i]))
			}
		case 3:
			expected := lines[i][:256*1024]
			if msg != expected {
				t.Errorf("Log Event %d should be truncated, does not match expectation, end of the logs are '%v' != '%v'", i, msg[len(msg)-50:], expected[len(expected)-50:])
			}
		case 4, 5:
			// Know bug: truncated single line log event would be broken into 2n events
		case 6:
			expected := lines[4][:256*1024-len(defaultTruncateSuffix)] + defaultTruncateSuffix
			if msg != expected {
				t.Errorf("Log Event %d should be truncated, does not match expectation, end of the logs are '%v ... %v'(%v) != '%v ... %v'(%v)", i, msg[:50], msg[len(msg)-50:], len(msg), expected[:50], expected[len(expected)-50:], len(expected))
			}
		case 7:
			expected := lines[5][:256*1024-len(defaultTruncateSuffix)] + defaultTruncateSuffix
			if msg != expected {
				t.Errorf("Log Event %d should be truncated, does not match expectation, end of the logs are '%v ... %v'(%v) != '%v ... %v'(%v)", i, msg[:50], msg[len(msg)-50:], len(msg), expected[:50], expected[len(expected)-50:], len(expected))
			}
		case 8:
			expected := lines[7]
			if msg != expected {
				t.Errorf("Log Event %d does not match expectation, end of the logs are '%v ... %v'(%v) != '%v ... %v'(%v)", i, msg[:50], msg[len(msg)-50:], len(msg), expected[:50], expected[len(expected)-50:], len(expected))
			}
		default:
			t.Errorf("unexpected log event: %v", evt)
		}
		i++
	})

	// Slow send
	for _, l := range lines {
		fmt.Fprintln(file, l)
		time.Sleep(1 * time.Second)
	}

	// Fast send
	i = 0
	for _, l := range lines {
		fmt.Fprintln(file, l)
	}

	// Removal of log file should stop tailersrc
	if err := os.Remove(file.Name()); err != nil {
		t.Errorf("failed to remove log file '%v': %v", file.Name(), err)
	}
	<-done
}

func TestOffsetDoneCallBack(t *testing.T) {

	file, err := createTempFile("", "tailsrctest-*.log")
	defer os.Remove(file.Name())
	if err != nil {
		t.Errorf("Failed to create temp file: %v", err)
	}

	statefile, err := ioutil.TempFile("", "tailsrctest-state-*.log")
	defer os.Remove(statefile.Name())
	if err != nil {
		t.Errorf("Failed to create temp file: %v", err)
	}

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

	if err != nil {
		t.Errorf("Failed to create tailer src for file %v with error: %v", file, err)
		return
	}

	ts := NewTailerSrc(
		"groupName", "streamName",
		"destination",
		statefile.Name(),
		tailer,
		false, // AutoRemoval
		regexp.MustCompile("^[\\S]").MatchString,
		nil,
		parseRFC3339Timestamp,
		nil, // encoding
		defaultMaxEventSize,
		defaultTruncateSuffix,
		1,
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
		log.Println(i)
		if i == 10 { // Test before first truncate
			time.Sleep(1 * time.Second)
			b, err := ioutil.ReadFile(statefile.Name())
			if err != nil {
				t.Errorf("Failed to read state file: %v", err)
			}
			offset, err := strconv.Atoi(string(bytes.Split(b, []byte("\n"))[0]))
			if err != nil {
				t.Errorf("Failed to parse offset: %v, from '%s'", err, b)
			}

			if offset != 1010 {
				t.Errorf("Wrong offset %v is written to state file, expecting 1010", offset)
			}
		}

		if i == 15 { // Test after first truncate, saved offset should decrease
			time.Sleep(1 * time.Second)
			log.Println(statefile.Name())
			b, err := ioutil.ReadFile(statefile.Name())
			log.Println(b)
			if err != nil {
				t.Errorf("Failed to read state file: %v", err)
			}
			file_parts := bytes.Split(b, []byte("\n"))
			log.Println("file_parts: ", file_parts)
			file_string := string(file_parts[0])
			log.Println("file_string: ", file_string)
			offset, err := strconv.Atoi(file_string)
			log.Println(offset)
			log.Println(err)
			if err != nil {
				t.Errorf("Failed to parse offset: %v, from '%s'", err, b)
			}

			if offset != 505 {
				t.Errorf("Wrong offset %v is written to state file, after truncate and write shorter logs expecting 505", offset)
			}
		}

		if i == 35 { // Test after 2nd truncate, the offset should be larger
			time.Sleep(1 * time.Second)
			b, err := ioutil.ReadFile(statefile.Name())
			if err != nil {
				t.Errorf("Failed to read state file: %v", err)
			}
			offset, err := strconv.Atoi(string(bytes.Split(b, []byte("\n"))[0]))
			if err != nil {
				t.Errorf("Failed to parse offset: %v, from '%s'", err, b)
			}
			if offset != 2020 {
				t.Errorf("Wrong offset %v is written to state file, after truncate and write longer logs expecting 2020", offset)
			}
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
	if err := file.Truncate(0); err != nil {
		t.Errorf("Failed to truncate log file '%v': %v", file.Name(), err)
	}
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
	if err := file.Truncate(0); err != nil {
		t.Errorf("Failed to truncate log file '%v': %v", file.Name(), err)
	}
	time.Sleep(1 * time.Second)
	file.Seek(io.SeekStart, 0)
	for i := 0; i < 20; i++ {
		fmt.Fprintln(file, logLine("C", 100, time.Now()))
	}
	time.Sleep(2 * time.Second)

	// Removal of log file should stop tailersrc
	if err := os.Remove(file.Name()); err != nil {
		t.Errorf("failed to remove log file '%v': %v", file.Name(), err)
	}
	<-done
	if i < 35 {
		t.Errorf("Not enough logs have been processed, only %v are processed", i)
	}
}

func TestTailerSrcFiltersSingleLineLogs(t *testing.T) {
	profiler.Profiler.ReportAndClear()
	file, err := createTempFile("", "tailsrctest-*.log")
	config := &FileConfig{
		LogGroupName: "groupName",
		LogStreamName: "streamName",
		Filters: []LogFilter{
			{
				Type:       includeType,
				Expression: "(ERROR|WARN)",
			},
			{
				Type:       excludeType,
				Expression: "search_(\\w+)",
			},
			{
				Type:       includeType,
				Expression: "StatusCode: [4-5]\\d\\d",
			},
		},
		DropByDefault: true, // because we aren't calling init() to populate this value
	}
	defer os.Remove(file.Name())
	if err != nil {
		t.Errorf("Failed to create temp file: %v", err)
	}

	statefile, err := createTempFile("", "tailsrctest-state-*.log")
	defer os.Remove(statefile.Name())
	if err != nil {
		t.Errorf("Failed to create temp file: %v", err)
	}

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

	if err != nil {
		t.Errorf("Failed to create tailer src for file %v with error: %v", file, err)
		return
	}

	ts := NewTailerSrc(
		"groupName", "streamName",
		"destination",
		statefile.Name(),
		tailer,
		false, // AutoRemoval
		nil,
		config.shouldFilterLog,
		parseRFC3339Timestamp,
		nil, // encoding
		defaultMaxEventSize,
		defaultTruncateSuffix,
		1,
	)

	done := make(chan struct{})
	consumed := 0
	ts.SetOutput(func(evt logs.LogEvent) {
		if evt == nil {
			close(done)
			return
		}
		consumed += 1
		evt.Done()
		// extra sanity check
		assert.False(t, config.shouldFilterLog(evt))
		// make sure there aren't multiple lines in the log
		assert.False(t, strings.Contains(evt.Message(), "\n"))
	})

	// Write 100 lines
	for i := 0; i < 100; i++ {
		mod := i % 10 // use the mod value to control the log messages emitted for consistency
		/**
		1 => "ERROR" in log
		2 => "search_*" in log
		3 => "WARN" AND "search_*" in log
		4 => "StatusCode: 4xx" in log
		8 => "search_*" AND "StatusCode: 5xx" in log
		9 => "StatusCode: 2xx" in log (shouldn't be matched)
		default => "foo bar baz" in log
		 */
		switch mod {
		case 1:
			fmt.Fprintln(file, logSpecificLine(fmt.Sprintf("ERROR: This is an error on line %d", i), time.Now()))
		case 2:
			fmt.Fprintln(file, logSpecificLine(fmt.Sprintf("INFO: This is a log that has search_foo_barBaz%d in it on line %d", i, i), time.Now()))
		case 3:
			fmt.Fprintln(file, logSpecificLine(fmt.Sprintf("WARN: This log contains search_Abc123 in it on line %d", i), time.Now()))
		case 4:
			fmt.Fprintln(file, logSpecificLine(fmt.Sprintf("INFO: API for /foo/bar/baz responded with StatusCode: 400, Message: Bad Request on line %d", i), time.Now()))
		case 8:
			fmt.Fprintln(file, logSpecificLine(fmt.Sprintf("DEBUG: search request for search_XyZ responded with StatusCode: 503, Service Unavailable on line %d", i), time.Now()))
		case 9:
			fmt.Fprintln(file, logSpecificLine(fmt.Sprintf("DEBUG: Received normal API response of StatusCode: 200 for /foo/bar/baz on line %d", i), time.Now()))
		default:
			fmt.Fprintln(file, logSpecificLine(fmt.Sprintf("foo bar baz on line %d", i), time.Now()))
		}
	}

	time.Sleep(2 * time.Second)

	// Removal of log file should stop tailersrc
	if err := os.Remove(file.Name()); err != nil {
		t.Errorf("failed to remove log file '%v': %v", file.Name(), err)
	}
	<-done
	assert.Equal(t, 30, consumed)
	stats := profiler.Profiler.GetStats()
	assert.Len(t, stats, 1)
	if val, ok := stats["logfile_groupName_streamName_messages_dropped"]; !ok {
		t.Error("Missing profiled stat")
	} else {
		assert.Equal(t, 70, int(val))
	}
}

func TestTailerSrcFiltersMultiLineLogs(t *testing.T) {
	profiler.Profiler.ReportAndClear()
	file, err := createTempFile("", "tailsrctest-*.log")
	config := &FileConfig{
		LogGroupName: "groupName1",
		LogStreamName: "streamName1",
		Filters: []LogFilter{
			{
				Type:       includeType,
				Expression: "(ERROR|WARN)",
			},
			{
				Type:       excludeType,
				Expression: "search_(\\w+)",
			},
			{
				Type:       includeType,
				Expression: "StatusCode: [4-5]\\d\\d",
			},
		},
		DropByDefault: true, // because we aren't calling init() to populate this value
	}
	defer os.Remove(file.Name())
	if err != nil {
		t.Errorf("Failed to create temp file: %v", err)
	}

	statefile, err := createTempFile("", "tailsrctest-state-*.log")
	defer os.Remove(statefile.Name())
	if err != nil {
		t.Errorf("Failed to create temp file: %v", err)
	}

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

	if err != nil {
		t.Errorf("Failed to create tailer src for file %v with error: %v", file, err)
		return
	}

	ts := NewTailerSrc(
		"groupName1", "streamName1",
		"destination",
		statefile.Name(),
		tailer,
		false, // AutoRemoval
		regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[Z+\-]\d{2}:\d{2}`).MatchString,
		config.shouldFilterLog,
		parseRFC3339Timestamp,
		nil, // encoding
		defaultMaxEventSize,
		defaultTruncateSuffix,
		1,
	)
	multilineWaitPeriod = 100 * time.Millisecond

	done := make(chan struct{})
	consumed := 0
	ts.SetOutput(func(evt logs.LogEvent) {
		if evt == nil {
			close(done)
			return
		}
		consumed += 1
		evt.Done()
		// extra sanity check
		assert.False(t, config.shouldFilterLog(evt))
		// making sure that the messages include new line characters
		assert.True(t, strings.Contains(evt.Message(), "\n"))
	})

	// Write 100 lines
	var buf bytes.Buffer
	for i := 0; i < 100; i++ {
		mod := i % 10 // use the mod value to control the log messages emitted for consistency
		/**
		1 => multi line: has "ERROR" on first line
		3 => multi line: has "StatusCode 5xx" on later line
		5 => multi line: has "search_*" on later line
		default => "foo bar baz"
		*/
		switch mod {
		case 1:
			fmt.Fprintln(file, logSpecificLine("ERROR: This log has an error on it. Exception:", time.Now()) + strings.Repeat("\nfoo", 10))
		case 3:
			buf.WriteString(logSpecificLine("This log message contains a status code", time.Now()))
			for j := 0; j < 10; j++ {
				if j == 3 {
					buf.WriteString("\nFailed API /foo/bar/baz with StatusCode: 500")
				} else {
					buf.WriteString("\nbar")
				}
			}
			fmt.Fprintln(file, buf.String())
			buf.Reset()
		case 5:
			buf.WriteString(logSpecificLine("This log message should get filtered out.", time.Now()))
			for j := 0; j < 10; j++ {
				if j == 6 {
					buf.WriteString("\nCalled search_foo_barBaz and succeeded")
				} else {
					buf.WriteString("\nbaz")
				}
			}
			fmt.Fprintln(file, buf.String())
			buf.Reset()
		default:
			fmt.Fprintln(file, logSpecificLine(fmt.Sprintf("foo bar baz on line %d", i), time.Now()))
		}
	}

	time.Sleep(2 * time.Second)

	// Removal of log file should stop tailersrc
	if err := os.Remove(file.Name()); err != nil {
		t.Errorf("failed to remove log file '%v': %v", file.Name(), err)
	}
	<-done
	assert.Equal(t, 20, consumed)
	stats := profiler.Profiler.GetStats()
	assert.Len(t, stats, 1)
	if val, ok := stats["logfile_groupName1_streamName1_messages_dropped"]; !ok {
		t.Error("Missing profiled stat")
	} else {
		assert.Equal(t, 80, int(val))
	}
}

func parseRFC3339Timestamp(line string) time.Time {
	// Use RFC3339 for testing `2006-01-02T15:04:05Z07:00`
	re := regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[Z+\-]\d{2}:\d{2}`)
	tstr := re.FindString(line)
	var t time.Time
	if tstr != "" {
		t, _ = time.Parse(time.RFC3339, tstr)
	}
	return t
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

func logSpecificLine(s string, t time.Time) string {
	line := ""
	if !t.IsZero() {
		line += t.Format(time.RFC3339) + " "
	}
	line += s

	return line
}
