// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logfile

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/logs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

const (
	rawLogLine     = "raw_log_line"
	stringDateType = "string"
)

type TestLogger struct {
	t *testing.T
}

func (tl TestLogger) Errorf(format string, args ...interface{}) {
	tl.t.Errorf(format, args...)
}
func (tl TestLogger) Error(args ...interface{}) {
	tl.t.Error(args...)
}
func (tl TestLogger) Debugf(format string, args ...interface{}) { log.Printf(format, args...) }
func (tl TestLogger) Debug(args ...interface{})                 { log.Println(args...) }
func (tl TestLogger) Warnf(format string, args ...interface{})  { log.Printf(format, args...) }
func (tl TestLogger) Warn(args ...interface{})                  { log.Println(args...) }
func (tl TestLogger) Infof(format string, args ...interface{})  { log.Printf(format, args...) }
func (tl TestLogger) Info(args ...interface{})                  { log.Println(args...) }

func TestLogs(t *testing.T) {
	multilineWaitPeriod = 10 * time.Millisecond
	logEntryString := "cpu,mytag=foo usage_idle=100"
	tmpfile, err := createTempFile("", "")
	defer os.Remove(tmpfile.Name())

	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	require.NoError(t, err)

	_, err = tmpfile.WriteString(logEntryString + "\n")
	require.NoError(t, err)

	tt := NewLogFile()
	tt.Log = TestLogger{t}
	tt.FileConfig = []FileConfig{{FilePath: tmpfile.Name(), FromBeginning: true}}
	tt.FileConfig[0].init()
	tt.started = true

	lsrcs := tt.FindLogSrc()
	if len(lsrcs) != 1 {
		t.Fatalf("%v log src was returned when only 1 should be available", len(lsrcs))
	}

	done := make(chan struct{})
	lsrc := lsrcs[0]
	lsrc.SetOutput(func(e logs.LogEvent) {
		if e == nil {
			return
		}
		if e.Message() != logEntryString {
			t.Errorf("Log entry string does not match:\nExpect: %v\nFound : %v", logEntryString, e.Message())
		}
		if !e.Time().IsZero() {
			t.Errorf("Log entry should be zero time when no timestamp regex is configured")
		}
		close(done)
	})

	<-done

	lsrc.Stop()
	tt.Stop()
}

func TestLogsEncoding(t *testing.T) {
	multilineWaitPeriod = 10 * time.Millisecond
	//2 * rune_len when it is coded in gbk encoding.
	logEntryString := "测试"
	tmpfile, err := createTempFile("", "")
	defer os.Remove(tmpfile.Name())
	require.NoError(t, err)

	writer := transform.NewWriter(tmpfile, simplifiedchinese.GBK.NewEncoder())
	_, err = writer.Write([]byte(logEntryString + "\n"))
	require.NoError(t, err)

	tt := NewLogFile()
	tt.Log = TestLogger{t}
	tt.FileConfig = []FileConfig{{FilePath: tmpfile.Name(), Encoding: "gbk", FromBeginning: true}}
	tt.FileConfig[0].init()
	tt.started = true

	lsrcs := tt.FindLogSrc()
	if len(lsrcs) != 1 {
		t.Fatalf("%v log src was returned when only 1 should be available", len(lsrcs))
	}

	done := make(chan struct{})
	lsrc := lsrcs[0]
	lsrc.SetOutput(func(e logs.LogEvent) {
		if e == nil {
			return
		}
		if e.Message() != logEntryString {
			t.Errorf("Log entry string does not match:\nExpect: %v\nFound : %v", logEntryString, e.Message())
		}
		close(done)
	})

	<-done

	lsrc.Stop()
	tt.Stop()
}

func TestLogsEncodingUtf16(t *testing.T) {
	multilineWaitPeriod = 10 * time.Millisecond
	f, err := createTempFile("", "")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	inputBytesArray := []byte{
		// 'A' 00, 'B' 00, '\n' 00
		'\u0061', '\u0000', '\u0062', '\u0000', '\u000a', '\u0000',
		// '\n' 00
		'\u000a', '\u0000',
		// '\r' 00, '\n' 00
		'\u000d', '\u0000', '\u000a', '\u0000',
		// 'C' 00 '\r' 00, '\n' 00
		'\u0063', '\u0000', '\u000d', '\u0000', '\u000a', '\u0000',
		// D 00 '\r' 00 0a 66 '\n' 00
		'\u0064', '\u0000', '\u000d', '\u0000', '\u000a', '\u0066', '\u000a', '\u0000'}
	f.Write(inputBytesArray)
	f.Sync()

	tt := NewLogFile()
	tt.Log = TestLogger{t}
	tt.FileConfig = []FileConfig{{FilePath: f.Name(), Encoding: "utf-16le", FromBeginning: true}}
	tt.FileConfig[0].init()
	tt.started = true

	lsrcs := tt.FindLogSrc()
	if len(lsrcs) != 1 {
		t.Fatalf("%v log src was returned when only 1 should be available", len(lsrcs))
	}

	evts := make(chan logs.LogEvent)
	lsrc := lsrcs[0]
	lsrc.SetOutput(func(e logs.LogEvent) {
		evts <- e
	})

	expected := []string{"ab\n\n", "c", "d\r昊"}
	for _, expect := range expected {
		e := <-evts
		if e != nil && e.Message() != expect {
			t.Errorf("Log message does not match expectation, expect %q but found %q", expect, e.Message())
		}
	}

	lsrc.Stop()
	tt.Stop()

}

