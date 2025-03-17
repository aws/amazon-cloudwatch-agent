// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux

package nvme

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetAllDevices(t *testing.T) {
	tests := []struct {
		name           string
		mockDirEntries []os.DirEntry
		mockError      error
		expected       []NvmeDeviceFileAttributes
		expectedError  error
	}{
		{
			name: "successful read with multiple devices",
			mockDirEntries: []os.DirEntry{
				mockDirEntry{name: "nvme0n1"},
				mockDirEntry{name: "nvme1n1"},
				mockDirEntry{name: "other-device"}, // Should be ignored
			},
			expected: []NvmeDeviceFileAttributes{
				{controller: 0, namespace: 1, partition: -1},
				{controller: 1, namespace: 1, partition: -1},
			},
		},
		{
			name:          "directory read error",
			mockError:     errors.New("read error"),
			expectedError: errors.New("read error"),
		},
		{
			name: "invalid device name format",
			mockDirEntries: []os.DirEntry{
				mockDirEntry{name: "nvmeinvalid"},
				mockDirEntry{name: "nvme0n1"},
			},
			expected: []NvmeDeviceFileAttributes{
				{controller: 0, namespace: 1, partition: -1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {
				osReadDir = os.ReadDir
			})

			osReadDir = func(path string) ([]os.DirEntry, error) {
				if tt.mockError != nil {
					return nil, tt.mockError
				}
				return tt.mockDirEntries, nil
			}

			util := &NvmeUtil{}
			devices, err := util.GetAllDevices()

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError.Error(), err.Error())
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, devices)
		})
	}
}

func TestGetDeviceSerial(t *testing.T) {
	tests := []struct {
		name          string
		device        NvmeDeviceFileAttributes
		mockData      string
		mockError     error
		expected      string
		expectedError error
	}{
		{
			name:     "successful read",
			device:   NvmeDeviceFileAttributes{controller: 0, namespace: 1, partition: -1},
			mockData: "vol0123456789\n",
			expected: "vol0123456789",
		},
		{
			name:          "read error",
			device:        NvmeDeviceFileAttributes{controller: 0, namespace: 1, partition: -1},
			mockError:     errors.New("read error"),
			expectedError: errors.New("read error"),
		},
		{
			name:     "padded serial number",
			device:   NvmeDeviceFileAttributes{controller: 0, namespace: 1, partition: -1},
			mockData: "  vol0123456789  \n",
			expected: "vol0123456789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {
				osReadFile = os.ReadFile
			})

			osReadFile = func(path string) ([]byte, error) {
				if tt.mockError != nil {
					return nil, tt.mockError
				}
				return []byte(tt.mockData), nil
			}

			util := &NvmeUtil{}
			serial, err := util.GetDeviceSerial(&tt.device)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError.Error(), err.Error())
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, serial)
		})
	}
}

func TestGetDeviceModel(t *testing.T) {
	tests := []struct {
		name          string
		device        NvmeDeviceFileAttributes
		mockData      string
		mockError     error
		expected      string
		expectedError error
	}{
		{
			name:     "successful read",
			device:   NvmeDeviceFileAttributes{controller: 0, namespace: 1, partition: -1},
			mockData: "Amazon Elastic Block Store\n",
			expected: "Amazon Elastic Block Store",
		},
		{
			name:          "read error",
			device:        NvmeDeviceFileAttributes{controller: 0, namespace: 1, partition: -1},
			mockError:     errors.New("read error"),
			expectedError: errors.New("read error"),
		},
		{
			name:     "padded model name",
			device:   NvmeDeviceFileAttributes{controller: 0, namespace: 1, partition: -1},
			mockData: "  Amazon Elastic Block Store  \n",
			expected: "Amazon Elastic Block Store",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {
				osReadFile = os.ReadFile
			})

			osReadFile = func(path string) ([]byte, error) {
				if tt.mockError != nil {
					return nil, tt.mockError
				}
				return []byte(tt.mockData), nil
			}

			util := &NvmeUtil{}
			model, err := util.GetDeviceModel(&tt.device)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError.Error(), err.Error())
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, model)
		})
	}
}

func TestIsEbsDevice(t *testing.T) {
	tests := []struct {
		name          string
		device        NvmeDeviceFileAttributes
		mockData      string
		mockError     error
		expected      bool
		expectedError error
	}{
		{
			name:     "is EBS device",
			device:   NvmeDeviceFileAttributes{controller: 0, namespace: 1, partition: -1},
			mockData: "Amazon Elastic Block Store\n",
			expected: true,
		},
		{
			name:     "not EBS device",
			device:   NvmeDeviceFileAttributes{controller: 0, namespace: 1, partition: -1},
			mockData: "Other Storage Device\n",
			expected: false,
		},
		{
			name:          "read error",
			device:        NvmeDeviceFileAttributes{controller: 0, namespace: 1, partition: -1},
			mockError:     errors.New("read error"),
			expectedError: errors.New("read error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {
				osReadFile = os.ReadFile
			})

			osReadFile = func(path string) ([]byte, error) {
				if tt.mockError != nil {
					return nil, tt.mockError
				}
				return []byte(tt.mockData), nil
			}

			util := &NvmeUtil{}
			isEbs, err := util.IsEbsDevice(&tt.device)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError.Error(), err.Error())
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, isEbs)
		})
	}
}

func TestCleanupString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "string with newline",
			input:    "test string\n",
			expected: "test string",
		},
		{
			name:     "string with spaces",
			input:    "  test string  ",
			expected: "test string",
		},
		{
			name:     "string with spaces and newline",
			input:    "  test string  \n",
			expected: "test string",
		},
		{
			name:     "clean string",
			input:    "test string",
			expected: "test string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanupString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Mock DirEntry implementation
type mockDirEntry struct {
	name string
}

func (m mockDirEntry) Name() string {
	return m.name
}

func (m mockDirEntry) IsDir() bool {
	return false
}

func (m mockDirEntry) Type() os.FileMode {
	return 0
}

func (m mockDirEntry) Info() (os.FileInfo, error) {
	return nil, nil
}
