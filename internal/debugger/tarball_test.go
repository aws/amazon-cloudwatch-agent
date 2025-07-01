// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package debugger

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddStringToTarball(t *testing.T) {
	var buf bytes.Buffer
	tarWriter := tar.NewWriter(&buf)

	testContent := "test content"
	testPath := "test/path.txt"

	err := addStringToTarball(tarWriter, testContent, testPath)
	require.NoError(t, err, "Should not error when adding string to tarball")

	require.NoError(t, tarWriter.Close(), "Should close the tar writer without error")

	tarReader := tar.NewReader(&buf)
	header, err := tarReader.Next()
	require.NoError(t, err, "Should read tar header without error")

	assert.Equal(t, testPath, header.Name, "Tar entry should have the correct path")
	assert.Equal(t, int64(len(testContent)), header.Size, "Tar entry should have the correct size")

	content := make([]byte, header.Size)
	_, err = io.ReadFull(tarReader, content)
	require.NoError(t, err, "Should read tar content without error")
	assert.Equal(t, testContent, string(content), "Content should match what was added")
}

func TestFindTailContent(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-tail-*.txt")
	require.NoError(t, err, "Should create temp file without error")
	defer os.Remove(tmpFile.Name())

	testContent := "line1\nline2\nline3\nline4\nline5\n"
	_, err = tmpFile.WriteString(testContent)
	require.NoError(t, err, "Should write to temp file without error")
	require.NoError(t, tmpFile.Sync(), "Should sync file without error")

	testCases := []struct {
		name          string
		maxLines      int
		expectedLines string
	}{
		{"All lines", 10, "line1\nline2\nline3\nline4\nline5\n"},
		{"Last 3 lines", 3, "line4\nline5\n"},
		{"Last line", 1, ""},
		{"Zero lines", 0, "line1\nline2\nline3\nline4\nline5\n"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tmpFile.Seek(0, io.SeekStart)
			require.NoError(t, err, "Should seek to start without error")

			pos, content, err := findTailContent(tmpFile, tc.maxLines)
			require.NoError(t, err, "Should find tail content without error")

			assert.Equal(t, tc.expectedLines, string(content), "Should return correct tail content")

			fileInfo, err := tmpFile.Stat()
			require.NoError(t, err, "Should get file info without error")
			assert.GreaterOrEqual(t, pos, int64(0), "Position should be non-negative")
			assert.LessOrEqual(t, pos, fileInfo.Size(), "Position should not exceed file size")
		})
	}
}

func TestAddFileToTarball(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-file-*.txt")
	require.NoError(t, err, "Should create temp file without error")
	defer os.Remove(tmpFile.Name())

	testContent := "line1\nline2\nline3\nline4\nline5\n"
	_, err = tmpFile.WriteString(testContent)
	require.NoError(t, err, "Should write to temp file without error")
	require.NoError(t, tmpFile.Sync(), "Should sync file without error")

	testCases := []struct {
		name      string
		maxLines  int
		expectAll bool
	}{
		{"Full file", -1, true},
		{"Last 3 lines", 3, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			tarWriter := tar.NewWriter(&buf)

			var maxLength []int
			if tc.maxLines >= 0 {
				maxLength = []int{tc.maxLines}
			}

			err := addFileToTarball(tarWriter, tmpFile.Name(), "test.txt", maxLength...)
			require.NoError(t, err, "Should add file to tarball without error")
			require.NoError(t, tarWriter.Close(), "Should close tar writer without error")

			tarReader := tar.NewReader(&buf)
			header, err := tarReader.Next()
			require.NoError(t, err, "Should read tar header without error")

			assert.Equal(t, "test.txt", header.Name, "Tar entry should have the correct name")

			content := make([]byte, header.Size)
			_, err = io.ReadFull(tarReader, content)
			require.NoError(t, err, "Should read tar content without error")

			if tc.expectAll {
				assert.Equal(t, testContent, string(content), "Should contain all content")
			} else {
				lines := strings.Split(strings.TrimSpace(string(content)), "\n")
				assert.LessOrEqual(t, len(lines), tc.maxLines, "Should not exceed max lines")

				allLines := strings.Split(strings.TrimSpace(testContent), "\n")
				expectedLines := allLines[len(allLines)-len(lines):]
				for i := range lines {
					assert.Equal(t, expectedLines[i], lines[i], "Line content should match")
				}
			}
		})
	}
}

func TestAddDirectoryToTarball(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-dir-*")
	require.NoError(t, err, "Should create temp directory without error")
	defer os.RemoveAll(tmpDir)

	subDir := filepath.Join(tmpDir, "subdir")
	require.NoError(t, os.Mkdir(subDir, 0755), "Should create subdirectory without error")

	files := map[string]string{
		filepath.Join(tmpDir, "file1.txt"): "content1",
		filepath.Join(subDir, "file2.txt"): "content2",
		filepath.Join(subDir, "file3.txt"): "content3",
	}

	for path, content := range files {
		err := os.WriteFile(path, []byte(content), 0644)
		require.NoError(t, err, "Should write file without error")
	}

	var buf bytes.Buffer
	tarWriter := tar.NewWriter(&buf)

	err = addDirectoryToTarball(tarWriter, tmpDir, "test-dir")
	require.NoError(t, err, "Should add directory to tarball without error")
	require.NoError(t, tarWriter.Close(), "Should close tar writer without error")

	tarReader := tar.NewReader(&buf)

	// Expected entries (directory + files)
	expectedEntries := map[string]string{
		"test-dir/subdir/":          "",
		"test-dir/file1.txt":        "content1",
		"test-dir/subdir/file2.txt": "content2",
		"test-dir/subdir/file3.txt": "content3",
	}

	foundEntries := make(map[string]bool)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err, "Should read tar header without error")

		foundEntries[header.Name] = true

		if !strings.HasSuffix(header.Name, "/") {
			expectedContent, exists := expectedEntries[header.Name]
			require.True(t, exists, "Entry should be expected: %s", header.Name)

			content := make([]byte, header.Size)
			_, err = io.ReadFull(tarReader, content)
			require.NoError(t, err, "Should read tar content without error")
			assert.Equal(t, expectedContent, string(content), "File content should match")
		}
	}

	for entry := range expectedEntries {
		assert.True(t, foundEntries[entry], "Expected entry should be found: %s", entry)
	}
}

func TestPermissionDeniedRegex(t *testing.T) {
	testCases := []struct {
		name        string
		errorMsg    string
		expectMatch bool
	}{
		{"Standard permission denied", "permission denied", true},
		{"Capitalized permission denied", "Permission denied", true},
		{"All caps permission denied", "PERMISSION DENIED", true},
		{"Mixed case permission denied", "Permission Denied", true},
		{"Extra spaces", "permission   denied", true},
		{"Tab separated", "permission\tdenied", true},
		{"In sentence", "open file: permission denied", true},
		{"With path", "/opt/aws/file: permission denied", true},
		{"Different error", "file not found", false},
		{"Access denied", "access denied", false},
		{"Empty string", "", false},
		{"Partial match", "permission", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := errors.New(tc.errorMsg)
			matched, regexErr := regexp.MatchString(`(?i)permission\s+denied`, err.Error())
			require.NoError(t, regexErr, "Regex should compile without error")
			assert.Equal(t, tc.expectMatch, matched, "Regex match result should be as expected")
		})
	}
}
