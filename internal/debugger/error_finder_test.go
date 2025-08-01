// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package debugger

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func createLogFile(t *testing.T, tempDir, fileName, content string) string {
	filePath := filepath.Join(tempDir, fileName)
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file %s: %v", fileName, err)
	}
	return filePath
}

func TestGetLogEntriesSinceLastStartup(t *testing.T) {
	tempDir := t.TempDir()

	logContentWithStartup := `2025-01-01T00:00:00Z I! Starting AmazonCloudWatchAgent version 1.0.0 with log file
							  2025-01-01T00:01:00Z I! Normal log entry
							  2025-01-01T00:02:00Z E! Error log entry 1
							  2025-01-01T00:03:00Z I! Another normal entry
							  2025-01-01T00:04:00Z E! Error log entry 2
							  2025-01-01T00:05:00Z I! Starting AmazonCloudWatchAgent version 1.0.0 with log file
							  2025-01-01T00:06:00Z I! Normal log after restart
							  2025-01-01T00:07:00Z E! Error after restart
							  2025-01-01T00:08:00Z I! Final normal entry`

	logFilePath := createLogFile(t, tempDir, "test.log", logContentWithStartup)

	entries, err := getLogEntries(logFilePath)
	assert.NoError(t, err, "Should not return error for valid log file")
	assert.Equal(t, 4, len(entries), "Should find 4 total entries since last startup (including startup entry)")
	assert.Contains(t, entries[1], "Normal log after restart", "First entry should be normal log after restart")
	assert.Contains(t, entries[2], "Error after restart", "Second entry should be error after restart")
	assert.Contains(t, entries[3], "Final normal entry", "Third entry should be final normal entry")
}

func TestGetLogEntriesWithoutStartupInLog(t *testing.T) {
	tempDir := t.TempDir()

	logContentWithoutStartup := `2025-01-01T00:01:00Z I! Normal log entry
								 2025-01-01T00:02:00Z E! Error log entry 1
								 2025-01-01T00:03:00Z I! Normal log entry
								 2025-01-01T00:04:00Z E! Error log entry 2
								 2025-01-01T00:06:00Z I! Normal log entry
								 2025-01-01T00:08:00Z I! Final normal entry`

	noStartupFilePath := createLogFile(t, tempDir, "no-startup.log", logContentWithoutStartup)

	entries, err := getLogEntries(noStartupFilePath)
	assert.NoError(t, err, "Should not return error for valid log file")
	assert.Equal(t, 6, len(entries), "Should find all 6 entries when no startup pattern is found")
}

func TestGetLogEntriesEmptyFile(t *testing.T) {
	tempDir := t.TempDir()

	emptyFilePath := createLogFile(t, tempDir, "empty.log", "")

	entries, err := getLogEntries(emptyFilePath)
	assert.NoError(t, err, "Should not return error for empty file")
	assert.Equal(t, 0, len(entries), "Should find 0 entries in empty file")
}

func TestGetLogEntriesInvalidPath(t *testing.T) {
	entries, err := getLogEntries("/nonexistent/path")
	assert.Error(t, err, "Should return error for nonexistent file")
	assert.Nil(t, entries, "Should return nil entries on error")
}

