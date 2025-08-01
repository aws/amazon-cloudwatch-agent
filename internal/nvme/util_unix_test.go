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

// TestUtil_DetectDeviceType tests the unified device type detection function
func TestUtil_DetectDeviceType(t *testing.T) {
	tests := []struct {
		name          string
		device        DeviceFileAttributes
		modelContent  string
		modelError    error
		expectedType  string
		expectedError bool
		description   string
	}{
		{
			name:          "ebs device detection",
			device:        DeviceFileAttributes{controller: 0, namespace: 1, partition: -1, deviceName: "nvme0n1"},
			modelContent:  ebsNvmeModelName,
			expectedType:  "ebs",
			expectedError: false,
			description:   "Should detect EBS device correctly",
		},
		{
			name:          "instance store device detection - model name only",
			device:        DeviceFileAttributes{controller: 1, namespace: 1, partition: -1, deviceName: "nvme1n1"},
			modelContent:  instanceStoreNvmeModelName,
			expectedType:  "",
			expectedError: false,
			description:   "Should handle Instance Store model name but fail magic number validation in test environment",
		},
		{
			name:          "unknown device model",
			device:        DeviceFileAttributes{controller: 2, namespace: 1, partition: -1, deviceName: "nvme2n1"},
			modelContent:  "Some Unknown NVMe Device",
			expectedType:  "",
			expectedError: true,
			description:   "Should return error for unknown device model",
		},
		{
			name:          "model read error",
			device:        DeviceFileAttributes{controller: 3, namespace: 1, partition: -1, deviceName: "nvme3n1"},
			modelError:    errors.New("failed to read model"),
			expectedType:  "",
			expectedError: true,
			description:   "Should return error when model file cannot be read",
		},
		{
			name:          "device with invalid controller",
			device:        DeviceFileAttributes{controller: -1, deviceName: "nvme0n1"},
			modelError:    errors.New("unable to re-create device name due to missing controller id"),
			expectedType:  "",
			expectedError: true,
			description:   "Should return error when device has invalid controller ID",
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
			deviceType, err := util.DetectDeviceType(&tt.device)

			if tt.expectedError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}
			assert.Equal(t, tt.expectedType, deviceType, tt.description)
		})
	}
}

// TestUtil_DetectDeviceType_EBSDevice tests EBS device detection specifically
func TestUtil_DetectDeviceType_EBSDevice(t *testing.T) {
	device := DeviceFileAttributes{controller: 0, namespace: 1, partition: -1, deviceName: "nvme0n1"}

	originalReadFile := osReadFile
	defer func() { osReadFile = originalReadFile }()

	osReadFile = func(filename string) ([]byte, error) {
		if filename == fmt.Sprintf("%s/nvme0/model", nvmeSysDirectoryPath) {
			return []byte(ebsNvmeModelName + "\n"), nil
		}
		return nil, errors.New("file not found")
	}

	util := &Util{}
	deviceType, err := util.DetectDeviceType(&device)

	assert.NoError(t, err, "Should successfully detect EBS device")
	assert.Equal(t, "ebs", deviceType, "Should return 'ebs' for EBS device")
}

// TestUtil_DetectDeviceType_InstanceStoreDevice tests Instance Store device detection
func TestUtil_DetectDeviceType_InstanceStoreDevice(t *testing.T) {
	device := DeviceFileAttributes{controller: 1, namespace: 1, partition: -1, deviceName: "nvme1n1"}

	originalReadFile := osReadFile
	defer func() { osReadFile = originalReadFile }()

	osReadFile = func(filename string) ([]byte, error) {
		if filename == fmt.Sprintf("%s/nvme1/model", nvmeSysDirectoryPath) {
			return []byte(instanceStoreNvmeModelName + "\n"), nil
		}
		return nil, errors.New("file not found")
	}

	util := &Util{}
	deviceType, err := util.DetectDeviceType(&device)

	// In test environment, this should not error but return empty string
	// because the magic number validation will fail (no actual device)
	assert.NoError(t, err, "Should handle Instance Store model name gracefully in test environment")
	assert.Equal(t, "", deviceType, "Should return empty string when magic number validation fails")
}

// TestUtil_DetectDeviceType_UnknownDevice tests handling of unknown device types
func TestUtil_DetectDeviceType_UnknownDevice(t *testing.T) {
	device := DeviceFileAttributes{controller: 2, namespace: 1, partition: -1, deviceName: "nvme2n1"}

	originalReadFile := osReadFile
	defer func() { osReadFile = originalReadFile }()

	osReadFile = func(filename string) ([]byte, error) {
		if filename == fmt.Sprintf("%s/nvme2/model", nvmeSysDirectoryPath) {
			return []byte("Unknown NVMe Device Model\n"), nil
		}
		return nil, errors.New("file not found")
	}

	util := &Util{}
	deviceType, err := util.DetectDeviceType(&device)

	assert.Error(t, err, "Should return error for unknown device model")
	assert.Contains(t, err.Error(), "unknown device type", "Error should mention unknown device type")
	assert.Contains(t, err.Error(), "Unknown NVMe Device Model", "Error should include the actual model name")
	assert.Equal(t, "", deviceType, "Should return empty string for unknown device")
}

