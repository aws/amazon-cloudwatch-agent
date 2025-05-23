// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package state

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilePath(t *testing.T) {
	testDir := filepath.Join("tmp", "state")
	testCases := map[string]struct {
		dir  string
		name string
		want string
	}{
		"WithoutDirectory": {
			dir:  "",
			name: "test",
			want: "",
		},
		"WithFilePath": {
			dir:  testDir,
			name: "/var/log/file.log",
			want: filepath.Join(testDir, "_var_log_file.log"),
		},
		"WithSpaces": {
			dir:  testDir,
			name: "replace spaces with underscores",
			want: filepath.Join(testDir, "replace_spaces_with_underscores"),
		},
		"WithColons": {
			dir:  testDir,
			name: "replace:colons:with:underscores",
			want: filepath.Join(testDir, "replace_colons_with_underscores"),
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			got := FilePath(testCase.dir, testCase.name)
			assert.Equal(t, testCase.want, got)
		})
	}
}

func TestManagerConfig(t *testing.T) {
	testDir := filepath.Join("tmp", "state")
	cfg := ManagerConfig{
		StateFileDir: testDir,
		Name:         "no_prefix.log",
	}
	assert.Equal(t, filepath.Join(testDir, "no_prefix.log"), cfg.StateFilePath())
	cfg = ManagerConfig{
		StateFileDir:    testDir,
		StateFilePrefix: "dormant volca",
		Name:            "no_prefix.log",
	}
	assert.Equal(t, filepath.Join(testDir, "dormant_volcano_prefix.log"), cfg.StateFilePath())
}
