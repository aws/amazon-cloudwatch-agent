// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package debugger

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDisplayName(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "directory path",
			path:     "/path/to/config.d",
			expected: "amazon-cloudwatch-agent.d",
		},
		{
			name:     "regular file path",
			path:     "/path/to/config.json",
			expected: "config.json",
		},
		{
			name:     "nested file path",
			path:     "/very/long/path/to/file.toml",
			expected: "file.toml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getDisplayName(tt.path)
			assert.Equal(t, tt.expected, result, "getDisplayName() returned unexpected result")
		})
	}
}

func TestCheckFileContentStatus(t *testing.T) {
	// Non-existent file
	status := checkFileContentStatus("/non/existent/file.json")
	assert.Equal(t, StatusMissing, status, "checkFileContentStatus() for non-existent file returned unexpected status")

	// Valid JSON
	tmpFile, err := os.CreateTemp("", "test-*.json")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	_, err = tmpFile.WriteString("{\"key\": \"value\"}")
	require.NoError(t, err)
	tmpFile.Sync()

	status = checkFileContentStatus(tmpFile.Name())
	assert.Equal(t, StatusPresent, status, "checkFileContentStatus() for valid JSON file returned unexpected status")

	// Invalid JSON
	invalidJSONFile, err := os.CreateTemp("", "invalid-*.json")
	require.NoError(t, err)
	defer os.Remove(invalidJSONFile.Name())
	defer invalidJSONFile.Close()

	_, err = invalidJSONFile.WriteString("{\"key\": value}")
	require.NoError(t, err)
	invalidJSONFile.Sync()

	status = checkFileContentStatus(invalidJSONFile.Name())
	assert.Equal(t, StatusInvalidJSONFormat, status, "checkFileContentStatus() for invalid JSON file returned unexpected status")
}

func TestCheckDirectoryStatus(t *testing.T) {
	// Non-existent directory
	status := checkDirectoryStatus("/non/existent/dir.d")
	assert.Equal(t, StatusMissing, status, "checkDirectoryStatus() for non-existent directory returned unexpected status")

	// Empty directory
	tmpDir, err := os.MkdirTemp("", "test-dir")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	status = checkDirectoryStatus(tmpDir)
	assert.Equal(t, StatusNoFile, status, "checkDirectoryStatus() for empty directory returned unexpected status")

	// Multiple files in the directory
	file1, err := os.Create(filepath.Join(tmpDir, "file1.json"))
	require.NoError(t, err)
	defer file1.Close()

	file2, err := os.Create(filepath.Join(tmpDir, "file2.json"))
	require.NoError(t, err)
	defer file2.Close()

	status = checkDirectoryStatus(tmpDir)
	assert.Equal(t, StatusMultipleFiles, status, "checkDirectoryStatus() for directory with multiple files returned unexpected status")
}

func TestCheckFileStatus(t *testing.T) {
	// Directory path
	tmpDir, err := os.MkdirTemp("", "test-dir")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Add .d suffix to make it recognized as a directory
	dirPath := tmpDir + ".d"
	err = os.Rename(tmpDir, dirPath)
	if err != nil {
		// If the rename fails somehow just skip the test
		t.Skip("Could not rename directory with .d suffix")
	}

	status := checkFileStatus(dirPath)
	assert.Equal(t, StatusNoFile, status, "checkFileStatus() for directory path returned unexpected status")

	tmpFile, err := os.CreateTemp("", "test-*.json")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	_, err = tmpFile.WriteString("{\"key\": \"value\"}")
	require.NoError(t, err)
	tmpFile.Sync()

	status = checkFileStatus(tmpFile.Name())
	assert.Equal(t, StatusPresent, status, "checkFileStatus() for file path returned unexpected status")
}

func TestGetConfigFiles(t *testing.T) {
	files := getConfigFiles()

	assert.NotEmpty(t, files, "getConfigFiles() returned empty slice")

	// Check that all files have required fields
	for i, file := range files {
		assert.NotEmpty(t, file.Path, "File %d has empty Path", i)
		assert.NotEmpty(t, file.Description, "File %d has empty Description", i)
	}
}

// Smoke test
func TestCheckConfigFiles(t *testing.T) {
	CheckConfigFiles()
}
