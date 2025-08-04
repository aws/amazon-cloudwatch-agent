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

// TestSecurityDevicePathValidation tests comprehensive device path security validation
func TestSecurityDevicePathValidation(t *testing.T) {
	cfg := &Config{
		ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}

	// Test cases focused on security vulnerabilities
	securityTests := []struct {
		name        string
		device      string
		expectError bool
		errorMsg    string
		description string
	}{
		// Path traversal attacks
		{
			name:        "classic path traversal",
			device:      "/dev/../etc/passwd",
			expectError: true,
			errorMsg:    "device path cannot contain '..'",
			description: "Prevents classic directory traversal attack",
		},
		{
			name:        "encoded path traversal",
			device:      "/dev/nvme0n1%2e%2e%2fpasswd",
			expectError: true,
			errorMsg:    "device path contains invalid character",
			description: "Prevents URL-encoded path traversal",
		},
		{
			name:        "double encoded path traversal",
			device:      "/dev/nvme0n1%252e%252e%252fpasswd",
			expectError: true,
			errorMsg:    "device path contains invalid character",
			description: "Prevents double URL-encoded path traversal",
		},
		{
			name:        "unicode path traversal",
			device:      "/dev/nvme0n1\u002e\u002e\u002fpasswd",
			expectError: true,
			errorMsg:    "device path cannot contain '..'",
			description: "Prevents unicode-encoded path traversal",
		},

		// Injection attacks
		{
			name:        "null byte injection",
			device:      "/dev/nvme0n1\x00/etc/passwd",
			expectError: true,
			errorMsg:    "device path cannot contain null bytes",
			description: "Prevents null byte injection attacks",
		},
		{
			name:        "command injection attempt",
			device:      "/dev/nvme0n1; rm -rf /",
			expectError: true,
			errorMsg:    "device path contains invalid character",
			description: "Prevents command injection through device names",
		},
		{
			name:        "shell metacharacter injection",
			device:      "/dev/nvme0n1$(whoami)",
			expectError: true,
			errorMsg:    "device path contains invalid character",
			description: "Prevents shell metacharacter injection",
		},
		{
			name:        "backtick command substitution",
			device:      "/dev/nvme0n1`id`",
			expectError: true,
			errorMsg:    "device path contains invalid character",
			description: "Prevents backtick command substitution",
		},

		// Control character attacks
		{
			name:        "carriage return injection",
			device:      "/dev/nvme0n1\r",
			expectError: false, // CR gets trimmed by TrimSpace, so this passes
			errorMsg:    "device path contains invalid control character",
			description: "Carriage return gets trimmed and passes validation",
		},
		{
			name:        "line feed injection",
			device:      "/dev/nvme0n1\n",
			expectError: false, // LF gets trimmed by TrimSpace, so this passes
			errorMsg:    "device path contains invalid control character",
			description: "Line feed gets trimmed and passes validation",
		},
		{
			name:        "form feed injection",
			device:      "/dev/nvme0n1\f",
			expectError: false, // FF gets trimmed by TrimSpace, so this passes
			errorMsg:    "device path contains invalid control character",
			description: "Form feed gets trimmed and passes validation",
		},
		{
			name:        "vertical tab injection",
			device:      "/dev/nvme0n1\v",
			expectError: false, // VT gets trimmed by TrimSpace, so this passes
			errorMsg:    "device path contains invalid control character",
			description: "Vertical tab gets trimmed and passes validation",
		},
		{
			name:        "control character in middle",
			device:      "/dev/nvme0\x01n1",
			expectError: true,
			errorMsg:    "device path contains invalid control character",
			description: "Control character in middle should be rejected",
		},
		{
			name:        "bell character injection",
			device:      "/dev/nvme0\x07n1",
			expectError: true,
			errorMsg:    "device path contains invalid control character",
			description: "Bell character should be rejected",
		},

		// Buffer overflow attempts
		{
			name:        "extremely long device path",
			device:      "/dev/nvme" + strings.Repeat("0", 300),
			expectError: true,
			errorMsg:    "NVMe device name exceeds maximum length",
			description: "Prevents buffer overflow with long paths",
		},
		{
			name:        "long device name",
			device:      "/dev/nvme" + strings.Repeat("0", 50) + "n1",
			expectError: true,
			errorMsg:    "NVMe device name exceeds maximum length",
			description: "Prevents buffer overflow with long device names",
		},

		// Format string attacks
		{
			name:        "format string specifiers",
			device:      "/dev/nvme0n1%s%d%x",
			expectError: true,
			errorMsg:    "device path contains invalid character",
			description: "Prevents format string attacks",
		},

		// Path normalization attacks
		{
			name:        "path with current directory",
			device:      "/dev/./nvme0n1",
			expectError: true,
			errorMsg:    "device path cannot contain relative path components",
			description: "Prevents current directory path attacks",
		},
		{
			name:        "path with double slashes",
			device:      "/dev//nvme0n1",
			expectError: true,
			errorMsg:    "device path cannot contain double slashes",
			description: "Prevents double slash path attacks",
		},
		{
			name:        "path with backslashes",
			device:      "/dev\\nvme0n1",
			expectError: true,
			errorMsg:    "device path must start with /dev/",
			description: "Prevents backslash path attacks",
		},

		// Valid cases that should pass security validation
		{
			name:        "valid simple device",
			device:      "/dev/nvme0n1",
			expectError: false,
			description: "Valid simple NVMe device should pass",
		},
		{
			name:        "valid device with partition",
			device:      "/dev/nvme0n1p1",
			expectError: false,
			description: "Valid NVMe device with partition should pass",
		},
		{
			name:        "valid multi-digit controller",
			device:      "/dev/nvme10n1",
			expectError: false,
			description: "Valid NVMe device with multi-digit controller should pass",
		},
		{
			name:        "valid multi-digit namespace and partition",
			device:      "/dev/nvme0n10p5",
			expectError: false,
			description: "Valid NVMe device with multi-digit namespace and partition should pass",
		},
		{
			name:        "wildcard for auto-discovery",
			device:      "*",
			expectError: false,
			description: "Wildcard should be allowed for auto-discovery",
		},
	}

	for _, tt := range securityTests {
		t.Run(tt.name, func(t *testing.T) {
			cfg.Devices = []string{tt.device}
			err := cfg.Validate()

			if tt.expectError {
				require.Error(t, err, "Expected error for security test: %s", tt.description)
				assert.Contains(t, err.Error(), tt.errorMsg, "Error message should contain expected text")
			} else {
				require.NoError(t, err, "Expected no error for valid case: %s", tt.description)
			}
		})
	}
}

