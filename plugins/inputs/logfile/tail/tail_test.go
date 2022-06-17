package tail

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"
	"github.com/aws/amazon-cloudwatch-agent/internal/semaphore"
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
	err := os.Remove(tmpfile.Name())
	assert.NoError(t, err)

	// Wait until the tailer should have been terminated
	time.Sleep(exitOnDeletionWaitDuration + exitOnDeletionCheckDuration + 1*time.Second)

	verifyTailerLogging(t, tlog, "File "+tmpfile.Name()+" was deleted, but file content is not tailed completely.")
	verifyTailerExited(t, tail)
}

func TestStopAtEOF(t *testing.T) {
	tmpfile, tail, _ := setup(t)
	defer tearDown(tmpfile)

	readThreelines(t, tail)

	// Since StopAtEOF() will block until the EOF is reached, run it in a goroutine.
	done := make(chan bool)
	go func() {
		tail.StopAtEOF()
		close(done)
	}()

	// Verify the goroutine is blocked indefinitely.
	select {
	case <-done:
		t.Fatalf("StopAtEOF() completed unexpectedly")
	case <- time.After(time.Second * 1):
		t.Log("timeout waiting for StopAtEOF() (as expected)")
	}

	assert.Equal(t, errStopAtEOF, tail.Err())

	// Read to EOF
	for i := 0; i < linesWrittenToFile - 3; i++ {
		<-tail.Lines
	}

	// Verify StopAtEOF() has completed.
	select {
	case <-done:
		t.Log("StopAtEOF() completed (as expected)")
	case <-time.After(time.Second * 1):
		t.Fatalf("StopAtEOF() has not completed")
	}

	// Then remove the tmpfile
	err := os.Remove(tmpfile.Name())
	assert.NoError(t, err)

	verifyTailerExited(t, tail)
}

func setup(t *testing.T) (*os.File, *Tail, *testLogger) {
	tmpfile, err := ioutil.TempFile("", "example")
	assert.NoError(t, err)

	// Write the file content
	for i := 0; i < linesWrittenToFile; i++ {
		_, err := fmt.Fprintf(tmpfile, "%v some log line\n", time.Now())
		assert.NoError(t, err)
	}

	err = tmpfile.Close()
	assert.NoError(t, err)

	// Modify the exit on deletion wait to reduce test length
	exitOnDeletionCheckDuration = 100 * time.Millisecond
	exitOnDeletionWaitDuration = 500 * time.Millisecond

	// Setup the tail
	var tl testLogger
	numUsedFds := semaphore.NewSemaphore(1)
	tail, err := TailFile(tmpfile.Name(), numUsedFds,
		Config{
			Logger: &tl,
			ReOpen: false,
			Follow: true,
		})
	assert.NoError(t, err)

	//Increase the slots in semaphore by 1 to later confirmed if the slots in semaphore has been released yet in line 180
	ok := tail.numUsedFds.Acquire(time.Second)
	assert.True(t, ok)
	
	return tmpfile, tail, &tl
}

func readThreelines(t *testing.T, tail *Tail) {
	for i := 0; i < 3; i++ {
		line := <-tail.Lines
		assert.NoError(t, line.Err)

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
		//Ensure all the tailers are released when signal dead.
		assert.Equal(t, 0, tail.numUsedFds.GetCount())
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
