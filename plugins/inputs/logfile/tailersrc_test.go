// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logfile

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/logs"
	"github.com/aws/amazon-cloudwatch-agent/plugins/inputs/logfile/tail"
)

func TestTailerSrc(t *testing.T) {

	file, err := ioutil.TempFile("", "tailsrctest-*.log")
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
		parseRFC3339Timestamp,
		nil, // encoding
		defaultMaxEventSize,
		defaultTruncateSuffix,
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

	os.Remove(file.Name())
	<-done
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
