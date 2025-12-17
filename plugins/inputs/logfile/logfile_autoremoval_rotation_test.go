// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logfile

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/logs"
)

// TestAutoRemovalWithLogRotation verifies that auto_removal correctly handles
// log rotation by deleting the rotated file, not the new file.
//
// Expected behavior:
// 1. CW is tailing app.log
// 2. Log rotator renames app.log -> app.log.1 and creates new app.log
// 3. CW detects file deletion, continues reading old inode via open FD
// 4. CW reaches EOF and calls cleanUp() which should remove the rotated file
// 5. The new app.log should remain intact
//
// Current bug (test will fail):
// cleanUp() removes "app.log" by filename, which deletes the NEW file instead
// of the rotated one, because it stores the filename string not the inode.
func TestAutoRemovalWithLogRotation(t *testing.T) {
	multilineWaitPeriod = 10 * time.Millisecond

	// Create initial log file
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "app.log")

	// Write initial content
	err := os.WriteFile(logPath, []byte("line1\nline2\n"), 0644)
	require.NoError(t, err)

	// Setup LogFile with auto_removal enabled
	tt := NewLogFile()
	tt.Log = TestLogger{t}
	tt.FileConfig = []FileConfig{{
		FilePath:      logPath,
		FromBeginning: true,
		AutoRemoval:   true,
	}}
	require.NoError(t, tt.FileConfig[0].init())
	tt.started = true

	// Start tailing
	lsrcs := tt.FindLogSrc()
	require.Len(t, lsrcs, 1)

	evts := make(chan logs.LogEvent, 10)
	lsrcs[0].SetOutput(func(e logs.LogEvent) {
		if e != nil {
			evts <- e
		}
	})

	// Read initial lines
	e1 := <-evts
	assert.Equal(t, "line1", e1.Message())
	e2 := <-evts
	assert.Equal(t, "line2", e2.Message())

	// Simulate log rotation (like lumberjack does)
	rotatedPath := logPath + ".1"
	err = os.Rename(logPath, rotatedPath)
	require.NoError(t, err)

	// Create new log file with fresh content
	err = os.WriteFile(logPath, []byte("new_line1\n"), 0644)
	require.NoError(t, err)

	// Give time for file watcher to detect deletion and for cleanUp() to execute
	time.Sleep(500 * time.Millisecond)

	// EXPECTED BEHAVIOR: The NEW app.log should still exist
	_, err = os.Stat(logPath)
	assert.NoError(t, err,
		"New app.log should still exist after rotation")

	// EXPECTED BEHAVIOR: The rotated file should be deleted by auto_removal
	_, err = os.Stat(rotatedPath)
	assert.True(t, os.IsNotExist(err),
		"Rotated file app.log.1 should be deleted by auto_removal, not the fresh log")
}
