// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package tail

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/plugins/inputs/logfile/constants"
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
		line := <-tail.Lines
		tail.ReleaseLine(line)
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
			tail.ReleaseLine(line) // Release even on error
			continue
		}
		if !strings.HasSuffix(line.Text, "some log line") {
			t.Errorf("wrong line from tail found: '%v'", line.Text)
		}
		tail.ReleaseLine(line) // Release line back to pool
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

func TestUtf16LineSize(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	// Create a UTF-16 BOM
	_, err = tmpfile.Write([]byte{0xFE, 0xFF})
	require.NoError(t, err)

	// Create a tail with a small MaxLineSize
	maxLineSize := 100
	tail, err := TailFile(tmpfile.Name(), Config{
		MaxLineSize: maxLineSize,
		Follow:      true,
		ReOpen:      false,
		Poll:        true,
	})
	require.NoError(t, err)
	defer tail.Stop()

	// Write a UTF-16 encoded line that exceeds MaxLineSize when decoded
	// Each 'a' will be 2 bytes in UTF-16
	utf16Line := make([]byte, 0, maxLineSize*4)
	for i := 0; i < maxLineSize*2; i++ {
		utf16Line = append(utf16Line, 0x00, 'a')
	}
	utf16Line = append(utf16Line, 0x00, '\n')

	_, err = tmpfile.Write(utf16Line)
	require.NoError(t, err)
	err = tmpfile.Sync()
	require.NoError(t, err)

	// Read the line and verify it's truncated
	select {
	case line := <-tail.Lines:
		// The line should be truncated to maxLineSize
		assert.LessOrEqual(t, len(line.Text), maxLineSize)
		tail.ReleaseLine(line)
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for line")
	}
}

func TestTail_DefaultBuffer(t *testing.T) {
	// Test that default buffer works with normal-sized log lines
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "test.log")

	// Create a file with a normal-sized line (1KB - well within default buffer)
	normalContent := strings.Repeat("b", 1024) // 1KB
	err := os.WriteFile(filename, []byte(normalContent+"\n"), 0600)
	require.NoError(t, err)

	tail, err := TailFile(filename, Config{
		Follow:    false,
		MustExist: true,
		// MaxLineSize not set - should use default buffer
	})
	require.NoError(t, err)
	defer tail.Stop()

	select {
	case line := <-tail.Lines:
		assert.NoError(t, line.Err)
		assert.Equal(t, normalContent, line.Text)
		tail.ReleaseLine(line)
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for line")
	}
}

func TestTail_1MBWithExplicitMaxLineSize(t *testing.T) {
	// Test that large lines work when MaxLineSize is explicitly set
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "test.log")

	// Create a file with a 512KB line
	largeContent := strings.Repeat("b", 512*1024) // 512KB
	err := os.WriteFile(filename, []byte(largeContent+"\n"), 0600)
	require.NoError(t, err)

	tail, err := TailFile(filename, Config{
		Follow:      false,
		MustExist:   true,
		MaxLineSize: constants.DefaultMaxEventSize, // Explicitly set 1MB buffer
	})
	require.NoError(t, err)
	defer tail.Stop()

	select {
	case line := <-tail.Lines:
		assert.NoError(t, line.Err)
		assert.Equal(t, largeContent, line.Text)
		tail.ReleaseLine(line)
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for line")
	}
}

// TestLinePooling verifies that Line objects are properly pooled and reused
func TestLinePooling(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "pool_test")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	content := "line1\nline2\nline3\n"
	err = os.WriteFile(tmpfile.Name(), []byte(content), 0600)
	require.NoError(t, err)

	tail, err := TailFile(tmpfile.Name(), Config{
		Follow:    false,
		MustExist: true,
	})
	require.NoError(t, err)
	defer tail.Stop()

	var lines []*Line
	for i := 0; i < 3; i++ {
		select {
		case line := <-tail.Lines:
			lines = append(lines, line)
		case <-time.After(time.Second):
			t.Fatal("Timeout waiting for line")
		}
	}

	assert.Equal(t, "line1", lines[0].Text)
	assert.Equal(t, "line2", lines[1].Text)
	assert.Equal(t, "line3", lines[2].Text)

	// Release all lines back to pool
	for _, line := range lines {
		tail.ReleaseLine(line)
	}

	// Line object should be zeroed out because we released it
	pooledLine := tail.linePool.Get().(*Line)
	assert.Empty(t, pooledLine.Text, "Pooled line should remain zeroed")
	assert.Empty(t, pooledLine.Time, "Pooled line should remain zeroed")
	assert.Empty(t, pooledLine.Err, "Pooled line should remain zeroed")
	assert.Empty(t, pooledLine.Offset, "Pooled line should remain zeroed")
	tail.ReleaseLine(pooledLine)
}

