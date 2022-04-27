package tail

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"testing"
	"time"
)

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

// TestNotTailedCompletelyLogging verifies that the tailer
// logs an error if the tailer knows that it is not done reading the
// file but is exiting.
// Deprecated: This relies on the `ReOpen` flag being set to false for the
// tailer. Leaving this to illustrate the old functionality
func TestNotTailedCompletelyLogging(t *testing.T) {
	tmpfile, tail, tlog := setup(t)
	defer tearDown(tmpfile)
	go tail.exitOnDeletion() // see deprecation notice

	readThreeLines(t, tail)

	// Then remove the tmpfile
	if err := os.Remove(tmpfile.Name()); err != nil {
		t.Fatalf("failed to remove temporary log file %v: %v", tmpfile.Name(), err)
	}
	// Wait until the tailer should have been terminated
	time.Sleep(exitOnDeletionWaitDuration + exitOnDeletionCheckDuration + 1*time.Second)

	verifyTailerLogging(t, tlog, "File "+tmpfile.Name()+" was deleted, but file content is not tailed completely.")
	verifyTailerExited(t, tail)
}

func TestDroppedLinesWhenStopAtEOFLogging(t *testing.T) {
	tmpfile, tail, tlog := setup(t)
	defer tearDown(tmpfile)

	readThreeLines(t, tail)

	// Ask the tailer to StopAtEOF
	tail.StopAtEOF()
	// Then remove the tmpfile
	if err := os.Remove(tmpfile.Name()); err != nil {
		t.Fatalf("failed to remove temporary log file %v: %v", tmpfile.Name(), err)
	}
	// Wait until the tailer should have been terminated
	time.Sleep(exitOnDeletionWaitDuration + exitOnDeletionCheckDuration + 1*time.Second)

	verifyTailerLogging(t, tlog, "Dropped 7 lines for stopped tail for file "+tmpfile.Name())
	verifyTailerExited(t, tail)
}

func TestReopenExhaustsRetries(t *testing.T) {
	tmpfile, err := ioutil.TempFile("", "example")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer tearDown(tmpfile)

	// Write the file content
	for i := 0; i < 10; i++ {
		if _, err := fmt.Fprintf(tmpfile, "%v some log line\n", time.Now()); err != nil {
			log.Fatal(err)
		}
	}

	// Setup the tail
	var tl testLogger
	tail, err := TailFile(tmpfile.Name(), Config{
		Logger: &tl,
		ReOpen: true,
		Follow: true,
	})
	if err != nil {
		t.Fatalf("failed to tail file %v: %v", tmpfile.Name(), err)
	}
	defer tail.Stop()

	// force the file permission of the tailed file to not be readable.
	// this has to be done before closing the file
	err = tmpfile.Chmod(0)
	if err != nil {
		t.Fatalf("%v", err)
	}

	if err = tmpfile.Close(); err != nil {
		log.Fatal(err)
	}

	err = tail.reopen()
	if err == nil {
		t.Fatal("Expected an error when reopening the file")
	}

	// the root error message differs on OS, ("permission denied" on Linux vs "access denied" on Windows)
	// so use the beginning of the error message that we expect to return
	if !strings.Contains(err.Error(), "Unable to open file") {
		t.Fatal("Expected an error that indicates that the tailer could not open the file for tailing")
	}

	cnt := 0
	hasExhausted := false
	for _, l := range tl.debugs {
		if strings.Contains(l, "and retrying") {
			cnt += 1
		}
		if !hasExhausted && strings.Contains(l, "Retried 5/5 times so far") {
			hasExhausted = true
		}
	}
	// Not an exact check here because the async tail process tries to monitor the file
	// more than just the one time we explicitly call reopen(). The number of retrying logs
	// is not consistent, test over test, but we expect
	if cnt < fileOpenMaxRetries {
		t.Errorf("Did not execute the expected %d retries", fileOpenMaxRetries)
	}
	if !hasExhausted {
		t.Errorf("Expected to emit a debug log that the tailer retried the max %d times", fileOpenMaxRetries)
	}
}

func setup(t *testing.T) (*os.File, *Tail, *testLogger) {
	tmpfile, err := ioutil.TempFile("", "example")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	// Write the file content
	for i := 0; i < 10; i++ {
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
		ReOpen: true,
		Follow: true,
	})
	if err != nil {
		t.Fatalf("failed to tail file %v: %v", tmpfile.Name(), err)
	}

	return tmpfile, tail, &tl
}

func readThreeLines(t *testing.T, tail *Tail) {
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