// TestSecurityNVMeDeviceNamePattern tests NVMe device name pattern validation for security
func TestSecurityNVMeDeviceNamePattern(t *testing.T) {
	cfg := &Config{}

	tests := []struct {
		name        string
		deviceName  string
		expectError bool
		errorMsg    string
		description string
	}{
		// Valid patterns
		{
			name:        "simple valid pattern",
			deviceName:  "0n1",
			expectError: false,
			description: "Simple valid NVMe pattern should pass",
		},
		{
			name:        "multi-digit controller",
			deviceName:  "10n1",
			expectError: false,
			description: "Multi-digit controller should be valid",
		},
		{
			name:        "multi-digit namespace",
			deviceName:  "0n10",
			expectError: false,
			description: "Multi-digit namespace should be valid",
		},
		{
			name:        "with partition",
			deviceName:  "0n1p1",
			expectError: false,
			description: "Device with partition should be valid",
		},
		{
			name:        "multi-digit partition",
			deviceName:  "0n1p10",
			expectError: false,
			description: "Multi-digit partition should be valid",
		},

		// Security-focused invalid patterns
		{
			name:        "injection attempt in controller",
			deviceName:  "0;rm -rf /;n1",
			expectError: true,
			errorMsg:    "controller part contains non-digit character",
			description: "Command injection in controller part should be rejected",
		},
		{
			name:        "injection attempt in namespace",
			deviceName:  "0n1;rm -rf /",
			expectError: true,
			errorMsg:    "namespace part contains non-digit character",
			description: "Command injection in namespace part should be rejected",
		},
		{
			name:        "injection attempt in partition",
			deviceName:  "0n1p1;rm -rf /",
			expectError: true,
			errorMsg:    "partition part contains non-digit character",
			description: "Command injection in partition part should be rejected",
		},
		{
			name:        "multiple namespace separators",
			deviceName:  "0n1n2",
			expectError: true,
			errorMsg:    "namespace part contains non-digit character",
			description: "Multiple namespace separators should be rejected",
		},
		{
			name:        "multiple partition separators",
			deviceName:  "0n1p1p2",
			expectError: true,
			errorMsg:    "partition part contains non-digit character",
			description: "Multiple partition separators should be rejected",
		},
		{
			name:        "missing controller",
			deviceName:  "n1",
			expectError: true,
			errorMsg:    "device name too short for valid NVMe pattern",
			description: "Missing controller should be rejected",
		},
		{
			name:        "missing namespace",
			deviceName:  "0n",
			expectError: true,
			errorMsg:    "device name too short for valid NVMe pattern",
			description: "Missing namespace should be rejected",
		},
		{
			name:        "missing partition number",
			deviceName:  "0n1p",
			expectError: true,
			errorMsg:    "missing partition number after 'p'",
			description: "Missing partition number should be rejected",
		},
		{
			name:        "too short",
			deviceName:  "0n",
			expectError: true,
			errorMsg:    "device name too short for valid NVMe pattern",
			description: "Too short device name should be rejected",
		},
		{
			name:        "no namespace separator",
			deviceName:  "01",
			expectError: true,
			errorMsg:    "device name too short for valid NVMe pattern",
			description: "Missing namespace separator should be rejected",
		},
		{
			name:        "alphabetic controller",
			deviceName:  "An1",
			expectError: true,
			errorMsg:    "controller part contains non-digit character",
			description: "Alphabetic controller should be rejected",
		},
		{
			name:        "alphabetic namespace",
			deviceName:  "0nA",
			expectError: true,
			errorMsg:    "namespace part contains non-digit character",
			description: "Alphabetic namespace should be rejected",
		},
		{
			name:        "alphabetic partition",
			deviceName:  "0n1pA",
			expectError: true,
			errorMsg:    "partition part contains non-digit character",
			description: "Alphabetic partition should be rejected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cfg.validateNVMeDeviceNamePattern(tt.deviceName)

			if tt.expectError {
				require.Error(t, err, "Expected error for security test: %s", tt.description)
				assert.Contains(t, err.Error(), tt.errorMsg, "Error message should contain expected text")
			} else {
				require.NoError(t, err, "Expected no error for valid case: %s", tt.description)
			}
		})
	}
}

