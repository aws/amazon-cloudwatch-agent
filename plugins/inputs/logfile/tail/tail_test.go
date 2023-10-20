// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package tail

import (
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const linesWrittenToFile int = 10

type testLogger struct {
	debugs, infos, warns, errors []string
}

func (l *testLogger) Errorf(format string, args ...interface{}) {
	line := fmt.Sprintf(format, args...)
	l.errors = append(l.errors, line)
}

func (l *testLogger) Error(args ...interface{}) {
	line := fmt.Sprint(args...)
	l.errors = append(l.errors, line)
}

func (l *testLogger) Debugf(format string, args ...interface{}) {
	line := fmt.Sprintf(format, args...)
	l.debugs = append(l.debugs, line)
}

func (l *testLogger) Debug(args ...interface{}) {
	line := fmt.Sprint(args...)
	l.debugs = append(l.debugs, line)
}

func (l *testLogger) Warnf(format string, args ...interface{}) {
	line := fmt.Sprintf(format, args...)
	l.warns = append(l.warns, line)
}

func (l *testLogger) Warn(args ...interface{}) {
	line := fmt.Sprint(args...)
	l.warns = append(l.warns, line)
}

func (l *testLogger) Infof(format string, args ...interface{}) {
	line := fmt.Sprintf(format, args...)
	l.infos = append(l.infos, line)
}

func (l *testLogger) Info(args ...interface{}) {
	line := fmt.Sprint(args...)
	l.infos = append(l.infos, line)
}

func TestNotTailedCompeletlyLogging(t *testing.T) {
	tmpfile, tail, tlog := setup(t)
	defer tearDown(tmpfile)

	readThreelines(t, tail)

	// Then remove the tmpfile
	if err := os.Remove(tmpfile.Name()); err != nil {
		t.Fatalf("failed to remove temporary log file %v: %v", tmpfile.Name(), err)
	}
	// Wait until the tailer should have been terminated
	time.Sleep(exitOnDeletionWaitDuration + exitOnDeletionCheckDuration + 1*time.Second)

	verifyTailerLogging(t, tlog, "File "+tmpfile.Name()+" was deleted, but file content is not tailed completely.")
	verifyTailerExited(t, tail)
}

func TestStopAtEOF(t *testing.T) {
	tmpfile, tail, _ := setup(t)
	defer tearDown(tmpfile)

	readThreelines(t, tail)

	// Since tail.Wait() will block until the EOF is reached, run it in a goroutine.
	done := make(chan bool)
	go func() {
		tail.StopAtEOF()
		tail.Wait()
		close(done)
	}()

	// Verify the goroutine is blocked indefinitely.
	select {
	case <-done:
		t.Fatalf("tail.Wait() completed unexpectedly")
	case <-time.After(time.Second * 1):
		t.Log("timeout waiting for tail.Wait() (as expected)")
	}

	assert.Equal(t, errStopAtEOF, tail.Err())

	// Read to EOF
	for i := 0; i < linesWrittenToFile-3; i++ {
		<-tail.Lines
	}

	// Verify tail.Wait() has completed.
	select {
	case <-done:
		t.Log("tail.Wait() completed (as expected)")
	case <-time.After(time.Second * 1):
		t.Fatalf("tail.Wait() has not completed")
	}

	// Then remove the tmpfile
	if err := os.Remove(tmpfile.Name()); err != nil {
		t.Fatalf("failed to remove temporary log file %v: %v", tmpfile.Name(), err)
	}
	verifyTailerExited(t, tail)
}

func setup(t *testing.T) (*os.File, *Tail, *testLogger) {
	tmpfile, err := os.CreateTemp("", "example")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	// Write the file content
	for i := 0; i < linesWrittenToFile; i++ {
		if _, err := fmt.Fprintf(tmpfile, "%v some log line\n", time.Now()); err != nil {
			log.Fatal(err)
		}
	}
	if err := tmpfile.Close(); err != nil {
		log.Fatal(err)
	}

	// Modify the exit on deletion wait to reduce test length
	exitOnDeletionCheckDuration = 100 * time.Millisecond
	exitOnDeletionWaitDuration = 500 * time.Millisecond

	// Setup the tail
	var tl testLogger
	tail, err := TailFile(tmpfile.Name(), Config{
		Logger: &tl,
		ReOpen: false,
		Follow: true,
	})
	if err != nil {
		t.Fatalf("failed to tail file %v: %v", tmpfile.Name(), err)
	}
	// Cannot expect OpenFileCount to be 1 because the TailFile struct
	// was not created with MustExist=true, so file may not yet be opened.
	return tmpfile, tail, &tl
}

func readThreelines(t *testing.T, tail *Tail) {
	for i := 0; i < 3; i++ {
		line := <-tail.Lines
		if line.Err != nil {
			t.Errorf("error tailing test file: %v", line.Err)
			continue
		}
		if !strings.HasSuffix(line.Text, "some log line") {
			t.Errorf("wrong line from tail found: '%v'", line.Text)
		}
	}
	// If file was readable, then expect it to exist.
	assert.Equal(t, int64(1), OpenFileCount.Load())
}

func verifyTailerLogging(t *testing.T, tlog *testLogger, expectedErrorMsg string) {
	if len(tlog.errors) == 0 {
		t.Errorf("No error logs found: %v", tlog.errors)
		return
	}

	if tlog.errors[0] != expectedErrorMsg {
		t.Errorf("Incorrect error message for incomplete tail of file:\nExpecting: %v\nFound    : '%v'", expectedErrorMsg, tlog.errors[0])
	}
}

func verifyTailerExited(t *testing.T, tail *Tail) {
	select {
	case <-tail.Dead():
		assert.Equal(t, int64(0), OpenFileCount.Load())
		return
	default:
		t.Errorf("Tailer is still alive after file removed and wait period")
	}
}

func tearDown(tmpfile *os.File) {
	os.Remove(tmpfile.Name())
	exitOnDeletionCheckDuration = time.Minute
	exitOnDeletionWaitDuration = 5 * time.Minute
}