// TestConcurrentLinePoolAccess tests that the line pool is thread-safe
func TestConcurrentLinePoolAccess(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "concurrent_test")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	// Create content with multiple lines
	numLines := 100
	content := strings.Repeat("concurrent test line\n", numLines)
	err = os.WriteFile(tmpfile.Name(), []byte(content), 0600)
	require.NoError(t, err)

	tail, err := TailFile(tmpfile.Name(), Config{
		Follow:    false,
		MustExist: true,
	})
	require.NoError(t, err)
	defer tail.Stop()

	// Process lines concurrently
	var wg sync.WaitGroup
	linesChan := make(chan *Line, numLines)

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < numLines; i++ {
			select {
			case line := <-tail.Lines:
				linesChan <- line
			case <-time.After(5 * time.Second):
				t.Errorf("Timeout waiting for line %d", i)
				return
			}
		}
		close(linesChan)
	}()

	numWorkers := 5
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for line := range linesChan {
				assert.Equal(t, "concurrent test line", line.Text)
				tail.ReleaseLine(line) // Release back to pool
			}
		}()
	}

	wg.Wait()
}

// TestDynamicBufferSmallLines tests that small lines use the default small buffer
func TestDynamicBufferSmallLines(t *testing.T) {
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "small_lines.log")

	// Create file with small lines (well within 256KB default buffer)
	smallLine := strings.Repeat("a", 1024) // 1KB line
	content := smallLine + "\n" + smallLine + "\n"
	err := os.WriteFile(filename, []byte(content), 0600)
	require.NoError(t, err)

	tail, err := TailFile(filename, Config{
		Follow:      false,
		MustExist:   true,
		MaxLineSize: constants.DefaultMaxEventSize, // 1MB max
	})
	require.NoError(t, err)
	defer tail.Stop()

	// Read first line to ensure reader is initialized
	select {
	case line := <-tail.Lines:
		assert.NoError(t, line.Err)
		assert.Equal(t, smallLine, line.Text)
		tail.ReleaseLine(line)
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for first line")
	}

	// Now check buffer size after reader is initialized
	assert.Equal(t, constants.DefaultReaderBufferSize, tail.reader.Size(), "Should use default 256KB buffer for small lines")
	assert.False(t, tail.useLargeBuffer, "Should not be using large buffer for small lines")

	// Read second line
	select {
	case line := <-tail.Lines:
		assert.NoError(t, line.Err)
		assert.Equal(t, smallLine, line.Text)
		tail.ReleaseLine(line)
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for second line")
	}

	// Verify buffer is still small after reading small lines
	assert.Equal(t, constants.DefaultReaderBufferSize, tail.reader.Size(), "Should still use 256KB buffer for small lines")
	assert.False(t, tail.useLargeBuffer, "Should still not be using large buffer")
}

// TestDynamicBufferLargeLineUpgrade tests buffer upgrade when encountering large lines
func TestDynamicBufferLargeLineUpgrade(t *testing.T) {
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "large_line.log")

	// Create file with multiple large lines to test buffer upgrade
	largeLine := strings.Repeat("b", 300*1024)                                 // 300KB line
	nearlyOneMBLine := strings.Repeat("x", constants.DefaultMaxEventSize-1024) // Nearly 1MB line (1MB - 1KB)
	content := largeLine + "\nafter large line\n" + nearlyOneMBLine + "\nafter nearly 1MB line\n"
	err := os.WriteFile(filename, []byte(content), 0600)
	require.NoError(t, err)

	tail, err := TailFile(filename, Config{
		Follow:      false,
		MustExist:   true,
		MaxLineSize: constants.DefaultMaxEventSize, // 1MB max
	})
	require.NoError(t, err)
	defer tail.Stop()

	// Read first large line (300KB) - should trigger buffer upgrade
	select {
	case line := <-tail.Lines:
		assert.NoError(t, line.Err)
		assert.Equal(t, largeLine, line.Text)
		assert.Equal(t, 300*1024, len(line.Text), "First line should be 300KB")
		tail.ReleaseLine(line)
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for large line")
	}

	// Verify buffer was upgraded after reading large line
	assert.Equal(t, constants.DefaultMaxEventSize, tail.reader.Size(), "Should upgrade to 1MB buffer after large line")
	assert.True(t, tail.useLargeBuffer, "Should be using large buffer after upgrade")

	// Read line after large line
	select {
	case line := <-tail.Lines:
		assert.NoError(t, line.Err)
		assert.Equal(t, "after large line", line.Text)
		tail.ReleaseLine(line)
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for line after large")
	}

	// Read nearly 1MB line - should work with already upgraded buffer
	select {
	case line := <-tail.Lines:
		assert.NoError(t, line.Err)
		assert.Equal(t, nearlyOneMBLine, line.Text)
		assert.Equal(t, constants.DefaultMaxEventSize-1024, len(line.Text), "Line should be nearly 1MB")
		tail.ReleaseLine(line)
	case <-time.After(2 * time.Second): // Longer timeout for very large line
		t.Fatal("Timeout waiting for nearly 1MB line")
	}

	// Read final line
	select {
	case line := <-tail.Lines:
		assert.NoError(t, line.Err)
		assert.Equal(t, "after nearly 1MB line", line.Text)
		tail.ReleaseLine(line)
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for final line")
	}

	// Verify buffer remains large throughout
	assert.Equal(t, constants.DefaultMaxEventSize, tail.reader.Size(), "Should maintain 1MB buffer")
	assert.True(t, tail.useLargeBuffer, "Should continue using large buffer")
}

