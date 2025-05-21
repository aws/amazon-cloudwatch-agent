// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/internal/logscommon"
	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"
)

func TestRetentionPolicyTTL(t *testing.T) {
	logger := testutil.NewNopLogger()

	t.Run("NewRetentionPolicyTTL", func(t *testing.T) {
		tempDir := t.TempDir()
		ttl := NewRetentionPolicyTTL(logger, tempDir)
		defer ttl.Done()
		defer cleanupTTLFile(tempDir)

		assert.NotNil(t, ttl)
		assert.Equal(t, filepath.Join(tempDir, logscommon.RetentionPolicyTTLFileName), ttl.stateFilePath)
		assert.NotNil(t, ttl.oldTimestamps)
		assert.NotNil(t, ttl.newTimestamps)
		assert.NotNil(t, ttl.ch)
		assert.NotNil(t, ttl.done)
	})

	t.Run("IsExpired_NoExistingTimestamp", func(t *testing.T) {
		tempDir := t.TempDir()
		ttl := NewRetentionPolicyTTL(logger, tempDir)
		defer cleanupTTLFile(tempDir)
		defer ttl.Done()

		// When no timestamp exists for a group, it should be considered expired
		assert.True(t, ttl.IsExpired("TestGroup"))
	})

	t.Run("IsExpired_WithExpiredTimestamp", func(t *testing.T) {
		tempDir := t.TempDir()
		ttl := NewRetentionPolicyTTL(logger, tempDir)
		defer cleanupTTLFile(tempDir)
		defer ttl.Done()

		// Set an expired timestamp (more than ttlTime in the past)
		expiredTime := time.Now().Add(-10 * time.Minute)
		ttl.oldTimestamps["TestGroup"] = expiredTime

		assert.True(t, ttl.IsExpired("TestGroup"))
	})

	t.Run("IsExpired_WithValidTimestamp", func(t *testing.T) {
		tempDir := t.TempDir()
		ttl := NewRetentionPolicyTTL(logger, tempDir)
		defer cleanupTTLFile(tempDir)
		defer ttl.Done()

		// Set a valid timestamp (less than ttlTime in the past)
		validTime := time.Now().Add(-1 * time.Minute)
		ttl.oldTimestamps["TestGroup"] = validTime

		assert.False(t, ttl.IsExpired("TestGroup"))
	})

	t.Run("Update_SavesTimestamp", func(t *testing.T) {
		tempDir := t.TempDir()
		ttl := NewRetentionPolicyTTL(logger, tempDir)
		defer cleanupTTLFile(tempDir)
		defer ttl.Done()

		ttl.Update("TestGroup")

		time.Sleep(100 * time.Millisecond)

		ttl.mu.RLock()
		_, exists := ttl.newTimestamps["TestGroup"]
		ttl.mu.RUnlock()

		assert.True(t, exists)
	})

	t.Run("UpdateFromFile_CopiesOldTimestamp", func(t *testing.T) {
		tempDir := t.TempDir()
		ttl := NewRetentionPolicyTTL(logger, tempDir)
		defer cleanupTTLFile(tempDir)
		defer ttl.Done()

		// Set an old timestamp
		oldTime := time.Now().Add(-1 * time.Minute)
		ttl.oldTimestamps["TestGroup"] = oldTime

		// Persist old timestamp to new timestamp
		ttl.UpdateFromFile("TestGroup")

		time.Sleep(200 * time.Millisecond)

		ttl.mu.RLock()
		newTime, exists := ttl.newTimestamps["TestGroup"]
		ttl.mu.RUnlock()

		assert.True(t, exists)
		assert.Equal(t, oldTime.UnixMilli(), newTime.UnixMilli())
	})

	t.Run("saveTTLState_WritesFile", func(t *testing.T) {
		tempDir := t.TempDir()
		ttl := NewRetentionPolicyTTL(logger, tempDir)
		defer cleanupTTLFile(tempDir)
		defer ttl.Done()

		now := time.Now()
		ttl.mu.Lock()
		ttl.newTimestamps["group1"] = now
		ttl.newTimestamps["group2"] = now.Add(1 * time.Minute)
		ttl.mu.Unlock()

		ttl.saveTTLState()

		_, err := os.Stat(ttl.stateFilePath)
		assert.NoError(t, err)

		content, err := os.ReadFile(ttl.stateFilePath)
		assert.NoError(t, err)

		contentStr := string(content)
		assert.Contains(t, contentStr, "group1:")
		assert.Contains(t, contentStr, "group2:")
	})

	t.Run("loadTTLState_ReadsFile", func(t *testing.T) {
		tempDir := t.TempDir()
		stateFilePath := filepath.Join(tempDir, logscommon.RetentionPolicyTTLFileName)

		// Create a state file
		now := time.Now()
		nowMillis := now.UnixMilli()
		content := "group1:" + strconv.FormatInt(nowMillis, 10) + "\n" +
			"group2:" + strconv.FormatInt(nowMillis+60000, 10) + "\n"

		err := os.WriteFile(stateFilePath, []byte(content), 0644) // nolint:gosec
		assert.NoError(t, err)

		// Create a new TTL instance that will load the file
		ttl := NewRetentionPolicyTTL(logger, tempDir)
		defer cleanupTTLFile(tempDir)
		defer ttl.Done()

		time.Sleep(200 * time.Millisecond)

		assert.Len(t, ttl.oldTimestamps, 2)
		assert.Contains(t, ttl.oldTimestamps, "group1")
		assert.Contains(t, ttl.oldTimestamps, "group2")

		assert.Equal(t, nowMillis, ttl.oldTimestamps["group1"].UnixMilli())
		assert.Equal(t, nowMillis+60000, ttl.oldTimestamps["group2"].UnixMilli())
	})

	t.Run("loadTTLState_HandlesInvalidFile", func(t *testing.T) {
		tempDir := t.TempDir()
		stateFilePath := filepath.Join(tempDir, logscommon.RetentionPolicyTTLFileName)

		// Create an invalid state file
		content := "group1:invalid_timestamp\n" +
			"group2:123456789\n" +
			"\n" + // Empty line should be skipped
			"invalid_line_no_separator\n"

		err := os.WriteFile(stateFilePath, []byte(content), 0644) // nolint:gosec
		assert.NoError(t, err)

		ttl := NewRetentionPolicyTTL(logger, tempDir)
		defer cleanupTTLFile(tempDir)
		defer ttl.Done()

		assert.Len(t, ttl.oldTimestamps, 1)
		assert.Contains(t, ttl.oldTimestamps, "group2")
		assert.NotContains(t, ttl.oldTimestamps, "group1")
		assert.NotContains(t, ttl.oldTimestamps, "invalid_line_no_separator")
	})

	t.Run("Done_ClosesChannel", func(t *testing.T) {
		tempDir := t.TempDir()
		ttl := NewRetentionPolicyTTL(logger, tempDir)

		ttl.Update("TestGroup")
		time.Sleep(100 * time.Millisecond)
		ttl.Done()
		time.Sleep(100 * time.Millisecond)

		_, err := os.Stat(ttl.stateFilePath)
		assert.NoError(t, err)
	})

	t.Run("escapeLogGroup", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected string
		}{
			{"/aws/lambda/function", "_aws_lambda_function"},
			{"my log group", "my_log_group"},
			{"group:with:colons", "group_with_colons"},
			{"/path/with/slashes", "_path_with_slashes"},
			{"normal-group", "normal-group"},
		}

		for _, tc := range testCases {
			result := escapeLogGroup(tc.input)
			assert.Equal(t, tc.expected, result)
		}
	})
}

func cleanupTTLFile(dir string) {
	time.Sleep(100 * time.Millisecond)
	os.Remove(filepath.Join(dir, logscommon.RetentionPolicyTTLFileName))
}