// TestUtil_DetectDeviceType_ErrorHandling tests various error scenarios
func TestUtil_DetectDeviceType_ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		device      DeviceFileAttributes
		mockError   error
		description string
	}{
		{
			name:        "model read permission error",
			device:      DeviceFileAttributes{controller: 0, namespace: 1, partition: -1, deviceName: "nvme0n1"},
			mockError:   errors.New("permission denied"),
			description: "Should handle permission errors gracefully",
		},
		{
			name:        "model file not found",
			device:      DeviceFileAttributes{controller: 1, namespace: 1, partition: -1, deviceName: "nvme1n1"},
			mockError:   errors.New("no such file or directory"),
			description: "Should handle missing model file",
		},
		{
			name:        "invalid controller ID",
			device:      DeviceFileAttributes{controller: -1, deviceName: "invalid"},
			mockError:   errors.New("unable to re-create device name due to missing controller id"),
			description: "Should handle invalid controller ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalReadFile := osReadFile
			defer func() { osReadFile = originalReadFile }()

			osReadFile = func(filename string) ([]byte, error) {
				return nil, tt.mockError
			}

			util := &Util{}
			deviceType, err := util.DetectDeviceType(&tt.device)

			assert.Error(t, err, tt.description)
			assert.Equal(t, "", deviceType, "Should return empty string on error")
		})
	}
}

// TestUtil_DetectDeviceType_MixedDeviceScenarios tests scenarios with multiple device types
func TestUtil_DetectDeviceType_MixedDeviceScenarios(t *testing.T) {
	devices := []struct {
		device       DeviceFileAttributes
		modelContent string
		expectedType string
	}{
		{
			device:       DeviceFileAttributes{controller: 0, namespace: 1, partition: -1, deviceName: "nvme0n1"},
			modelContent: ebsNvmeModelName,
			expectedType: "ebs",
		},
		{
			device:       DeviceFileAttributes{controller: 1, namespace: 1, partition: -1, deviceName: "nvme1n1"},
			modelContent: ebsNvmeModelName,
			expectedType: "ebs",
		},
		{
			device:       DeviceFileAttributes{controller: 2, namespace: 1, partition: -1, deviceName: "nvme2n1"},
			modelContent: instanceStoreNvmeModelName,
			expectedType: "", // Will be empty due to magic number validation failure in test
		},
	}

	originalReadFile := osReadFile
	defer func() { osReadFile = originalReadFile }()

	osReadFile = func(filename string) ([]byte, error) {
		for _, d := range devices {
			if filename == fmt.Sprintf("%s/nvme%d/model", nvmeSysDirectoryPath, d.device.Controller()) {
				return []byte(d.modelContent + "\n"), nil
			}
		}
		return nil, errors.New("file not found")
	}

	util := &Util{}

	for _, d := range devices {
		t.Run(fmt.Sprintf("device_%s", d.device.DeviceName()), func(t *testing.T) {
			deviceType, err := util.DetectDeviceType(&d.device)

			if d.expectedType == "" {
				// For Instance Store devices in test environment, we expect no error but empty type
				assert.NoError(t, err, "Should handle Instance Store gracefully in test environment")
			} else {
				assert.NoError(t, err, "Should successfully detect device type")
			}
			assert.Equal(t, d.expectedType, deviceType, "Should return correct device type")
		})
	}
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

// TestUtil_DetectDeviceType_Integration documents integration testing for DetectDeviceType
func TestUtil_DetectDeviceType_Integration(t *testing.T) {
	t.Skip("Integration test - requires actual EC2 environment with mixed devices")

	// This test would run on an actual EC2 instance with both EBS and Instance Store devices
	// It would:
	// 1. Parse real device names from /dev (e.g., "nvme0n1", "nvme1n1")
	// 2. Call DetectDeviceType for each device
	// 3. Verify that EBS devices return "ebs"
	// 4. Verify that Instance Store devices return "instance_store"
	// 5. Ensure no unknown device types are returned for valid AWS NVMe devices

	// Example integration test code:
	// util := &Util{}
	// devices, err := util.GetAllDevices()
	// require.NoError(t, err)
	//
	// for _, device := range devices {
	//     deviceType, err := util.DetectDeviceType(&device)
	//     assert.NoError(t, err)
	//     assert.Contains(t, []string{"ebs", "instance_store"}, deviceType)
	// }
}
