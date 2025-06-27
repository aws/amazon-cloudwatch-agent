// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package debugger

import (
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

func TestCheckLogs(t *testing.T) {
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
			result := CheckLogs(tt.config)

			assert.Equal(t, tt.expectedSuccess, result.Success, "CheckLogs() returned unexpected success status")
			assert.Len(t, result.Files, tt.expectedFiles, "CheckLogs() returned unexpected number of files")
			assert.NotEmpty(t, result.Message, "CheckLogs() should return a non-empty message")
		})
	}
}
