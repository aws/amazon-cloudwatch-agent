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

func TestDroppedLinesWhenStopAtEOFLogging(t *testing.T) {
	tmpfile, tail, tlog := setup(t)
	defer tearDown(tmpfile)

	readThreelines(t, tail)

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

func TestIgnoreSymLinksOfInvalidPath(t *testing.T) {
	var tl testLogger
	tail, err := TailFile("._noexitent_file", Config{
		Logger: &tl,
		ReOpen: false,
		Follow: true,
		IgnoreSymLinks: true,
	})
	if tail !=nil || err == nil {
    t.Fatal("If symbolic link are not allowed, tailing an invalid file should fail")
	}
  if err.Error() != "cannot get file info." {
    t.Fatal("Mismatch in error message while getting file info")
  }
}

func TestIgnoreSymLinksOfRegularFile(t *testing.T) {
  tmpfile := setupTmpFile(t)
	defer tearDown(tmpfile)

	var tl testLogger
	tail, err := TailFile(tmpfile.Name(), Config{
		Logger: &tl,
		ReOpen: false,
		Follow: true,
		IgnoreSymLinks: true,
	})
	if tail == nil || err != nil {
		t.Fatalf("failed to tail file %v: %v", tmpfile.Name(), err)
	}
}

func TestIgnoreSymLinksOfSymbolicLink(t *testing.T) {
  tmpfile := setupTmpFile(t)
	defer tearDown(tmpfile)

  var symlink = tmpfile.Name() + ".symlink"
  err := os.Symlink(tmpfile.Name(), symlink)
	if err != nil {
		t.Fatalf("failed to create symbolic link %v: %v", symlink, err)
	}
	defer deleteSymLink(symlink)

	var tl testLogger
	tail, err := TailFile(symlink, Config{
		Logger: &tl,
		ReOpen: false,
		Follow: true,
		IgnoreSymLinks: true,
	})

	if tail != nil || err == nil {
    t.Fatal("If symbolic link are not allowed, tailing a symbolic link should fail")
	}
  if err.Error() != "symbolic links not allowed." {
    t.Fatal("Mismatch in error message while trying to tail a symbolic link")
  }
}

func setupTmpFile(t *testing.T) (*os.File) {
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
  return tmpfile;
}

func setup(t *testing.T) (*os.File, *Tail, *testLogger) {
	tmpfile := setupTmpFile(t)
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

func deleteSymLink(symlik string) {
	os.Remove(symlik)
}