func TestCompressedFile(t *testing.T) {
	multilineWaitPeriod = 10 * time.Millisecond
	filepath := "/tmp/logfile.log"
	compressed := isCompressedFile(filepath)
	assert.False(t, compressed, "This should not be a compressed file.")
	filepath = "/tmp/logfile.log.gz"
	compressed = isCompressedFile(filepath)
	assert.True(t, compressed, "This should be a compressed file.")
}

func TestRestoreState(t *testing.T) {
	multilineWaitPeriod = 10 * time.Millisecond
	tmpfolder, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer os.RemoveAll(tmpfolder)

	logFilePath := "/tmp/logfile.log"
	logFileStateFileName := "_tmp_logfile.log"

	offset := int64(9323)
	err = ioutil.WriteFile(
		tmpfolder+string(filepath.Separator)+logFileStateFileName,
		[]byte(strconv.FormatInt(offset, 10)+"\n"+logFilePath),
		os.ModePerm)
	require.NoError(t, err)

	tt := NewLogFile()
	tt.Log = TestLogger{t}
	tt.FileStateFolder = tmpfolder
	roffset, err := tt.restoreState(logFilePath)
	assert.Equal(t, offset, roffset, fmt.Sprintf("The actual offset is %d, different from the expected offset %d.", roffset, offset))
	tt.Stop()
}

func TestMultipleFilesForSameConfig(t *testing.T) {
	multilineWaitPeriod = 10 * time.Millisecond
	tmpfile1, err := createTempFile("", "tmp1_")
	defer os.Remove(tmpfile1.Name())
	require.NoError(t, err)

	_, err = tmpfile1.WriteString("1\n")
	require.NoError(t, err)

	//make file stat reflect the diff of file ModTime
	time.Sleep(time.Second * 2)

	tmpfile2, err := createTempFile("", "tmp2_")
	defer os.Remove(tmpfile2.Name())
	require.NoError(t, err)

	_, err = tmpfile2.WriteString("2\n")
	require.NoError(t, err)

	logGroupName := "SomeLogGroupName"
	tt := NewLogFile()
	tt.Log = TestLogger{t}
	tt.FileConfig = []FileConfig{{
		FilePath:      filepath.Dir(tmpfile1.Name()) + string(filepath.Separator) + "*",
		FromBeginning: true,
		LogGroupName:  logGroupName,
	}}
	tt.FileConfig[0].init()
	tt.started = true

	lsrcs := tt.FindLogSrc()
	if len(lsrcs) != 1 {
		t.Fatalf("%v log src was returned when 1 should be available", len(lsrcs))
	}

	evts := make(chan logs.LogEvent)
	lsrc := lsrcs[0]
	if lsrc.Group() != logGroupName {
		t.Errorf("Wrong LogGroupName is set for log src, expecting %v, but received %v", logGroupName, lsrc.Group())
	}
	lsrc.SetOutput(func(e logs.LogEvent) {
		evts <- e
	})

	e := <-evts
	expect := "2"
	if e.Message() != expect {
		t.Errorf("Log message does not match expectation, expect %q but found %q", expect, e.Message())
	}

	lsrc.Stop()
	tt.Stop()
}

func TestLogsMultilineEvent(t *testing.T) {
	multilineWaitPeriod = 10 * time.Millisecond
	logEntryString := "multiline begin1\n append line1\nmultiline begin2\n append line2"
	tmpfile, err := createTempFile("", "")
	defer os.Remove(tmpfile.Name())
	require.NoError(t, err)

	_, err = tmpfile.WriteString(logEntryString + "\n")
	require.NoError(t, err)

	tt := NewLogFile()
	tt.Log = TestLogger{t}
	tt.FileConfig = []FileConfig{{FilePath: tmpfile.Name(), FromBeginning: true}}
	tt.FileConfig[0].init()
	tt.started = true

	lsrcs := tt.FindLogSrc()
	if len(lsrcs) != 1 {
		t.Fatalf("%v log src was returned when 1 should be available", len(lsrcs))
	}

	lsrc := lsrcs[0]
	evts := make(chan logs.LogEvent)
	lsrc.SetOutput(func(e logs.LogEvent) {
		evts <- e
	})

	e1 := "multiline begin1\n append line1"
	e2 := "multiline begin2\n append line2"

	e := <-evts
	if e.Message() != e1 {
		t.Errorf("Wrong multiline log found: \n%v\nExpecting:\n%v\n", e.Message(), e1)
	}

	e = <-evts
	if e.Message() != e2 {
		t.Errorf("Wrong multiline log found: \n%v\nExpecting:\n%v\n", e.Message(), e2)
	}

	lsrc.Stop()
	tt.Stop()
}