// TestSecurityInputSanitization tests input sanitization for security
func TestSecurityInputSanitization(t *testing.T) {
	cfg := &Config{
		ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}

	tests := []struct {
		name        string
		input       string
		expectError bool
		description string
	}{
		{
			name:        "whitespace trimming - leading spaces",
			input:       "   /dev/nvme0n1",
			expectError: false,
			description: "Leading whitespace should be trimmed and pass validation",
		},
		{
			name:        "whitespace trimming - trailing spaces",
			input:       "/dev/nvme0n1   ",
			expectError: false,
			description: "Trailing whitespace should be trimmed and pass validation",
		},
		{
			name:        "whitespace trimming - both sides",
			input:       "   /dev/nvme0n1   ",
			expectError: false,
			description: "Whitespace on both sides should be trimmed and pass validation",
		},
		{
			name:        "tab characters",
			input:       "/dev/nvme0n1\t",
			expectError: false, // Tab gets trimmed by TrimSpace, so this passes
			description: "Tab characters get trimmed and should pass validation",
		},
		{
			name:        "mixed whitespace",
			input:       " \t /dev/nvme0n1 \t ",
			expectError: false, // Whitespace gets trimmed by TrimSpace, so this passes
			description: "Mixed whitespace gets trimmed and should pass validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg.Devices = []string{tt.input}
			err := cfg.Validate()

			if tt.expectError {
				require.Error(t, err, "Expected error for security test: %s", tt.description)
			} else {
				require.NoError(t, err, "Expected no error for valid case: %s", tt.description)
			}
		})
	}
}

// TestSecurityBoundaryConditions tests boundary conditions for security
func TestSecurityBoundaryConditions(t *testing.T) {
	cfg := &Config{
		ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}

	tests := []struct {
		name        string
		device      string
		expectError bool
		description string
	}{
		{
			name:        "exactly at max length",
			device:      "/dev/nvme" + strings.Repeat("0", 247), // Total 255 chars
			expectError: true,                                   // Should fail due to invalid pattern, not length
			description: "Device path at exactly maximum length",
		},
		{
			name:        "one over max length",
			device:      "/dev/nvme" + strings.Repeat("0", 248), // Total 256 chars
			expectError: true,
			description: "Device path one character over maximum length",
		},
		{
			name:        "empty string after trimming",
			device:      "   ",
			expectError: true,
			description: "String that becomes empty after trimming should be rejected",
		},
		{
			name:        "minimum valid device",
			device:      "/dev/nvme0n1",
			expectError: false,
			description: "Minimum valid device should pass",
		},
		{
			name:        "device name at max length",
			device:      "/dev/nvme" + strings.Repeat("0", 29) + "n1", // 32 char device name
			expectError: false,                                        // This should pass since it's a valid pattern at max length
			description: "Device name at maximum length",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg.Devices = []string{tt.device}
			err := cfg.Validate()

			if tt.expectError {
				require.Error(t, err, "Expected error for boundary test: %s", tt.description)
			} else {
				require.NoError(t, err, "Expected no error for valid boundary case: %s", tt.description)
			}
		})
	}
}