// TestDynamicBufferPersistentUpgrade tests that buffer upgrade persists across file reopens
func TestDynamicBufferPersistentUpgrade(t *testing.T) {
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "persistent_test.log")

	// Create file with large line to trigger upgrade
	largeLine := strings.Repeat("c", 300*1024) // 300KB line
	content := largeLine + "\n"
	err := os.WriteFile(filename, []byte(content), 0600)
	require.NoError(t, err)

	tail, err := TailFile(filename, Config{
		Follow:      true,
		ReOpen:      true,
		MustExist:   true,
		MaxLineSize: constants.DefaultMaxEventSize, // 1MB max
	})
	require.NoError(t, err)
	defer tail.Stop()

	// Read large line to trigger upgrade
	select {
	case line := <-tail.Lines:
		assert.NoError(t, line.Err)
		assert.Equal(t, largeLine, line.Text)
		tail.ReleaseLine(line)
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for large line")
	}

	// Verify buffer was upgraded
	assert.True(t, tail.useLargeBuffer, "Should be using large buffer after upgrade")

	// Force a reopen by simulating file recreation
	err = tail.Reopen(false)
	require.NoError(t, err)

	// Verify buffer upgrade persisted across reopen
	assert.Equal(t, constants.DefaultMaxEventSize, tail.reader.Size(), "Should use 1MB buffer after reopen")
	assert.True(t, tail.useLargeBuffer, "Should maintain large buffer flag after reopen")
}

// TestDynamicBufferMaxLineSizeLimit tests behavior when line exceeds MaxLineSize
func TestDynamicBufferMaxLineSizeLimit(t *testing.T) {
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "max_size_test.log")

	maxLineSize := 512 * 1024 // 512KB max
	// Create line larger than MaxLineSize
	hugeLine := strings.Repeat("d", maxLineSize+1024) // 513KB line
	content := hugeLine + "\n"
	err := os.WriteFile(filename, []byte(content), 0600)
	require.NoError(t, err)

	tail, err := TailFile(filename, Config{
		Follow:      false,
		MustExist:   true,
		MaxLineSize: maxLineSize,
	})
	require.NoError(t, err)
	defer tail.Stop()

	// Read the line - should be truncated at buffer boundary
	select {
	case line := <-tail.Lines:
		assert.NoError(t, line.Err)
		// Line should be truncated to buffer size
		assert.Equal(t, maxLineSize, len(line.Text))
		assert.Equal(t, strings.Repeat("d", maxLineSize), line.Text)
		tail.ReleaseLine(line)
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for truncated line")
	}

	// Verify buffer was upgraded to MaxLineSize
	assert.Equal(t, maxLineSize, tail.reader.Size(), "Should upgrade to MaxLineSize buffer")
	assert.True(t, tail.useLargeBuffer, "Should be using large buffer")
}

// TestDynamicBufferMultipleUpgrades tests that buffer doesn't upgrade multiple times
func TestDynamicBufferMultipleUpgrades(t *testing.T) {
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "multiple_upgrades.log")

	// Create multiple large lines
	largeLine1 := strings.Repeat("e", 300*1024) // 300KB
	largeLine2 := strings.Repeat("f", 400*1024) // 400KB
	content := largeLine1 + "\n" + largeLine2 + "\n"
	err := os.WriteFile(filename, []byte(content), 0600)
	require.NoError(t, err)

	tail, err := TailFile(filename, Config{
		Follow:      false,
		MustExist:   true,
		MaxLineSize: constants.DefaultMaxEventSize, // 1MB max
	})
	require.NoError(t, err)
	defer tail.Stop()

	// Read first large line
	select {
	case line := <-tail.Lines:
		assert.NoError(t, line.Err)
		assert.Equal(t, largeLine1, line.Text)
		tail.ReleaseLine(line)
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for first large line")
	}

	// Verify buffer was upgraded
	assert.Equal(t, constants.DefaultMaxEventSize, tail.reader.Size(), "Should upgrade to 1MB after first large line")
	assert.True(t, tail.useLargeBuffer, "Should be using large buffer")

	// Read second large line
	select {
	case line := <-tail.Lines:
		assert.NoError(t, line.Err)
		assert.Equal(t, largeLine2, line.Text)
		tail.ReleaseLine(line)
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for second large line")
	}

	// Verify buffer size didn't change (no double upgrade)
	assert.Equal(t, constants.DefaultMaxEventSize, tail.reader.Size(), "Should maintain 1MB buffer")
	assert.True(t, tail.useLargeBuffer, "Should continue using large buffer")
}