//When file is removed, the related tail routing should exit
func TestLogsFileRemove(t *testing.T) {
	multilineWaitPeriod = 10 * time.Millisecond
	logEntryString := "anything"
	tmpfile, err := createTempFile("", "")
	defer os.Remove(tmpfile.Name())
	require.NoError(t, err)

	_, err = tmpfile.WriteString(logEntryString + "\n")
	require.NoError(t, err)

	tt := NewLogFile()
	tt.Log = TestLogger{t}
	tt.FileConfig = []FileConfig{{FilePath: tmpfile.Name(), FromBeginning: true}}
	tt.FileConfig[0].init()
	tt.started = true

	lsrcs := tt.FindLogSrc()
	if len(lsrcs) != 1 {
		t.Fatalf("%v log src was returned when 1 should be available", len(lsrcs))
	}

	ts := lsrcs[0].(*tailerSrc)
	ts.outputFn = func(e logs.LogEvent) {}

	go func() {
		time.Sleep(500 * time.Millisecond)
		if err := os.Remove(tmpfile.Name()); err != nil {
			t.Errorf("Failed to remove tmp file '%v': %v", tmpfile.Name(), err)
		}
	}()

	stopped := make(chan struct{})
	go func() {
		ts.runTail()
		close(stopped)
	}()

	select {
	case <-time.After(1 * time.Second):
		t.Errorf("tailerSrc should have stopped after tile is removed")
	case <-stopped:
	}

	tt.Stop()
}

//When another file is created for the same file config and the file config has auto_removal as true, the old files will stop at EOF and removed afterwards
func TestLogsFileAutoRemoval(t *testing.T) {
	var wg sync.WaitGroup

	multilineWaitPeriod = 10 * time.Millisecond
	logEntryString := "anything"
	filePrefix := "file_auto_removal"
	tmpfile1, err := createTempFile("", filePrefix)
	require.NoError(t, err)

	_, err = tmpfile1.WriteString(logEntryString + "\n")
	require.NoError(t, err)
	tmpfile1.Sync()
	tmpfile1.Close()

	tt := NewLogFile()
	tt.Log = TestLogger{t}
	tt.FileConfig = []FileConfig{{
		FilePath:      filepath.Join(filepath.Dir(tmpfile1.Name()), filePrefix+"*"),
		FromBeginning: true,
		AutoRemoval:   true,
	}}
	tt.FileConfig[0].init()
	tt.started = true

	lsrcs := tt.FindLogSrc()
	if len(lsrcs) != 1 {
		t.Fatalf("%v log src was returned when 1 should be available", len(lsrcs))
	}

	lsrc := lsrcs[0]
	evts := make(chan logs.LogEvent)
	lsrc.SetOutput(func(e logs.LogEvent) {
		if e != nil {
			evts <- e
		}
	})

	var tmpfile2 *os.File
	defer func() {
		if tmpfile2 != nil {
			os.Remove(tmpfile2.Name())
		}
	}()

	wg.Add(1)
	
	go func() {
		defer wg.Done()

		// create a new file matching configured pattern
		tmpfile2, err = createTempFile("", filePrefix)
		require.NoError(t, err)

		_, err = tmpfile2.WriteString(logEntryString + "\n")
		require.NoError(t, err)

	}()

	e := <-evts
	if e.Message() != logEntryString {
		t.Errorf("Wrong log found from first file: \n%v\nExpecting:\n%v\n", e.Message(), logEntryString)
	}
	defer lsrc.Stop()

	for {
		lsrcs = tt.FindLogSrc()
		if len(lsrcs) > 0 {
			break
		}
		time.Sleep(1 * time.Second)
	}

	lsrc = lsrcs[0]
	lsrc.SetOutput(func(e logs.LogEvent) {
		if e != nil {
			evts <- e
		}
	})

	e = <-evts
	if e.Message() != logEntryString {
		t.Errorf("Wrong log found from 2nd file: \n% x\nExpecting:\n% x\n", e.Message(), logEntryString)
	}

	//Use Wait Group to avoid race condition between opening tmpfile2 to delete tmpfile1 with auto_removal and opening tmpfile1
	//to check it exist
	wg.Wait()

	_, err = os.Open(tmpfile1.Name())
	assert.True(t, os.IsNotExist(err))

	lsrc.Stop()
	tt.Stop()
}

