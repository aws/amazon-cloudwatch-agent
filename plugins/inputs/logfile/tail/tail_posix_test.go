//go:build linux || darwin || freebsd || netbsd || openbsd
// +build linux darwin freebsd netbsd openbsd

package tail

import (
	"fmt"
	"io/ioutil"
	"strings"
	"testing"
	"time"
)

// os.Chmod() is not supported on Windows
func TestReopenExhaustsRetries(t *testing.T) {
	tmpfile, err := ioutil.TempFile("", "example")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer tearDown(tmpfile)

	// Write the file content
	for i := 0; i < 10; i++ {
		if _, err := fmt.Fprintf(tmpfile, "%v some log line\n", time.Now()); err != nil {
			t.Fatal(err)
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
		t.Fatal(err)
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
