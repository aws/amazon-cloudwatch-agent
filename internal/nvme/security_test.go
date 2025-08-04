// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nvme

import (
	"runtime"
	"strings"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// isLinuxPlatform checks if the current platform is Linux
func isLinuxPlatform() bool {
	return runtime.GOOS == "linux"
}

func TestParseEBSLogPageSecurityValidation(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		expectError bool
		errorMsg    string
	}{
		{
			name:        "nil data",
			data:        nil,
			expectError: true,
			errorMsg:    "input data is nil",
		},
		{
			name:        "empty data",
			data:        []byte{},
			expectError: true,
			errorMsg:    "data length 0 is insufficient",
		},
		{
			name:        "insufficient data length",
			data:        make([]byte, 100),
			expectError: true,
			errorMsg:    "data length 100 is insufficient",
		},
		{
			name:        "excessive data length",
			data:        make([]byte, 10000),
			expectError: true,
			errorMsg:    "data length 10000 exceeds maximum allowed size",
		},
		{
			name:        "wrong log page size",
			data:        make([]byte, 2048),
			expectError: true,
			errorMsg:    "data length 2048 is insufficient",
		},
		{
			name:        "invalid magic number",
			data:        createValidEBSLogPageWithMagic(0x12345678),
			expectError: true,
			errorMsg:    "invalid EBS magic number",
		},
		{
			name:        "valid EBS log page",
			data:        createValidEBSLogPageWithMagic(EBSMagicNumber),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseEBSLogPage(tt.data)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestParseInstanceStoreLogPageSecurityValidation(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		expectError bool
		errorMsg    string
	}{
		{
			name:        "nil data",
			data:        nil,
			expectError: true,
			errorMsg:    "input data is nil",
		},
		{
			name:        "empty data",
			data:        []byte{},
			expectError: true,
			errorMsg:    "data length 0 is insufficient",
		},
		{
			name:        "insufficient data length",
			data:        make([]byte, 100),
			expectError: true,
			errorMsg:    "data length 100 is insufficient",
		},
		{
			name:        "excessive data length",
			data:        make([]byte, 10000),
			expectError: true,
			errorMsg:    "data length 10000 exceeds maximum allowed size",
		},
		{
			name:        "wrong log page size",
			data:        make([]byte, 2048),
			expectError: true,
			errorMsg:    "data length 2048 is insufficient",
		},
		{
			name:        "invalid magic number",
			data:        createValidInstanceStoreLogPageWithMagic(0x12345678),
			expectError: true,
			errorMsg:    "invalid Instance Store magic number",
		},
		{
			name:        "valid Instance Store log page",
			data:        createValidInstanceStoreLogPageWithMagic(InstanceStoreMagicNumber),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseInstanceStoreLogPage(tt.data)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateEBSMetricBounds(t *testing.T) {
	tests := []struct {
		name        string
		metrics     EBSMetrics
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid metrics",
			metrics:     createValidEBSMetrics(),
			expectError: false,
		},
		{
			name: "excessive ReadOps",
			metrics: func() EBSMetrics {
				m := createValidEBSMetrics()
				m.ReadOps = 1e13 // Exceeds 1e12 limit
				return m
			}(),
			expectError: true,
			errorMsg:    "ReadOps value",
		},
		{
			name: "excessive WriteOps",
			metrics: func() EBSMetrics {
				m := createValidEBSMetrics()
				m.WriteOps = 1e13 // Exceeds 1e12 limit
				return m
			}(),
			expectError: true,
			errorMsg:    "WriteOps value",
		},
		{
			name: "excessive ReadBytes",
			metrics: func() EBSMetrics {
				m := createValidEBSMetrics()
				m.ReadBytes = 1e19 // Exceeds 1e18 limit
				return m
			}(),
			expectError: true,
			errorMsg:    "ReadBytes value",
		},
		{
			name: "excessive WriteBytes",
			metrics: func() EBSMetrics {
				m := createValidEBSMetrics()
				m.WriteBytes = 1e19 // Exceeds 1e18 limit
				return m
			}(),
			expectError: true,
			errorMsg:    "WriteBytes value",
		},
		{
			name: "excessive TotalReadTime",
			metrics: func() EBSMetrics {
				m := createValidEBSMetrics()
				m.TotalReadTime = 1e19 // Exceeds 1e18 limit
				return m
			}(),
			expectError: true,
			errorMsg:    "TotalReadTime value",
		},
		{
			name: "excessive TotalWriteTime",
			metrics: func() EBSMetrics {
				m := createValidEBSMetrics()
				m.TotalWriteTime = 1e19 // Exceeds 1e18 limit
				return m
			}(),
			expectError: true,
			errorMsg:    "TotalWriteTime value",
		},
		{
			name: "excessive QueueLength",
			metrics: func() EBSMetrics {
				m := createValidEBSMetrics()
				m.QueueLength = 1e7 // Exceeds 1e6 limit
				return m
			}(),
			expectError: true,
			errorMsg:    "QueueLength value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEBSMetricBounds(&tt.metrics)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateInstanceStoreMetricBounds(t *testing.T) {
	tests := []struct {
		name        string
		metrics     InstanceStoreMetrics
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid metrics",
			metrics:     createValidInstanceStoreMetrics(),
			expectError: false,
		},
		{
			name: "excessive ReadOps",
			metrics: func() InstanceStoreMetrics {
				m := createValidInstanceStoreMetrics()
				m.ReadOps = 1e13 // Exceeds 1e12 limit
				return m
			}(),
			expectError: true,
			errorMsg:    "ReadOps value",
		},
		{
			name: "excessive NumHistograms",
			metrics: func() InstanceStoreMetrics {
				m := createValidInstanceStoreMetrics()
				m.NumHistograms = 20 // Exceeds 10 limit
				return m
			}(),
			expectError: true,
			errorMsg:    "NumHistograms value",
		},
		{
			name: "excessive NumBins",
			metrics: func() InstanceStoreMetrics {
				m := createValidInstanceStoreMetrics()
				m.NumBins = 300 // Exceeds 256 limit
				return m
			}(),
			expectError: true,
			errorMsg:    "NumBins value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateInstanceStoreMetricBounds(&tt.metrics)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateHistogramBounds(t *testing.T) {
	tests := []struct {
		name        string
		histogram   Histogram
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid histogram",
			histogram:   createValidHistogram(),
			expectError: false,
		},
		{
			name: "excessive bin count",
			histogram: Histogram{
				BinCount: 300, // Exceeds 256 limit
				Bins:     [64]HistogramBin{},
			},
			expectError: true,
			errorMsg:    "BinCount value",
		},
		{
			name: "invalid bin bounds - lower > upper",
			histogram: Histogram{
				BinCount: 2,
				Bins: [64]HistogramBin{
					{Lower: 100, Upper: 50, Count: 10}, // Invalid: Lower > Upper
					{Lower: 200, Upper: 300, Count: 20},
				},
			},
			expectError: true,
			errorMsg:    "has invalid bounds",
		},
		{
			name: "excessive bin lower value",
			histogram: Histogram{
				BinCount: 1,
				Bins: [64]HistogramBin{
					{Lower: 1e19, Upper: 1e19 + 100, Count: 10}, // Exceeds 1e18 limit
				},
			},
			expectError: true,
			errorMsg:    "Lower value",
		},
		{
			name: "excessive bin upper value",
			histogram: Histogram{
				BinCount: 1,
				Bins: [64]HistogramBin{
					{Lower: 100, Upper: 1e19, Count: 10}, // Exceeds 1e18 limit
				},
			},
			expectError: true,
			errorMsg:    "Upper value",
		},
		{
			name: "excessive bin count value",
			histogram: Histogram{
				BinCount: 1,
				Bins: [64]HistogramBin{
					{Lower: 100, Upper: 200, Count: 1e19}, // Exceeds 1e18 limit
				},
			},
			expectError: true,
			errorMsg:    "Count value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateHistogramBounds(&tt.histogram)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDevicePathSecurityValidation(t *testing.T) {
	// Skip on non-Linux platforms since NVMe operations are not supported
	if !isLinuxPlatform() {
		t.Skip("Skipping NVMe security tests on non-Linux platform")
	}

	util := &Util{}

	tests := []struct {
		name        string
		device      string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid device name",
			device:      "nvme0n1",
			expectError: false,
		},
		{
			name:        "empty device name",
			device:      "",
			expectError: true,
			errorMsg:    "device name cannot be empty",
		},
		{
			name:        "device name with path separator",
			device:      "nvme0n1/test",
			expectError: true,
			errorMsg:    "device name cannot contain path separators",
		},
		{
			name:        "device name with path traversal",
			device:      "nvme0n1/../test",
			expectError: true,
			errorMsg:    "device name cannot contain path separators",
		},
		{
			name:        "device name with invalid characters",
			device:      "nvme0n1@#$",
			expectError: true,
			errorMsg:    "device name contains invalid character",
		},
		{
			name:        "device name too long",
			device:      strings.Repeat("a", 40),
			expectError: true,
			errorMsg:    "device name exceeds maximum length",
		},
		{
			name:        "device name with whitespace",
			device:      "  nvme0n1  ",
			expectError: false, // Should be trimmed and pass
		},
		// Enhanced security test cases
		{
			name:        "null byte injection",
			device:      "nvme0n1\x00",
			expectError: true,
			errorMsg:    "device name cannot contain null bytes",
		},
		{
			name:        "control character injection",
			device:      "nvme0n1\x01",
			expectError: true,
			errorMsg:    "device name contains invalid control character",
		},
		{
			name:        "backslash injection",
			device:      "nvme0n1\\test",
			expectError: true,
			errorMsg:    "device name cannot contain backslashes",
		},
		{
			name:        "path resolution mismatch",
			device:      "nvme0n1/../nvme1n1",
			expectError: true,
			errorMsg:    "device name cannot contain path separators",
		},
		{
			name:        "invalid device pattern - no namespace",
			device:      "nvme0",
			expectError: true,
			errorMsg:    "device name too short for valid NVMe pattern",
		},
		{
			name:        "invalid device pattern - multiple n separators",
			device:      "nvme0n1n2",
			expectError: true,
			errorMsg:    "device name contains multiple namespace separators",
		},
		{
			name:        "invalid device pattern - multiple p separators",
			device:      "nvme0n1p1p2",
			expectError: true,
			errorMsg:    "device name contains multiple partition separators",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := util.DevicePath(tt.device)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestSecurityDeviceNamePatternValidation tests the device name pattern validation for security
func TestSecurityDeviceNamePatternValidation(t *testing.T) {
	// Skip on non-Linux platforms since NVMe operations are not supported
	if !isLinuxPlatform() {
		t.Skip("Skipping NVMe security tests on non-Linux platform")
	}

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
			name:        "with partition",
			deviceName:  "0n1p1",
			expectError: false,
			description: "Device with partition should be valid",
		},

		// Security-focused invalid patterns
		{
			name:        "command injection in controller",
			deviceName:  "0;rm -rf /;n1",
			expectError: true,
			errorMsg:    "controller part contains non-digit character",
			description: "Command injection in controller part should be rejected",
		},
		{
			name:        "command injection in namespace",
			deviceName:  "0n1;rm -rf /",
			expectError: true,
			errorMsg:    "namespace part contains non-digit character",
			description: "Command injection in namespace part should be rejected",
		},
		{
			name:        "command injection in partition",
			deviceName:  "0n1p1;rm -rf /",
			expectError: true,
			errorMsg:    "partition part contains non-digit character",
			description: "Command injection in partition part should be rejected",
		},
		{
			name:        "format string attack",
			deviceName:  "0n%s%d%x",
			expectError: true,
			errorMsg:    "namespace part contains non-digit character",
			description: "Format string attack should be rejected",
		},
		{
			name:        "buffer overflow attempt",
			deviceName:  strings.Repeat("0", 50) + "n1",
			expectError: false, // This should pass pattern validation but fail length validation elsewhere
			description: "Very long device name should be handled",
		},
		{
			name:        "multiple separators attack",
			deviceName:  "0n1n2n3",
			expectError: true,
			errorMsg:    "device name contains multiple namespace separators",
			description: "Multiple namespace separators should be rejected",
		},
		{
			name:        "partition separator attack",
			deviceName:  "0n1p1p2p3",
			expectError: true,
			errorMsg:    "device name contains multiple partition separators",
			description: "Multiple partition separators should be rejected",
		},
		{
			name:        "missing components",
			deviceName:  "n1",
			expectError: true,
			errorMsg:    "missing controller number",
			description: "Missing controller should be rejected",
		},
		{
			name:        "empty controller",
			deviceName:  "n1",
			expectError: true,
			errorMsg:    "missing controller number",
			description: "Empty controller should be rejected",
		},
		{
			name:        "empty namespace",
			deviceName:  "0n",
			expectError: true,
			errorMsg:    "missing namespace number",
			description: "Empty namespace should be rejected",
		},
		{
			name:        "empty partition",
			deviceName:  "0n1p",
			expectError: true,
			errorMsg:    "missing partition number after 'p'",
			description: "Empty partition should be rejected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDeviceNamePattern(tt.deviceName)

			if tt.expectError {
				require.Error(t, err, "Expected error for security test: %s", tt.description)
				assert.Contains(t, err.Error(), tt.errorMsg, "Error message should contain expected text")
			} else {
				require.NoError(t, err, "Expected no error for valid case: %s", tt.description)
			}
		})
	}
}

// Note: isValidNVMeDeviceChar is defined in the receiver package, not here
// This test is moved to the receiver package test file

// Helper functions to create test data

func createValidEBSLogPageWithMagic(magic uint64) []byte {
	// Create a 4KB buffer
	result := make([]byte, 4096)

	// Write the magic number at the beginning (first 8 bytes)
	copy(result[0:8], (*[8]byte)(unsafe.Pointer(&magic))[:])

	// Fill in some basic valid data for other fields to make parsing work
	// This is a simplified approach - in real tests you might want to create
	// a complete valid structure

	return result
}

func createValidInstanceStoreLogPageWithMagic(magic uint32) []byte {
	// Create buffer large enough for InstanceStoreMetrics structure (4136 bytes)
	data := make([]byte, 4136)

	// Write magic number at the beginning (first 4 bytes)
	copy(data[0:4], (*[4]byte)(unsafe.Pointer(&magic))[:])

	return data
}

func createValidEBSMetrics() EBSMetrics {
	return EBSMetrics{
		EBSMagic:              EBSMagicNumber,
		ReadOps:               1000,
		WriteOps:              2000,
		ReadBytes:             1000000,
		WriteBytes:            2000000,
		TotalReadTime:         5000000,
		TotalWriteTime:        10000000,
		EBSIOPSExceeded:       0,
		EBSThroughputExceeded: 0,
		EC2IOPSExceeded:       0,
		EC2ThroughputExceeded: 0,
		QueueLength:           5,
		ReadLatency:           createValidHistogram(),
		WriteLatency:          createValidHistogram(),
	}
}

func createValidInstanceStoreMetrics() InstanceStoreMetrics {
	return InstanceStoreMetrics{
		Magic:                 InstanceStoreMagicNumber,
		Reserved:              0,
		ReadOps:               1000,
		WriteOps:              2000,
		ReadBytes:             1000000,
		WriteBytes:            2000000,
		TotalReadTime:         5000000,
		TotalWriteTime:        10000000,
		EBSIOPSExceeded:       0, // Skipped for Instance Store
		EBSThroughputExceeded: 0, // Skipped for Instance Store
		EC2IOPSExceeded:       0,
		EC2ThroughputExceeded: 0,
		QueueLength:           5,
		NumHistograms:         2,
		NumBins:               4,
		IOSizeRange:           1024,
		ReadLatency:           createValidHistogram(),
		WriteLatency:          createValidHistogram(),
	}
}

func createValidHistogram() Histogram {
	return Histogram{
		BinCount: 4,
		Bins: [64]HistogramBin{
			{Lower: 0, Upper: 100, Count: 10},
			{Lower: 100, Upper: 200, Count: 20},
			{Lower: 200, Upper: 300, Count: 15},
			{Lower: 300, Upper: 400, Count: 5},
		},
	}
}