func TestLogsTimestampAsMultilineStarter(t *testing.T) {
	multilineWaitPeriod = 10 * time.Millisecond
	logEntryString := `15:04:05 18 Nov 2 multiline starter is in begining
append line
multiline starter is not in beginning 15:04:06 18 Nov 2
append line`
	tmpfile, err := createTempFile("", "")
	defer os.Remove(tmpfile.Name())
	require.NoError(t, err)

	_, err = tmpfile.WriteString(logEntryString + "\n")
	require.NoError(t, err)

	tt := NewLogFile()
	tt.Log = TestLogger{t}
	tt.FileConfig = []FileConfig{{
		FilePath:              tmpfile.Name(),
		FromBeginning:         true,
		TimestampRegex:        "(\\d{2}:\\d{2}:\\d{2} \\d{2} \\w{3} \\s{0,1}\\d{1,2})",
		TimestampLayout:       "15:04:05 06 Jan 2",
		MultiLineStartPattern: "{timestamp_regex}",
		Timezone:              time.UTC.String(),
	}}
	tt.FileConfig[0].init()
	tt.started = true

	lsrcs := tt.FindLogSrc()
	if len(lsrcs) != 1 {
		t.Fatalf("%v log src was returned when 1 should be available", len(lsrcs))
	}

	lsrc := lsrcs[0]
	evts := make(chan logs.LogEvent)
	lsrc.SetOutput(func(e logs.LogEvent) {
		evts <- e
	})

	e1 := "15:04:05 18 Nov 2 multiline starter is in begining\nappend line"
	et1 := time.Unix(1541171045, 0)
	e2 := "multiline starter is not in beginning 15:04:06 18 Nov 2\nappend line"
	et2 := time.Unix(1541171046, 0)

	e := <-evts
	if e.Message() != e1 && e.Time() != et1 {
		t.Errorf("Wrong multiline first log found: \n%v (%v)\nExpecting:\n%v (%v)\n", e.Message(), e.Time(), e1, et1)
	}

	e = <-evts
	if e.Message() != e2 && e.Time() != et2 {
		t.Errorf("Wrong multiline second log found: \n%v (%v)\nExpecting:\n%v (%v)\n", e.Message(), e.Time(), e2, et2)
	}

	lsrc.Stop()
	tt.Stop()
}

func TestLogsMultilineTimeout(t *testing.T) {
	// multline line starter as [^/s]
	logEntryString1 := `multiline begin
 append line
 append line`
	logEntryString2 := " append line"

	tmpfile, err := createTempFile("", "")
	defer os.Remove(tmpfile.Name())
	require.NoError(t, err)

	tt := NewLogFile()
	tt.Log = TestLogger{t}
	tt.FileConfig = []FileConfig{{FilePath: tmpfile.Name(), FromBeginning: true}}
	tt.FileConfig[0].init()
	tt.started = true

	lsrcs := tt.FindLogSrc()
	if len(lsrcs) != 1 {
		t.Fatalf("%v log src was returned when 1 should be available", len(lsrcs))
	}

	lsrc := lsrcs[0]
	evts := make(chan logs.LogEvent)
	lsrc.SetOutput(func(e logs.LogEvent) {
		evts <- e
	})

	go func() {
		_, err = tmpfile.WriteString(logEntryString1 + "\n")
		require.NoError(t, err)

		// sleep 5 second for multiline timeout
		time.Sleep(5 * time.Second)
		_, err = tmpfile.WriteString(logEntryString2 + "\n")
		require.NoError(t, err)
	}()

	e := <-evts
	if e.Message() != logEntryString1 {
		t.Errorf("Wrong multiline log found: \n%v\nExpecting:\n%v\n", e.Message(), logEntryString1)
	}

	e = <-evts
	if e.Message() != logEntryString2 {
		t.Errorf("Wrong multiline log found: \n% x\nExpecting:\n% x\n", e.Message(), logEntryString2)
	}

	lsrc.Stop()
	tt.Stop()
}

func TestLogsFileTruncate(t *testing.T) {
	multilineWaitPeriod = 10 * time.Millisecond
	lineBeforeFileTruncate := "lineBeforeFileTruncate"
	lineAfterFileTruncate := "lineAfterFileTruncate"

	tmpfile, err := createTempFile("", "")
	defer os.Remove(tmpfile.Name())
	require.NoError(t, err)

	tt := NewLogFile()
	tt.Log = TestLogger{t}
	tt.FileConfig = []FileConfig{{FilePath: tmpfile.Name(), FromBeginning: true}}
	tt.FileConfig[0].init()
	tt.started = true

	lsrcs := tt.FindLogSrc()
	if len(lsrcs) != 1 {
		t.Fatalf("%v log src was returned when 1 should be available", len(lsrcs))
	}

	lsrc := lsrcs[0]
	evts := make(chan logs.LogEvent)
	lsrc.SetOutput(func(e logs.LogEvent) {
		evts <- e
	})

	go func() {
		_, err = tmpfile.WriteString(lineBeforeFileTruncate + "\n")
		require.NoError(t, err)
		time.Sleep(1 * time.Second)

		// Truncate the file
		err = os.Truncate(tmpfile.Name(), 0)
		tmpfile, err = os.OpenFile(tmpfile.Name(), os.O_RDWR, 0600)
		require.NoError(t, err)
		_, err = tmpfile.WriteString(lineAfterFileTruncate + "\n")
		require.NoError(t, err)

	}()

	e := <-evts
	if e.Message() != lineBeforeFileTruncate {
		t.Errorf("Wrong log found before truncate: \n%v\nExpecting:\n%v\n", e.Message(), lineBeforeFileTruncate)
	}

	e = <-evts
	if e.Message() != lineAfterFileTruncate {
		t.Errorf("Wrong log found after truncate: \n% x\nExpecting:\n% x\n", e.Message(), lineAfterFileTruncate)
	}

	lsrc.Stop()
	tt.Stop()
}

