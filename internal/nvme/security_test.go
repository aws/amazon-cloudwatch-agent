// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nvme

import (
	"strings"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
