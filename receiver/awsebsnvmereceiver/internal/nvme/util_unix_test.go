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
		expected       []DeviceFileAttributes
		expectedError  error
	}{
		{
			name: "successful read with multiple devices",
			mockDirEntries: []os.DirEntry{
				mockDirEntry{name: "nvme0n1", isDir: false},
				mockDirEntry{name: "nvme1n1", isDir: false},
				mockDirEntry{name: "other-device", isDir: false}, // Should be ignored
				mockDirEntry{name: "nvme2", isDir: true},         // Should be ignored because it's a directory
			},
			expected: []DeviceFileAttributes{
				{controller: 0, namespace: 1, partition: -1, deviceName: "nvme0n1"},
				{controller: 1, namespace: 1, partition: -1, deviceName: "nvme1n1"},
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
				mockDirEntry{name: "nvmeinvalid", isDir: false},
				mockDirEntry{name: "nvme0n1", isDir: false},
			},
			expected: []DeviceFileAttributes{
				{controller: 0, namespace: 1, partition: -1, deviceName: "nvme0n1"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {
				osReadDir = os.ReadDir
			})

			osReadDir = func(_ string) ([]os.DirEntry, error) {
				if tt.mockError != nil {
					return nil, tt.mockError
				}
				return tt.mockDirEntries, nil
			}

			util := &Util{}
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
		device        DeviceFileAttributes
		mockData      string
		mockError     error
		expected      string
		expectedError error
	}{
		{
			name:     "successful read",
			device:   DeviceFileAttributes{controller: 0, namespace: 1, partition: -1},
			mockData: "vol0123456789\n",
			expected: "vol0123456789",
		},
		{
			name:          "read error",
			device:        DeviceFileAttributes{controller: 0, namespace: 1, partition: -1},
			mockError:     errors.New("read error"),
			expectedError: errors.New("read error"),
		},
		{
			name:     "padded serial number",
			device:   DeviceFileAttributes{controller: 0, namespace: 1, partition: -1},
			mockData: "  vol0123456789  \n",
			expected: "vol0123456789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {
				osReadFile = os.ReadFile
			})

			osReadFile = func(_ string) ([]byte, error) {
				if tt.mockError != nil {
					return nil, tt.mockError
				}
				return []byte(tt.mockData), nil
			}

			util := &Util{}
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
		device        DeviceFileAttributes
		mockData      string
		mockError     error
		expected      string
		expectedError error
	}{
		{
			name:     "successful read",
			device:   DeviceFileAttributes{controller: 0, namespace: 1, partition: -1},
			mockData: "Amazon Elastic Block Store\n",
			expected: "Amazon Elastic Block Store",
		},
		{
			name:          "read error",
			device:        DeviceFileAttributes{controller: 0, namespace: 1, partition: -1},
			mockError:     errors.New("read error"),
			expectedError: errors.New("read error"),
		},
		{
			name:     "padded model name",
			device:   DeviceFileAttributes{controller: 0, namespace: 1, partition: -1},
			mockData: "  Amazon Elastic Block Store  \n",
			expected: "Amazon Elastic Block Store",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {
				osReadFile = os.ReadFile
			})

			osReadFile = func(_ string) ([]byte, error) {
				if tt.mockError != nil {
					return nil, tt.mockError
				}
				return []byte(tt.mockData), nil
			}

			util := &Util{}
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
		device        DeviceFileAttributes
		mockData      string
		mockError     error
		expected      bool
		expectedError error
	}{
		{
			name:     "is EBS device",
			device:   DeviceFileAttributes{controller: 0, namespace: 1, partition: -1},
			mockData: "Amazon Elastic Block Store\n",
			expected: true,
		},
		{
			name:     "not EBS device",
			device:   DeviceFileAttributes{controller: 0, namespace: 1, partition: -1},
			mockData: "Other Storage Device\n",
			expected: false,
		},
		{
			name:          "read error",
			device:        DeviceFileAttributes{controller: 0, namespace: 1, partition: -1},
			mockError:     errors.New("read error"),
			expectedError: errors.New("read error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {
				osReadFile = os.ReadFile
			})

			osReadFile = func(_ string) ([]byte, error) {
				if tt.mockError != nil {
					return nil, tt.mockError
				}
				return []byte(tt.mockData), nil
			}

			util := &Util{}
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

// Mock DirEntry implementation
type mockDirEntry struct {
	name  string
	isDir bool
}

func (m mockDirEntry) Name() string {
	return m.name
}

func (m mockDirEntry) IsDir() bool {
	return m.isDir
}

func (m mockDirEntry) Type() os.FileMode {
	return 0
}

func (m mockDirEntry) Info() (os.FileInfo, error) {
	return nil, nil
}
func TestDevicePath(t *testing.T) {
	tests := []struct {
		name     string
		device   string
		expected string
	}{
		{
			name:     "valid device",
			device:   "nvme0n1",
			expected: "/dev/nvme0n1",
		},
		{
			name:     "empty device",
			device:   "",
			expected: "/dev",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			util := &Util{}
			path, err := util.DevicePath(tt.device)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, path)
		})
	}
}