func TestLogsFileWithOffset(t *testing.T) {
	multilineWaitPeriod = 10 * time.Millisecond
	logEntryString := "xxxxxxxxxxContentAfterOffset"

	tmpfile, err := createTempFile("", "")
	defer os.Remove(tmpfile.Name())
	require.NoError(t, err)

	stateDir, err := ioutil.TempDir("", "state")
	require.NoError(t, err)
	defer os.Remove(stateDir)

	stateFileName := filepath.Join(stateDir, escapeFilePath(tmpfile.Name()))
	stateFile, err := os.OpenFile(stateFileName, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	require.NoError(t, err)
	_, err = stateFile.WriteString("10")
	defer os.Remove(stateFileName)

	_, err = tmpfile.WriteString(logEntryString + "\n")
	require.NoError(t, err)

	tt := NewLogFile()
	tt.FileStateFolder = stateDir
	tt.Log = TestLogger{t}
	tt.FileConfig = []FileConfig{{FilePath: tmpfile.Name(), FromBeginning: true}}
	tt.FileConfig[0].init()
	tt.started = true

	lsrcs := tt.FindLogSrc()
	if len(lsrcs) != 1 {
		t.Fatalf("%v log src was returned when 1 should be available", len(lsrcs))
	}

	lsrc := lsrcs[0]
	evts := make(chan logs.LogEvent)
	lsrc.SetOutput(func(e logs.LogEvent) {
		evts <- e
	})

	e := <-evts
	el := "ContentAfterOffset"
	if e.Message() != el {
		t.Errorf("Wrong log found after offset: \n%v\nExpecting:\n%v\n", e.Message(), el)
	}

	lsrc.Stop()
	tt.Stop()

}

func TestLogsFileWithInvalidOffset(t *testing.T) {
	multilineWaitPeriod = 10 * time.Millisecond
	logEntryString := "xxxxxxxxxxContentAfterOffset"

	tmpfile, err := createTempFile("", "")
	defer os.Remove(tmpfile.Name())
	require.NoError(t, err)

	stateDir, err := ioutil.TempDir("", "state")
	require.NoError(t, err)
	defer os.Remove(stateDir)

	stateFileName := filepath.Join(stateDir, escapeFilePath(tmpfile.Name()))
	stateFile, err := os.OpenFile(stateFileName, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	require.NoError(t, err)
	_, err = stateFile.WriteString("100")
	defer os.Remove(stateFileName)

	_, err = tmpfile.WriteString(logEntryString + "\n")
	require.NoError(t, err)

	tt := NewLogFile()
	tt.FileStateFolder = stateDir
	tt.Log = TestLogger{t}
	tt.FileConfig = []FileConfig{{FilePath: tmpfile.Name(), FromBeginning: true}}
	tt.FileConfig[0].init()
	tt.started = true

	lsrcs := tt.FindLogSrc()
	if len(lsrcs) != 1 {
		t.Fatalf("%v log src was returned when 1 should be available", len(lsrcs))
	}

	lsrc := lsrcs[0]
	evts := make(chan logs.LogEvent)
	lsrc.SetOutput(func(e logs.LogEvent) {
		evts <- e
	})

	e := <-evts
	if e.Message() != logEntryString {
		t.Errorf("Wrong log found after offset: \n%v\nExpecting:\n%v\n", e.Message(), logEntryString)
	}

	lsrc.Stop()
	tt.Stop()
}

// TestLogsFileRecreate verifies that if a LogSrc matching a LogConfig is detected,
// We only receive log lines beginning at the offset specified in the corresponding state-file.
// And if the file happens to get deleted and recreated we expect to receive log lines beginning
// at that same offset in the state file.
func TestLogsFileRecreate(t *testing.T) {
	multilineWaitPeriod = 10 * time.Millisecond
	logEntryString := "xxxxxxxxxxContentAfterOffset"
	expectedContent := "ContentAfterOffset"

	tmpfile, err := createTempFile("", "")
	defer os.Remove(tmpfile.Name())
	require.NoError(t, err)
	_, err = tmpfile.WriteString(logEntryString + "\n")
	require.NoError(t, err)

	stateDir, err := ioutil.TempDir("", "state")
	require.NoError(t, err)
	defer os.Remove(stateDir)

	stateFileName := filepath.Join(stateDir, escapeFilePath(tmpfile.Name()))
	stateFile, err := os.OpenFile(stateFileName, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	require.NoError(t, err)
	_, err = stateFile.WriteString("10")
	defer os.Remove(stateFileName)

	tt := NewLogFile()
	tt.FileStateFolder = stateDir
	tt.Log = TestLogger{t}
	tt.FileConfig = []FileConfig{{FilePath: tmpfile.Name(), FromBeginning: true}}
	tt.FileConfig[0].init()
	tt.started = true

	lsrcs := tt.FindLogSrc()
	if len(lsrcs) != 1 {
		t.Fatalf("%v log src was returned when 1 should be available", len(lsrcs))
	}

	lsrc := lsrcs[0]
	evts := make(chan logs.LogEvent)
	lsrc.SetOutput(func(e logs.LogEvent) {
		if e != nil {
			evts <- e
		}
	})

	go func() {
		time.Sleep(1 * time.Second)

		// recreate file
		err = os.Remove(tmpfile.Name())
		require.NoError(t, err)
		require.NoError(t, tmpfile.Close())
		// 100 ms between deleting and recreating is enough on Linux and MacOS, but not Windows.
		time.Sleep(time.Second * 1)
		tmpfile, err = os.OpenFile(tmpfile.Name(), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
		require.NoError(t, err)

		_, err = tmpfile.WriteString(logEntryString + "\n")
		require.NoError(t, err)

	}()

	e := <-evts
	if e.Message() != expectedContent {
		t.Errorf("Wrong log found before file replacement: \n%v\nExpecting:\n%v\n", e.Message(), expectedContent)
	}
	defer lsrc.Stop()

	// Waiting 10 seconds for the recreated temp file to be detected is plenty sufficient on any OS.
	for start := time.Now(); time.Since(start) < 10*time.Second; {
		lsrcs = tt.FindLogSrc()
		if len(lsrcs) > 0 {
			break
		}
		time.Sleep(1 * time.Second)
	}
	if len(lsrcs) != 1 {
		t.Fatalf("%v log src was returned when 1 should be available", len(lsrcs))
	}
	lsrc = lsrcs[0]
	lsrc.SetOutput(func(e logs.LogEvent) {
		if e != nil {
			evts <- e
		}
	})

	e = <-evts
	if e.Message() != expectedContent {
		t.Errorf("Wrong log found after file replacement: \n% x\nExpecting:\n% x\n", e.Message(), expectedContent)
	}

	lsrc.Stop()
	tt.Stop()
}

func TestLogsPartialLineReading(t *testing.T) {
	multilineWaitPeriod = 10 * time.Millisecond
	logEntryPartialLine := "hello "
	logEntryComplish := " world"

	tmpfile, err := createTempFile("", "")
	defer os.Remove(tmpfile.Name())
	require.NoError(t, err)

	tt := NewLogFile()
	tt.Log = TestLogger{t}
	tt.FileConfig = []FileConfig{{FilePath: tmpfile.Name(), FromBeginning: true}}
	tt.FileConfig[0].init()
	tt.started = true

	lsrcs := tt.FindLogSrc()
	if len(lsrcs) != 1 {
		t.Fatalf("%v log src was returned when 1 should be available", len(lsrcs))
	}

	lsrc := lsrcs[0]
	evts := make(chan logs.LogEvent)
	lsrc.SetOutput(func(e logs.LogEvent) {
		evts <- e
	})

	go func() {
		// Write partial line
		_, err = tmpfile.WriteString(logEntryPartialLine)
		require.NoError(t, err)

		time.Sleep(1 * time.Second)

		// complete the line now
		_, err = tmpfile.WriteString(logEntryComplish + "\n")
		require.NoError(t, err)
	}()

	e := <-evts
	if e.Message() != logEntryPartialLine+logEntryComplish {
		t.Errorf("Wrong log found : \n%v\nExpecting:\n%v\n", e.Message(), logEntryPartialLine+logEntryComplish)
	}

	lsrc.Stop()
	tt.Stop()
}

func TestLogFileMultiLogsReading(t *testing.T) {
	multilineWaitPeriod = 10 * time.Millisecond
	logEntryString := "This is from Agent log"
	dir, e := ioutil.TempDir("", "test")
	require.NoError(t, e)
	defer os.Remove(dir)
	agentLog, err := createTempFile(dir, "test_agent.log")
	defer os.Remove(agentLog.Name())
	require.NoError(t, err)

	_, err = agentLog.WriteString(logEntryString + "\n")
	require.NoError(t, err)
	os.Remove(os.TempDir() + string(os.PathListSeparator) + "test_service.log*")
	serviceLog, err := createTempFile(dir, "test_service.log")
	defer os.Remove(serviceLog.Name())
	require.NoError(t, err)

	logEntryString = "This is from Service log"
	_, err = serviceLog.WriteString(logEntryString + "\n")
	require.NoError(t, err)

	tt := NewLogFile()
	tt.Log = TestLogger{t}
	tt.FileConfig = []FileConfig{{
		FilePath:         filepath.Dir(agentLog.Name()) + string(filepath.Separator) + "test_*",
		FromBeginning:    true,
		PublishMultiLogs: true,
	}}
	tt.FileConfig[0].init()
	tt.started = true

	var wg sync.WaitGroup
	lsrcs := tt.FindLogSrc()
	for _, lsrc := range lsrcs {
		wg.Add(1)
		switch lsrc.Group() {
		case generateLogGroupName(agentLog.Name()):
			lsrc.SetOutput(func(e logs.LogEvent) {
				if e != nil {
					if e.Message() != "This is from Agent log" {
						t.Errorf("Wrong agent log found : \n%v", e.Message())
					}
					wg.Done()
				}
			})
		case generateLogGroupName(serviceLog.Name()):
			lsrc.SetOutput(func(e logs.LogEvent) {
				if e != nil {
					if e.Message() != "This is from Service log" {
						t.Errorf("Wrong service log found : \n%v", e.Message())
					}
					wg.Done()
				}
			})
		default:
			t.Errorf("Invalid log group name %v found from logsrc", lsrc.Group())
		}
		defer lsrc.Stop()
	}
	wg.Wait()
	tt.Stop()
}

func TestLogFileMultiLogsReadingAddingFile(t *testing.T) {
	multilineWaitPeriod = 10 * time.Millisecond
	logEntryString := "This is from Agent log"
	dir, e := ioutil.TempDir("", "test")
	require.NoError(t, e)
	defer os.Remove(dir)

	agentLog, err := createTempFile(dir, "test_agent.log")
	defer os.Remove(agentLog.Name())
	require.NoError(t, err)

	tt := NewLogFile()
	tt.Log = TestLogger{t}
	tt.FileConfig = []FileConfig{{
		FilePath:         filepath.Dir(agentLog.Name()) + string(filepath.Separator) + "test_*",
		FromBeginning:    true,
		PublishMultiLogs: true,
	}}
	tt.FileConfig[0].init()
	tt.started = true

	var serviceLog *os.File
	defer func() {
		if serviceLog != nil {
			os.Remove(serviceLog.Name())
		}
	}()
	go func() {
		_, err := agentLog.WriteString(logEntryString + "\n")
		require.NoError(t, err)

		time.Sleep(2 * time.Second)

		serviceLog, err = createTempFile(dir, "test_service.log")
		require.NoError(t, err)

		logEntryString = "This is from Service log"
		_, err = serviceLog.WriteString(logEntryString + "\n")
		require.NoError(t, err)
	}()

	var wg sync.WaitGroup
	c := 0
	for c < 2 {
		lsrcs := tt.FindLogSrc()
		for _, lsrc := range lsrcs {
			wg.Add(1)
			switch lsrc.Group() {
			case generateLogGroupName(agentLog.Name()):
				lsrc.SetOutput(func(e logs.LogEvent) {
					if e != nil {
						if e.Message() != "This is from Agent log" {
							t.Errorf("Wrong agent log found : \n%v", e.Message())
						}
						wg.Done()
					}
				})
			default:
				lsrc.SetOutput(func(e logs.LogEvent) {
					if e != nil {
						if e.Message() != "This is from Service log" {
							t.Errorf("Wrong service log found : \n%v", e.Message())
						}
						wg.Done()
					}
				})
			}
			defer lsrc.Stop()
			c++
		}
		time.Sleep(1 * time.Second)
	}
	wg.Wait()
	tt.Stop()
}

func TestLogFileMultiLogsReadingWithBlacklist(t *testing.T) {
	multilineWaitPeriod = 10 * time.Millisecond
	logEntryString := "This is from Agent log"

	agentLog, err := createTempFile("", "test_agent.log")
	defer os.Remove(agentLog.Name())
	require.NoError(t, err)

	tt := NewLogFile()
	tt.Log = TestLogger{t}
	tt.FileConfig = []FileConfig{{
		FilePath:         filepath.Dir(agentLog.Name()) + string(filepath.Separator) + "test_*",
		FromBeginning:    true,
		PublishMultiLogs: true,
		Blacklist:        "^test_agent.log",
	}}
	tt.FileConfig[0].init()
	tt.started = true

	var serviceLog *os.File
	defer func() {
		if serviceLog != nil {
			os.Remove(serviceLog.Name())
		}
	}()
	go func() {
		_, err := agentLog.WriteString(logEntryString + "\n")
		require.NoError(t, err)

		time.Sleep(2 * time.Second)

		serviceLog, err = createTempFile("", "test_service.log")
		require.NoError(t, err)

		logEntryString = "This is from Service log"
		_, err = serviceLog.WriteString(logEntryString + "\n")
		require.NoError(t, err)
	}()

	var wg sync.WaitGroup
	c := 0
	for c < 4 {
		lsrcs := tt.FindLogSrc()
		for _, lsrc := range lsrcs {
			switch lsrc.Group() {
			case agentLog.Name():
				t.Errorf("Agent log should be blacklisted, but found : \n%v", lsrc.Group())
			default:
				wg.Add(1)
				lsrc.SetOutput(func(e logs.LogEvent) {
					if e != nil {
						if e.Message() != "This is from Service log" {
							t.Errorf("Wrong service log found : \n%v", e.Message())
						}
						wg.Done()
					}
				})
			}
			defer lsrc.Stop()
		}
		time.Sleep(1 * time.Second)
		c++
	}
	wg.Wait()
	tt.Stop()
}

func TestGenerateLogGroupName(t *testing.T) {
	multilineWaitPeriod = 10 * time.Millisecond
	fileName := "C:\\tmp\\soak Test\\tmp0.log"
	expectLogGroup := "C_/tmp/soak_Test/tmp0.log"
	logGroupName := generateLogGroupName(fileName)
	assert.Equal(t, expectLogGroup, logGroupName, fmt.Sprintf(
		"The log group name %s is not the same as %s.",
		logGroupName,
		expectLogGroup))

	fileName = "C:\\tes:t/fol,de%r\\tm p"
	expectLogGroup = "C_/tes_t/fol_de_r/tm_p"
	logGroupName = generateLogGroupName(fileName)
	assert.Equal(t, expectLogGroup, logGroupName, fmt.Sprintf(
		"The log group name %s is not the same as %s.",
		logGroupName,
		expectLogGroup))
}

func TestCheckForDuplicateRetentionSettingsPanics(t *testing.T) {
	tt := NewLogFile()
	logGroupName := "DuplicateLogGroupName"
	tt.FileConfig = []FileConfig{{
		FilePath:        "SampleFilePath",
		FromBeginning:   true,
		LogGroupName:    logGroupName,
		RetentionInDays: 1,
	},
		{
			FilePath:        "SampleFilePath",
			FromBeginning:   true,
			LogGroupName:    logGroupName,
			RetentionInDays: 3,
		},
		{
			FilePath:        "SampleFilePath",
			FromBeginning:   true,
			LogGroupName:    logGroupName,
			RetentionInDays: 3,
		},
	}
	assert.Panics(t, func() { tt.checkForDuplicateRetentionSettings() }, "Did not panic after finding duplicate log group")
}

func TestCheckForDuplicateRetentionSettingsWithDefaultRetention(t *testing.T) {
	tt := NewLogFile()
	logGroupName := "DuplicateLogGroupName"
	tt.FileConfig = []FileConfig{{
		FilePath:        "SampleFilePath",
		FromBeginning:   true,
		LogGroupName:    logGroupName,
		RetentionInDays: -1,
	},
		{
			FilePath:        "SampleFilePath",
			FromBeginning:   true,
			LogGroupName:    logGroupName,
			RetentionInDays: -1,
		},
	}
	assert.NotPanics(t, func() { tt.checkForDuplicateRetentionSettings() }, "Panicked when no duplicate retention settings exists")
}

func TestCheckForDuplicateRetentionWithDefaultAndNonDefaultValue(t *testing.T) {
	tt := NewLogFile()
	logGroupName := "DuplicateLogGroupName"
	tt.FileConfig = []FileConfig{{
		FilePath:        "SampleFilePath",
		FromBeginning:   true,
		LogGroupName:    logGroupName,
		RetentionInDays: -1,
	},
		{
			FilePath:        "SampleFilePath",
			FromBeginning:   true,
			LogGroupName:    logGroupName,
			RetentionInDays: 3,
		},
	}
	assert.NotPanics(t, func() { tt.checkForDuplicateRetentionSettings() }, "Panicked when no duplicate retention settings exists")
}

func TestCheckForDuplicateRetentionSettingsDifferentLogGroups(t *testing.T) {
	tt := NewLogFile()
	tt.FileConfig = []FileConfig{{
		FilePath:        "SampleFilePath",
		FromBeginning:   true,
		LogGroupName:    "logGroupName1",
		RetentionInDays: 5,
	},
		{
			FilePath:        "SampleFilePath",
			FromBeginning:   true,
			LogGroupName:    "logGroupName2",
			RetentionInDays: 3,
		},
	}
	assert.NotPanics(t, func() { tt.checkForDuplicateRetentionSettings() }, "Panicked when no duplicate retention settings exists")
}

func TestCheckDuplicateRetentionWithDefaultAndSetLogGroups(t *testing.T) {
	tt := NewLogFile()
	logGroupName := "DuplicateLogGroupName"
	tt.FileConfig = []FileConfig{{
		FilePath:        "SampleFilePath",
		FromBeginning:   true,
		LogGroupName:    logGroupName,
		RetentionInDays: 3,
	},
		{
			FilePath:        "SampleFilePath",
			FromBeginning:   true,
			LogGroupName:    logGroupName,
			RetentionInDays: 5,
		},
		{
			FilePath:        "SampleFilePath",
			FromBeginning:   true,
			LogGroupName:    logGroupName,
			RetentionInDays: -1,
		},
	}
	assert.Panics(t, func() { tt.checkForDuplicateRetentionSettings() }, "Did not panic after finding duplicate log group")
}

func TestCheckForDuplicateRetentionSettingsWithDefault(t *testing.T) {
	tt := NewLogFile()
	logGroupName := "DuplicateLogGroupName"
	tt.FileConfig = []FileConfig{{
		FilePath:        "SampleFilePath",
		FromBeginning:   true,
		LogGroupName:    logGroupName,
		RetentionInDays: 5,
	},
		{
			FilePath:        "SampleFilePath",
			FromBeginning:   true,
			LogGroupName:    logGroupName,
			RetentionInDays: 5,
		},
		{
			FilePath:        "SampleFilePath",
			FromBeginning:   true,
			LogGroupName:    logGroupName,
			RetentionInDays: 5,
		},
	}
	assert.NotPanics(t, func() { tt.checkForDuplicateRetentionSettings() }, "Panicked when no duplicate retention settings exists")
	assert.Equal(t, tt.FileConfig[0].RetentionInDays, 5)
	assert.Equal(t, tt.FileConfig[1].RetentionInDays, -1)
	assert.Equal(t, tt.FileConfig[2].RetentionInDays, -1)
}
