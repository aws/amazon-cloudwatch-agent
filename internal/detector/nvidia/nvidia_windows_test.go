//go:build windows

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
	windowsregistry "golang.org/x/sys/windows/registry"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
)

// mockRegistry implements Registry for testing
type mockRegistry struct {
	hasNvidiaDevice bool
}

func (mr *mockRegistry) OpenKey(k windowsregistry.Key, path string, access uint32) (registryKey, error) {
	return &mockRegistryKey{hasNvidiaDevice: mr.hasNvidiaDevice}, nil
}

type mockRegistryKey struct {
	hasNvidiaDevice bool
}

func (mrk *mockRegistryKey) Close() error {
	return nil
}

func (mrk *mockRegistryKey) ReadSubKeyNames(n int) ([]string, error) {
	if mrk.hasNvidiaDevice {
		return []string{"VEN_10DE&DEV_1234"}, nil
	}
	return []string{"VEN_8086&DEV_5678"}, nil
}

func createTestFiles(t *testing.T, tmpDir string, relativePaths []string) {
	t.Helper()
	for _, file := range relativePaths {
		err := os.WriteFile(filepath.Join(tmpDir, file), []byte(""), 0600)
		require.NoError(t, err)
	}
}

func TestWindowsChecker_HasNvidiaDevice(t *testing.T) {
	testCases := map[string]struct {
		hasDevice bool
		expected  bool
	}{
		"WithDevice":    {hasDevice: true, expected: true},
		"WithoutDevice": {hasDevice: false, expected: false},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			mr := &mockRegistry{hasNvidiaDevice: tc.hasDevice}
			c := newChecker().(*checker)
			c.registry = mr
			result := c.hasNvidiaDevice()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestWindowsChecker_HasDriverFiles(t *testing.T) {
	testCases := map[string]struct {
		setupFiles  []string
		driverPaths []string
		expected    bool
	}{
		"WithDriver": {
			setupFiles:  []string{"nvidia-smi.exe"},
			driverPaths: []string{},
			expected:    true,
		},
		"WithoutDriver": {
			setupFiles:  []string{},
			driverPaths: []string{},
			expected:    false,
		},
		"CustomPath": {
			setupFiles:  []string{"custom-nvidia-smi.exe"},
			driverPaths: []string{"custom-nvidia-smi.exe"},
			expected:    true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tempDir := t.TempDir()
			createTestFiles(t, tempDir, tc.setupFiles)

			d := NewDetector(slog.Default()).(*nvidiaDetector)
			c := d.checker.(*checker)

			if len(tc.driverPaths) > 0 {
				var driverPaths []string
				for _, path := range tc.driverPaths {
					driverPaths = append(driverPaths, filepath.Join(tempDir, path))
				}
				c.driverPaths = driverPaths
			} else {
				c.driverPaths = []string{
					filepath.Join(tempDir, "nvidia-smi.exe"),
					filepath.Join(tempDir, "nonexistent.exe"),
				}
			}

			result := c.hasDriverFiles()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestWindowsDetector_Detect(t *testing.T) {
	testCases := map[string]struct {
		hasDevice   bool
		driverFiles []string
		want        *detector.Metadata
		wantErr     error
	}{
		"NoDevice": {
			hasDevice:   false,
			driverFiles: []string{},
			wantErr:     detector.ErrIncompatibleDetector,
		},
		"DeviceWithDriver": {
			hasDevice:   true,
			driverFiles: []string{"nvidia-smi.exe"},
			want: &detector.Metadata{
				Categories: []detector.Category{detector.CategoryNvidiaGPU},
				Status:     detector.StatusReady,
			},
		},
		"DeviceWithoutDriver": {
			hasDevice:   true,
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
			createTestFiles(t, tempDir, tc.driverFiles)

			mr := &mockRegistry{hasNvidiaDevice: tc.hasDevice}
			d := NewDetector(slog.Default()).(*nvidiaDetector)
			c := d.checker.(*checker)
			c.registry = mr

			var driverPaths []string
			for _, file := range tc.driverFiles {
				driverPaths = append(driverPaths, filepath.Join(tempDir, file))
			}
			if len(driverPaths) == 0 {
				driverPaths = []string{filepath.Join(tempDir, "nonexistent.exe")}
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
