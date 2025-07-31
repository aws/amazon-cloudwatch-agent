//go:build linux

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nvme

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUtil_IsInstanceStoreDevice(t *testing.T) {
	tests := []struct {
		name           string
		device         DeviceFileAttributes
		modelContent   string
		modelError     error
		expectedResult bool
		expectedError  bool
		description    string
	}{
		{
			name:           "ebs device - wrong model name",
			device:         DeviceFileAttributes{controller: 0, namespace: 1, partition: -1, deviceName: "nvme0n1"},
			modelContent:   ebsNvmeModelName,
			expectedResult: false,
			expectedError:  false,
			description:    "Should return false when model name is EBS",
		},
		{
			name:           "unknown device - wrong model name",
			device:         DeviceFileAttributes{controller: 0, namespace: 1, partition: -1, deviceName: "nvme0n1"},
			modelContent:   "Some Other NVMe Device",
			expectedResult: false,
			expectedError:  false,
			description:    "Should return false when model name is not Instance Store",
		},
		{
			name:           "model read error",
			device:         DeviceFileAttributes{controller: 0, namespace: 1, partition: -1, deviceName: "nvme0n1"},
			modelError:     errors.New("failed to read model"),
			expectedResult: false,
			expectedError:  true,
			description:    "Should return error when model file cannot be read",
		},
		{
			name:           "device with invalid controller",
			device:         DeviceFileAttributes{controller: -1, deviceName: "nvme0n1"},
			modelError:     errors.New("unable to re-create device name due to missing controller id"),
			expectedResult: false,
			expectedError:  true,
			description:    "Should return error when device has invalid controller ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock the file system operations
			originalReadFile := osReadFile
			defer func() { osReadFile = originalReadFile }()

			osReadFile = func(filename string) ([]byte, error) {
				if tt.modelError != nil {
					return nil, tt.modelError
				}
				if filename == fmt.Sprintf("%s/nvme%d/model", nvmeSysDirectoryPath, tt.device.Controller()) {
					return []byte(tt.modelContent + "\n"), nil
				}
				return nil, errors.New("unexpected file read")
			}

			util := &Util{}
			result, err := util.IsInstanceStoreDevice(&tt.device)

			if tt.expectedError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}
			assert.Equal(t, tt.expectedResult, result, tt.description)
		})
	}
}

// TestUtil_IsInstanceStoreDevice_ModelNameCheck tests the first part of device identification
// This test focuses on the model name checking logic which can be easily mocked
func TestUtil_IsInstanceStoreDevice_ModelNameCheck(t *testing.T) {
	device := DeviceFileAttributes{controller: 0, namespace: 1, partition: -1, deviceName: "nvme0n1"}

	// Test case where model name matches Instance Store
	t.Run("correct model name", func(t *testing.T) {
		originalReadFile := osReadFile
		defer func() { osReadFile = originalReadFile }()

		osReadFile = func(filename string) ([]byte, error) {
			if filename == fmt.Sprintf("%s/nvme0/model", nvmeSysDirectoryPath) {
				return []byte(instanceStoreNvmeModelName + "\n"), nil
			}
			return nil, errors.New("file not found")
		}

		util := &Util{}

		// Note: This test will attempt to read the log page, which will fail in the test environment
		// The function should return false (not an error) when log page reading fails
		// This tests the graceful handling of ioctl failures
		result, err := util.IsInstanceStoreDevice(&device)

		// Should not return an error, but should return false due to log page read failure
		assert.NoError(t, err, "Should handle log page read failures gracefully")
		assert.False(t, result, "Should return false when log page cannot be read (expected in test environment)")
	})
}

// TestUtil_IsInstanceStoreDevice_Integration documents the integration testing requirements
// This test serves as documentation for what needs to be tested in an actual EC2 environment
func TestUtil_IsInstanceStoreDevice_Integration(t *testing.T) {
	t.Skip("Integration test - requires actual Instance Store device")

	// This test would run on an actual EC2 instance with Instance Store devices
	// It would:
	// 1. Parse a real device name (e.g., "nvme0n1")
	// 2. Read the actual model file from /sys/class/nvme/nvme0/model
	// 3. Attempt to read log page 0xC0 from /dev/nvme0n1
	// 4. Validate the magic number 0xEC2C0D7E
	// 5. Confirm the function returns true for valid Instance Store devices

	// Example integration test code:
	// device, err := ParseNvmeDeviceFileName("nvme0n1")
	// require.NoError(t, err)
	//
	// util := &Util{}
	// result, err := util.IsInstanceStoreDevice(&device)
	//
	// assert.NoError(t, err)
	// assert.True(t, result) // Assuming nvme0n1 is an Instance Store device
}