func TestGetLogEntriesMultilineError(t *testing.T) {
	tempDir := t.TempDir()

	logContentWithMultilineError := `2025-01-01T00:00:00Z I! Starting AmazonCloudWatchAgent version 1.0.0 with log file
									 2025-01-01T00:01:00Z E! [outputs.cloudwatchlogs] Aws error received when sending logs to testsuccess.log/i-99999999: AccessDeniedException: User: arn:aws:sts::490874679724:assumed-role/testRole/i-99999999 is not authorized to perform: logs:PutLogEvents on resource: arn:aws:logs:us-east-2:490874679724:log-group:testlog.log:log-stream:i-99999999 
									 because no identity-based policy allows the logs:PutLogEvents action
									 2025-01-01T00:02:00Z I! Normal entry after multi-line error`

	multilineFilePath := createLogFile(t, tempDir, "multiline.log", logContentWithMultilineError)

	entries, err := getLogEntries(multilineFilePath)
	assert.NoError(t, err, "Should not return error for valid log file with multiline entries")
	assert.Equal(t, 3, len(entries), "Should find all 3 entries")

	errorEntry := entries[1]
	assert.Contains(t, errorEntry, "AccessDeniedException", "Should contain the error type")
	assert.Contains(t, errorEntry, "logs:PutLogEvents", "Should contain the action being denied")
	assert.Contains(t, errorEntry, "because no identity-based policy allows", "Should contain the continuation line")

	filteredErrors := filterErrorEntries(entries)
	assert.Equal(t, 1, len(filteredErrors), "Should find 1 error when filtering multiline entries")
	assert.Equal(t, errorEntry, filteredErrors[0], "Filtered error should match the original multiline error")
}

func TestFilterErrorEntries(t *testing.T) {
	testCases := []struct {
		name     string
		entries  []string
		expected int
	}{
		{
			name: "Mixed entries",
			entries: []string{
				"2025-01-01T00:01:00Z I! Normal log entry",
				"2025-01-01T00:02:00Z E! Error log entry 1",
				"2025-01-01T00:03:00Z I! Another normal entry",
				"2025-01-01T00:04:00Z E! Error log entry 2",
			},
			expected: 2,
		},
		{
			name: "Only error entries",
			entries: []string{
				"2025-01-01T00:02:00Z E! Error log entry 1",
				"2025-01-01T00:04:00Z E! Error log entry 2",
			},
			expected: 2,
		},
		{
			name: "No error entries",
			entries: []string{
				"2025-01-01T00:01:00Z I! Normal log entry",
				"2025-01-01T00:03:00Z I! Another normal entry",
			},
			expected: 0,
		},
		{
			name:     "Empty input",
			entries:  []string{},
			expected: 0,
		},
		{
			name: "Malformed entries",
			entries: []string{
				"Not a valid log entry",
				"2025-01-01T00:02:00Z E! Valid error entry",
				"Also not valid",
			},
			expected: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := filterErrorEntries(tc.entries)
			assert.Equal(t, tc.expected, len(result), "Number of filtered error entries should match expected")

			for _, entry := range result {
				assert.Contains(t, entry, "E!", "Filtered entry should contain error marker")
			}
		})
	}
}

func TestDeduplicateSuggestions(t *testing.T) {
	testCases := []struct {
		name        string
		suggestions []DiagnosticSuggestion
		expected    int
	}{
		{
			name: "No duplicates",
			suggestions: []DiagnosticSuggestion{
				{Fix: "Error 1"},
				{Fix: "Error 2"},
				{Fix: "Error 3"},
			},
			expected: 3,
		},
		{
			name: "With duplicates",
			suggestions: []DiagnosticSuggestion{
				{Fix: "Error 1"},
				{Fix: "Error 1"},
				{Fix: "Error 2"},
				{Fix: "Error 2"},
				{Fix: "Error 3"},
			},
			expected: 3,
		},
		{
			name:        "Empty input",
			suggestions: []DiagnosticSuggestion{},
			expected:    0,
		},
		{
			name: "All duplicates",
			suggestions: []DiagnosticSuggestion{
				{Fix: "Error 1"},
				{Fix: "Error 1"},
				{Fix: "Error 1"},
			},
			expected: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := deduplicateSuggestions(tc.suggestions)
			assert.Equal(t, tc.expected, len(result), "Number of deduplicated suggestions should match expected")

			// Verify no duplicates in result
			seen := make(map[string]bool)
			for _, suggestion := range result {
				assert.False(t, seen[suggestion.Fix], "Should not have duplicates after deduplication")
				seen[suggestion.Fix] = true
			}
		})
	}
}
