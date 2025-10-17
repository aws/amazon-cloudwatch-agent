//go:build linux

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nvidia

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
)

func createTestFiles(t *testing.T, tmpDir string, relativePaths []string) {
	t.Helper()
	for _, file := range relativePaths {
		err := os.WriteFile(filepath.Join(tmpDir, file), []byte(""), 0600)
		require.NoError(t, err)
	}
}

func TestLinuxChecker_HasNvidiaDevice(t *testing.T) {
	testCases := map[string]struct {
		setupFiles []string
		expected   bool
	}{
		"WithDevice": {
			setupFiles: []string{"nvidia0", "nvidia1"},
			expected:   true,
		},
		"WithoutDevice": {
			setupFiles: []string{"other0", "random1"},
			expected:   false,
		},
		"EmptyDir": {
			setupFiles: []string{},
			expected:   false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tempDir := t.TempDir()
			createTestFiles(t, tempDir, tc.setupFiles)

			c := newChecker().(*checker)
			c.devPath = tempDir
			result := c.hasNvidiaDevice()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestLinuxChecker_HasDriverFiles(t *testing.T) {
	testCases := map[string]struct {
		setupFiles  []string
		driverPaths []string
		expected    bool
	}{
		"WithDriver": {
			setupFiles:  []string{"nvidia-smi"},
			driverPaths: []string{},
			expected:    true,
		},
		"WithoutDriver": {
			setupFiles:  []string{},
			driverPaths: []string{},
			expected:    false,
		},
		"CustomPath": {
			setupFiles:  []string{"custom-nvidia-smi"},
			driverPaths: []string{"custom-nvidia-smi"},
			expected:    true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tempDir := t.TempDir()
			createTestFiles(t, tempDir, tc.setupFiles)

			c := newChecker().(*checker)
			if len(tc.driverPaths) > 0 {
				var driverPaths []string
				for _, path := range tc.driverPaths {
					driverPaths = append(driverPaths, filepath.Join(tempDir, path))
				}
				c.driverPaths = driverPaths
			} else {
				c.driverPaths = []string{filepath.Join(tempDir, "nvidia-smi")}
			}

			result := c.hasDriverFiles()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestLinuxDetector_Detect(t *testing.T) {
	testCases := map[string]struct {
		deviceFiles []string
		driverFiles []string
		want        *detector.Metadata
		wantErr     error
	}{
		"NoDevice": {
			deviceFiles: []string{},
			driverFiles: []string{},
			wantErr:     detector.ErrIncompatibleDetector,
		},
		"DeviceWithDriver": {
			deviceFiles: []string{"nvidia0"},
			driverFiles: []string{"nvidia-smi"},
			want: &detector.Metadata{
				Categories: []detector.Category{detector.CategoryNvidiaGPU},
				Status:     detector.StatusReady,
			},
		},
		"DeviceWithoutDriver": {
			deviceFiles: []string{"nvidia0"},
			driverFiles: []string{},
			want: &detector.Metadata{
				Categories: []detector.Category{detector.CategoryNvidiaGPU},
				Status:     detector.StatusNeedsSetupNvidiaDriver,
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tempDir := t.TempDir()
			createTestFiles(t, tempDir, tc.deviceFiles)

			d := NewDetector(slog.Default()).(*nvidiaDetector)
			c := d.checker.(*checker)
			c.devPath = tempDir

			var driverPaths []string
			for _, file := range tc.driverFiles {
				driverPath := filepath.Join(tempDir, file)
				err := os.WriteFile(driverPath, []byte(""), 0600)
				require.NoError(t, err)
				driverPaths = append(driverPaths, driverPath)
			}
			if len(driverPaths) == 0 {
				driverPaths = []string{filepath.Join(tempDir, "nonexistent")}
			}
			c.driverPaths = driverPaths

			got, err := d.Detect()

			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}
