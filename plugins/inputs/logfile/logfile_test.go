package logfile

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
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
func (tl TestLogger) Debugf(format string, args ...interface{}) {}
func (tl TestLogger) Debug(args ...interface{})                 {}
func (tl TestLogger) Warnf(format string, args ...interface{})  {}
func (tl TestLogger) Warn(args ...interface{})                  {}
func (tl TestLogger) Infof(format string, args ...interface{})  {}
func (tl TestLogger) Info(args ...interface{})                  {}

func TestLogs(t *testing.T) {
	logEntryString := "cpu,mytag=foo usage_idle=100"
	tmpfile, err := ioutil.TempFile("", "")
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
	//2 * rune_len when it is coded in gbk encoding.
	logEntryString := "测试"
	tmpfile, err := ioutil.TempFile("", "")
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
	f, err := ioutil.TempFile("", "")
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
		if e.Message() != expect {
			t.Errorf("Log message does not match expectation, expect %q but found %q", expect, e.Message())
		}
	}

	lsrc.Stop()
	tt.Stop()

}

func TestCompressedFile(t *testing.T) {
	filepath := "/tmp/logfile.log"
	compressed := isCompressedFile(filepath)
	assert.False(t, compressed, "This should not be a compressed file.")
	filepath = "/tmp/logfile.log.gz"
	compressed = isCompressedFile(filepath)
	assert.True(t, compressed, "This should be a compressed file.")
}

func TestRestoreState(t *testing.T) {
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
	tmpfile1, err := ioutil.TempFile("", "tmp1_")
	defer os.Remove(tmpfile1.Name())
	require.NoError(t, err)

	_, err = tmpfile1.WriteString("1\n")
	require.NoError(t, err)

	//make file stat reflect the diff of file ModTime
	time.Sleep(time.Second * 2)

	tmpfile2, err := ioutil.TempFile("", "tmp2_")
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
	logEntryString := "multiline begin1\n append line1\nmultiline begin2\n append line2"
	tmpfile, err := ioutil.TempFile("", "")
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
