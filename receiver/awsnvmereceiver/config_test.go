// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsnvmereceiver

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/aws/amazon-cloudwatch-agent/receiver/awsnvmereceiver/internal/metadata"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid empty config",
			config: Config{
				ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				Devices:              []string{},
			},
			expectError: false,
		},
		{
			name: "valid wildcard config",
			config: Config{
				ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				Devices:              []string{"*"},
			},
			expectError: false,
		},
		{
			name: "valid specific devices",
			config: Config{
				ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				Devices:              []string{"/dev/nvme0n1", "/dev/nvme1n1"},
			},
			expectError: false,
		},
		{
			name: "invalid empty device path",
			config: Config{
				ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				Devices:              []string{""},
			},
			expectError: true,
			errorMsg:    "device path cannot be empty",
		},
		{
			name: "invalid device path with directory traversal",
			config: Config{
				ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				Devices:              []string{"/dev/../etc/passwd"},
			},
			expectError: true,
			errorMsg:    "device path cannot contain '..'",
		},
		{
			name: "invalid device path not starting with /dev/",
			config: Config{
				ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				Devices:              []string{"nvme0n1"},
			},
			expectError: true,
			errorMsg:    "device path must start with /dev/",
		},
		{
			name: "invalid non-nvme device path",
			config: Config{
				ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				Devices:              []string{"/dev/sda1"},
			},
			expectError: true,
			errorMsg:    "device path must be an NVMe device (/dev/nvme*)",
		},
		{
			name: "invalid device path with invalid characters",
			config: Config{
				ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				Devices:              []string{"/dev/nvme0n1/../nvme1n1"},
			},
			expectError: true,
			errorMsg:    "device path cannot contain '..'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateDevice(t *testing.T) {
	cfg := &Config{}

	tests := []struct {
		name        string
		device      string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid wildcard",
			device:      "*",
			expectError: false,
		},
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
			name:        "empty device",
			device:      "",
			expectError: true,
			errorMsg:    "device path cannot be empty",
		},
		{
			name:        "directory traversal",
			device:      "/dev/nvme0n1/../nvme1n1",
			expectError: true,
			errorMsg:    "device path cannot contain '..'",
		},
		{
			name:        "path traversal with ..",
			device:      "/dev/../etc/passwd",
			expectError: true,
			errorMsg:    "device path cannot contain '..'",
		},
		{
			name:        "not starting with /dev/",
			device:      "nvme0n1",
			expectError: true,
			errorMsg:    "device path must start with /dev/",
		},
		{
			name:        "non-nvme device",
			device:      "/dev/sda1",
			expectError: true,
			errorMsg:    "device path must be an NVMe device (/dev/nvme*)",
		},
		// Security-focused test cases
		{
			name:        "null byte injection",
			device:      "/dev/nvme0n1\x00",
			expectError: true,
			errorMsg:    "device path cannot contain null bytes",
		},
		{
			name:        "relative path component",
			device:      "/dev/./nvme0n1",
			expectError: true,
			errorMsg:    "device path cannot contain relative path components",
		},
		{
			name:        "double slash",
			device:      "/dev//nvme0n1",
			expectError: true,
			errorMsg:    "device path cannot contain double slashes",
		},
		{
			name:        "whitespace padding",
			device:      "  /dev/nvme0n1  ",
			expectError: false, // Should be trimmed and pass
		},
		{
			name:        "path too long",
			device:      "/dev/nvme" + strings.Repeat("a", 250),
			expectError: true,
			errorMsg:    "device path exceeds maximum allowed length",
		},
		{
			name:        "invalid characters in device name",
			device:      "/dev/nvme0n1@#$",
			expectError: true,
			errorMsg:    "device path contains invalid characters for NVMe device",
		},
		{
			name:        "path escape attempt",
			device:      "/dev/nvme0n1/../../etc/passwd",
			expectError: true,
			errorMsg:    "device path cannot contain '..'",
		},
		{
			name:        "symlink-like path",
			device:      "/dev/nvme0n1/../../../usr/bin/sh",
			expectError: true,
			errorMsg:    "device path cannot contain '..'",
		},
		{
			name:        "empty nvme device name",
			device:      "/dev/nvme",
			expectError: true,
			errorMsg:    "invalid NVMe device name format",
		},
		{
			name:        "path with backslash",
			device:      "/dev/nvme0n1\\test",
			expectError: true,
			errorMsg:    "device path contains invalid characters for NVMe device",
		},
		// Additional security-focused test cases
		{
			name:        "control character injection",
			device:      "/dev/nvme0n1\x01",
			expectError: true,
			errorMsg:    "device path contains invalid control character",
		},
		{
			name:        "tab character (should be allowed)",
			device:      "/dev/nvme0n1\t",
			expectError: true, // Tab should not be in device names
			errorMsg:    "device path contains invalid control character",
		},
		{
			name:        "unicode characters",
			device:      "/dev/nvme0n1ñ",
			expectError: true,
			errorMsg:    "device path contains invalid character",
		},
		{
			name:        "multiple path separators",
			device:      "/dev/nvme0n1/../nvme1n1",
			expectError: true,
			errorMsg:    "device path cannot contain '..'",
		},
		{
			name:        "absolute path resolution attack",
			device:      "/dev/nvme0n1/../../../etc/passwd",
			expectError: true,
			errorMsg:    "device path cannot contain '..'",
		},
		{
			name:        "device name with invalid NVMe pattern - no namespace",
			device:      "/dev/nvme0",
			expectError: true,
			errorMsg:    "invalid NVMe device name format",
		},
		{
			name:        "device name with invalid NVMe pattern - multiple n separators",
			device:      "/dev/nvme0n1n2",
			expectError: true,
			errorMsg:    "invalid NVMe device name pattern",
		},
		{
			name:        "device name with invalid NVMe pattern - multiple p separators",
			device:      "/dev/nvme0n1p1p2",
			expectError: true,
			errorMsg:    "invalid NVMe device name pattern",
		},
		{
			name:        "device name with invalid controller characters",
			device:      "/dev/nvmeAn1",
			expectError: true,
			errorMsg:    "invalid NVMe device name pattern",
		},
		{
			name:        "device name with invalid namespace characters",
			device:      "/dev/nvme0nA",
			expectError: true,
			errorMsg:    "invalid NVMe device name pattern",
		},
		{
			name:        "device name with invalid partition characters",
			device:      "/dev/nvme0n1pA",
			expectError: true,
			errorMsg:    "invalid NVMe device name pattern",
		},
		{
			name:        "device name missing controller",
			device:      "/dev/nvmen1",
			expectError: true,
			errorMsg:    "invalid NVMe device name pattern",
		},
		{
			name:        "device name missing namespace",
			device:      "/dev/nvme0n",
			expectError: true,
			errorMsg:    "invalid NVMe device name pattern",
		},
		{
			name:        "device name missing partition number",
			device:      "/dev/nvme0n1p",
			expectError: true,
			errorMsg:    "invalid NVMe device name pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cfg.validateDevice(tt.device)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateDevices(t *testing.T) {
	tests := []struct {
		name        string
		devices     []string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "empty devices list",
			devices:     []string{},
			expectError: false,
		},
		{
			name:        "nil devices list",
			devices:     nil,
			expectError: false,
		},
		{
			name:        "valid devices",
			devices:     []string{"/dev/nvme0n1", "/dev/nvme1n1"},
			expectError: false,
		},
		{
			name:        "mixed valid devices with wildcard",
			devices:     []string{"*"},
			expectError: false,
		},
		{
			name:        "invalid device in list",
			devices:     []string{"/dev/nvme0n1", "/dev/sda1"},
			expectError: true,
			errorMsg:    "device path must be an NVMe device (/dev/nvme*)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Devices: tt.devices}
			err := cfg.validateDevices()
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
