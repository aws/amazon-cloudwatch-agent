// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsinstancestorenvmereceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfigValidation(t *testing.T) {
	cfg := createDefaultConfig().(*Config)

	// Verify default configuration
	assert.NotNil(t, cfg)
	assert.Empty(t, cfg.Devices, "default config should have empty devices list")
	assert.NotNil(t, cfg.ControllerConfig, "controller config should be initialized")
	assert.NotNil(t, cfg.MetricsBuilderConfig, "metrics builder config should be initialized")

	// Verify default config is valid
	err := cfg.Validate()
	assert.NoError(t, err, "default config should be valid")
}

func TestConfigValidate_ValidConfigurations(t *testing.T) {
	testCases := []struct {
		name    string
		devices []string
	}{
		{
			name:    "empty devices list",
			devices: []string{},
		},
		{
			name:    "wildcard for auto-discovery",
			devices: []string{"*"},
		},
		{
			name:    "single nvme device",
			devices: []string{"/dev/nvme0n1"},
		},
		{
			name:    "multiple nvme devices",
			devices: []string{"/dev/nvme0n1", "/dev/nvme1n1"},
		},
		{
			name:    "nvme device with partition",
			devices: []string{"/dev/nvme0n1p1"},
		},
		{
			name:    "mixed wildcard and specific devices",
			devices: []string{"*", "/dev/nvme0n1"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.Devices = tc.devices

			err := cfg.Validate()
			assert.NoError(t, err, "configuration should be valid")
		})
	}
}

func TestConfigValidate_InvalidConfigurations(t *testing.T) {
	testCases := []struct {
		name          string
		devices       []string
		expectedError string
	}{
		{
			name:          "empty device path",
			devices:       []string{""},
			expectedError: "device path cannot be empty",
		},
		{
			name:          "device path not starting with /dev/",
			devices:       []string{"nvme0n1"},
			expectedError: "device path must start with /dev/",
		},
		{
			name:          "non-nvme device path",
			devices:       []string{"/dev/sda1"},
			expectedError: "device path must be an NVMe device (/dev/nvme*)",
		},
		{
			name:          "path traversal attempt with ..",
			devices:       []string{"/dev/../etc/passwd"},
			expectedError: "device path cannot contain '..'",
		},
		{
			name:          "path with directory traversal",
			devices:       []string{"/dev/nvme/../nvme0n1"},
			expectedError: "device path cannot contain '..'",
		},
		{
			name:          "invalid characters in path",
			devices:       []string{"/dev/nvme0n1/./"},
			expectedError: "device path contains invalid characters",
		},
		{
			name:          "mixed valid and invalid devices",
			devices:       []string{"/dev/nvme0n1", "/dev/sda1"},
			expectedError: "device path must be an NVMe device (/dev/nvme*)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.Devices = tc.devices

			err := cfg.Validate()
			require.Error(t, err, "configuration should be invalid")
			assert.Contains(t, err.Error(), tc.expectedError, "error message should contain expected text")
		})
	}
}

func TestConfigValidateDevice(t *testing.T) {
	cfg := &Config{}

	testCases := []struct {
		name          string
		device        string
		expectError   bool
		expectedError string
	}{
		{
			name:        "valid nvme device",
			device:      "/dev/nvme0n1",
			expectError: false,
		},
		{
			name:        "valid nvme device with partition",
			device:      "/dev/nvme0n1p1",
			expectError: false,
		},
		{
			name:        "wildcard",
			device:      "*",
			expectError: false,
		},
		{
			name:          "empty device",
			device:        "",
			expectError:   true,
			expectedError: "device path cannot be empty",
		},
		{
			name:          "relative path",
			device:        "nvme0n1",
			expectError:   true,
			expectedError: "device path must start with /dev/",
		},
		{
			name:          "non-nvme device",
			device:        "/dev/sda1",
			expectError:   true,
			expectedError: "device path must be an NVMe device (/dev/nvme*)",
		},
		{
			name:          "path traversal",
			device:        "/dev/../etc/passwd",
			expectError:   true,
			expectedError: "device path cannot contain '..'",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := cfg.validateDevice(tc.device)

			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigDevicesField(t *testing.T) {
	testCases := []struct {
		name    string
		devices []string
	}{
		{
			name:    "empty devices",
			devices: []string{},
		},
		{
			name:    "single device",
			devices: []string{"/dev/nvme0n1"},
		},
		{
			name:    "multiple devices",
			devices: []string{"/dev/nvme0n1", "/dev/nvme1n1"},
		},
		{
			name:    "wildcard",
			devices: []string{"*"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.Devices = tc.devices

			// Verify devices field is set correctly
			assert.Equal(t, tc.devices, cfg.Devices)
		})
	}
}
