// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package debugger

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsConfigFilesPresentAndReadable(t *testing.T) {
	tmpDir := t.TempDir()

	status := checkFileContentStatus("/non/existent/file.json")
	assert.Equal(t, StatusMissing, status, "checkFileContentStatus() for non-existent file returned unexpected status")

	validJSONPath := filepath.Join(tmpDir, "valid.json")
	require.NoError(t, os.WriteFile(validJSONPath, []byte("{\"key\": \"value\"}"), 0644))

	status = checkFileContentStatus(validJSONPath)
	assert.Equal(t, StatusPresent, status, "checkFileContentStatus() for valid JSON file returned unexpected status")

	invalidJSONPath := filepath.Join(tmpDir, "invalid.json")
	require.NoError(t, os.WriteFile(invalidJSONPath, []byte("{\"key\": value}"), 0644))

	status = checkFileContentStatus(invalidJSONPath)
	assert.Equal(t, StatusInvalidJSONFormat, status, "checkFileContentStatus() for invalid JSON file returned unexpected status")
}

func TestCheckDirectoryStatus(t *testing.T) {
	tmpDir := t.TempDir()

	status := checkDirectoryStatus("/non/existent/dir.d")
	assert.Equal(t, StatusMissing, status, "checkDirectoryStatus() for non-existent directory returned unexpected status")

	status = checkDirectoryStatus(tmpDir)
	assert.Equal(t, StatusNoFile, status, "checkDirectoryStatus() for empty directory returned unexpected status")

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "file1.json"), []byte{}, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "file2.json"), []byte{}, 0644))

	status = checkDirectoryStatus(tmpDir)
	assert.Equal(t, StatusMultipleFiles, status, "checkDirectoryStatus() for directory with multiple files returned unexpected status")
}

func TestCheckFileStatus(t *testing.T) {
	tmpDir := t.TempDir()

	dirPath := filepath.Join(tmpDir, "config.d")
	require.NoError(t, os.Mkdir(dirPath, 0755))

	status := checkFileStatus(dirPath)
	assert.Equal(t, StatusNoFile, status, "checkFileStatus() for directory path returned unexpected status")

	filePath := filepath.Join(tmpDir, "config.json")
	require.NoError(t, os.WriteFile(filePath, []byte("{\"key\": \"value\"}"), 0644))

	status = checkFileStatus(filePath)
	assert.Equal(t, StatusPresent, status, "checkFileStatus() for file path returned unexpected status")
}

func TestGetConfigFiles(t *testing.T) {
	files := getConfigFiles()

	assert.NotEmpty(t, files, "getConfigFiles() returned empty slice")

	for i, file := range files {
		assert.NotEmpty(t, file.Path, "File %d has empty Path", i)
		assert.NotEmpty(t, file.Description, "File %d has empty Description", i)
	}
}

func TestPrintConfigFilesCompact(t *testing.T) {
	var buf bytes.Buffer
	configFiles := getConfigFiles()
	printConfigFilesCompact(&buf, configFiles)
	output := buf.String()

	assert.Contains(t, output, "amazon-cloudwatch-agent.toml:", "Output should contain TOML config file")
	assert.Contains(t, output, "amazon-cloudwatch-agent.d:", "Output should contain JSON config directory")
	assert.Contains(t, output, "amazon-cloudwatch-agent.log:", "Output should contain log file")

	assert.NotContains(t, output, "┌", "Compact format should not contain table borders")
	assert.NotContains(t, output, "│", "Compact format should not contain table borders")
	assert.NotContains(t, output, "File", "Compact format should not contain column headers")
}

func TestPrintConfigFilesTable(t *testing.T) {
	var buf bytes.Buffer
	configFiles := getConfigFiles()
	printConfigFilesTable(&buf, configFiles)
	output := buf.String()

	assert.Contains(t, output, "┌", "Table format should contain table borders")
	assert.Contains(t, output, "│", "Table format should contain vertical borders")
	assert.Contains(t, output, "File", "Table format should contain File column header")
	assert.Contains(t, output, "Status", "Table format should contain Status column header")
	assert.Contains(t, output, "Description", "Table format should contain Description column header")

	assert.Contains(t, output, "amazon-cloudwatch-agent.toml", "Output should contain TOML config file")
	assert.Contains(t, output, "amazon-cloudwatch-agent.d", "Output should contain JSON config directory")
	assert.Contains(t, output, "amazon-cloudwatch-agent.log", "Output should contain log file")
}
