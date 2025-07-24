// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package debugger

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetCollectListFromConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      map[string]interface{}
		wantErr     bool
		expectedLen int
	}{
		{
			name:        "Empty config",
			config:      map[string]interface{}{},
			wantErr:     true,
			expectedLen: 0,
		},
		{
			name: "Missing logs_collected",
			config: map[string]interface{}{
				"logs": map[string]interface{}{},
			},
			wantErr:     true,
			expectedLen: 0,
		},
		{
			name: "Missing files",
			config: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{},
				},
			},
			wantErr:     true,
			expectedLen: 0,
		},
		{
			name: "Missing collect_list",
			config: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"files": map[string]interface{}{},
					},
				},
			},
			wantErr:     true,
			expectedLen: 0,
		},
		{
			name: "Valid config with empty collect_list",
			config: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"files": map[string]interface{}{
							"collect_list": []interface{}{},
						},
					},
				},
			},
			wantErr:     false,
			expectedLen: 0,
		},
		{
			name: "Valid config with collect_list",
			config: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"files": map[string]interface{}{
							"collect_list": []interface{}{
								map[string]interface{}{
									"file_path": "/var/log/test.log",
								},
							},
						},
					},
				},
			},
			wantErr:     false,
			expectedLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collectList, err := getCollectListFromConfig(tt.config)
			if tt.wantErr {
				assert.Error(t, err, "getCollectListFromConfig() should return an error")
			} else {
				assert.NoError(t, err, "getCollectListFromConfig() should not return an error")
				assert.Len(t, collectList, tt.expectedLen, "getCollectListFromConfig() returned unexpected number of items")
			}
		})
	}
}

func TestCheckConfiguredLogs(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "logcheck_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testLogFile := filepath.Join(tempDir, "test.log")
	if err := os.WriteFile(testLogFile, []byte("test log content"), 0644); err != nil {
		t.Fatalf("Failed to create test log file: %v", err)
	}

	tests := []struct {
		name            string
		config          map[string]interface{}
		expectedSuccess bool
		expectedFiles   int
	}{
		{
			name:            "Empty config",
			config:          map[string]interface{}{},
			expectedSuccess: false,
			expectedFiles:   0,
		},
		{
			name: "Config with invalid collect_list item",
			config: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"files": map[string]interface{}{
							"collect_list": []interface{}{
								"not a map",
							},
						},
					},
				},
			},
			expectedSuccess: true,
			expectedFiles:   0,
		},
		{
			name: "Config with item missing file_path",
			config: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"files": map[string]interface{}{
							"collect_list": []interface{}{
								map[string]interface{}{
									"not_file_path": "something",
								},
							},
						},
					},
				},
			},
			expectedSuccess: true,
			expectedFiles:   0,
		},
		{
			name: "Config with valid file path",
			config: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"files": map[string]interface{}{
							"collect_list": []interface{}{
								map[string]interface{}{
									"file_path": testLogFile,
								},
							},
						},
					},
				},
			},
			expectedSuccess: true,
			expectedFiles:   1,
		},
		{
			name: "Config with non-existent file path",
			config: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"files": map[string]interface{}{
							"collect_list": []interface{}{
								map[string]interface{}{
									"file_path": filepath.Join(tempDir, "nonexistent.log"),
								},
							},
						},
					},
				},
			},
			expectedSuccess: false,
			expectedFiles:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			result, err := CheckConfiguredLogsExistsAndReadable(&buf, tt.config, false)

			if tt.expectedSuccess {
				assert.NoError(t, err, "CheckConfiguredLogs() should not return an error")
			} else {
				if tt.expectedFiles == 0 {
					assert.Error(t, err, "CheckConfiguredLogs() should return an error")
				}
			}
			assert.Len(t, result, tt.expectedFiles, "CheckConfiguredLogs() returned unexpected number of files")
		})
	}
}

func TestExpandGlob(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "glob_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFiles := []string{"test1.log", "test2.log", "app.txt"}
	for _, file := range testFiles {
		if err := os.WriteFile(filepath.Join(tempDir, file), []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", file, err)
		}
	}

	tests := []struct {
		name     string
		pattern  string
		expected int
	}{
		{
			name:     "Literal path",
			pattern:  filepath.Join(tempDir, "test1.log"),
			expected: 1,
		},
		{
			name:     "Wildcard pattern",
			pattern:  filepath.Join(tempDir, "*.log"),
			expected: 2,
		},
		{
			name:     "No matches",
			pattern:  filepath.Join(tempDir, "*.xyz"),
			expected: 0,
		},
		{
			name:     "Recursive pattern",
			pattern:  filepath.Join(tempDir, "**/*.log"),
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandGlob(tt.pattern)
			assert.Len(t, result, tt.expected, "expandGlob() returned unexpected number of matches")
		})
	}
}

func TestExpandRecursiveGlob(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "recursive_glob_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	subDir := filepath.Join(tempDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	testFiles := map[string]string{
		"root.log":    tempDir,
		"app.txt":     tempDir,
		"nested.log":  subDir,
		"config.json": subDir,
	}

	for file, dir := range testFiles {
		if err := os.WriteFile(filepath.Join(dir, file), []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", file, err)
		}
	}

	tests := []struct {
		name     string
		pattern  string
		expected int
	}{
		{
			name:     "All log files recursively",
			pattern:  filepath.Join(tempDir, "**/*.log"),
			expected: 2,
		},
		{
			name:     "All files recursively",
			pattern:  filepath.Join(tempDir, "**"),
			expected: 4,
		},
		{
			name:     "No matches",
			pattern:  filepath.Join(tempDir, "**/*.xyz"),
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandRecursiveGlob(tt.pattern)
			assert.Len(t, result, tt.expected, "expandRecursiveGlob() returned unexpected number of matches")
		})
	}
}
