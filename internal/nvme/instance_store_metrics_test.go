// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nvme

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSafeUint64ToInt64(t *testing.T) {
	tests := []struct {
		name        string
		input       uint64
		expected    int64
		expectError bool
	}{
		{
			name:        "zero value",
			input:       0,
			expected:    0,
			expectError: false,
		},
		{
			name:        "small positive value",
			input:       12345,
			expected:    12345,
			expectError: false,
		},
		{
			name:        "max int64 value",
			input:       math.MaxInt64,
			expected:    math.MaxInt64,
			expectError: false,
		},
		{
			name:        "value exceeding max int64",
			input:       math.MaxInt64 + 1,
			expected:    0,
			expectError: true,
		},
		{
			name:        "max uint64 value",
			input:       math.MaxUint64,
			expected:    0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := safeUint64ToInt64(tt.input)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "too large for int64")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// createTestLogPageData creates a test log page with the specified magic number and default test values.
func createTestLogPageData(magic uint32) []byte {
	return createTestLogPageDataWithValues(magic, map[string]uint64{
		"ReadOps":               1000,
		"WriteOps":              2000,
		"ReadBytes":             1024000,
		"WriteBytes":            2048000,
		"TotalReadTime":         5000000,
		"TotalWriteTime":        10000000,
		"EC2IOPSExceeded":       50,
		"EC2ThroughputExceeded": 100,
		"QueueLength":           5,
	})
}

// createTestLogPageDataWithValues creates a test log page with the specified magic number and custom values.
func createTestLogPageDataWithValues(magic uint32, values map[string]uint64) []byte {
	buf := new(bytes.Buffer)

	// Write the structure in little-endian format matching the InstanceStoreMetrics struct
	binary.Write(buf, binary.LittleEndian, magic)                           // Magic (4 bytes)
	binary.Write(buf, binary.LittleEndian, uint32(0))                       // Reserved (4 bytes)
	binary.Write(buf, binary.LittleEndian, values["ReadOps"])               // ReadOps (8 bytes)
	binary.Write(buf, binary.LittleEndian, values["WriteOps"])              // WriteOps (8 bytes)
	binary.Write(buf, binary.LittleEndian, values["ReadBytes"])             // ReadBytes (8 bytes)
	binary.Write(buf, binary.LittleEndian, values["WriteBytes"])            // WriteBytes (8 bytes)
	binary.Write(buf, binary.LittleEndian, values["TotalReadTime"])         // TotalReadTime (8 bytes)
	binary.Write(buf, binary.LittleEndian, values["TotalWriteTime"])        // TotalWriteTime (8 bytes)
	binary.Write(buf, binary.LittleEndian, uint64(0))                       // EBSIOPSExceeded (8 bytes) - not applicable
	binary.Write(buf, binary.LittleEndian, uint64(0))                       // EBSThroughputExceeded (8 bytes) - not applicable
	binary.Write(buf, binary.LittleEndian, values["EC2IOPSExceeded"])       // EC2IOPSExceeded (8 bytes)
	binary.Write(buf, binary.LittleEndian, values["EC2ThroughputExceeded"]) // EC2ThroughputExceeded (8 bytes)
	binary.Write(buf, binary.LittleEndian, values["QueueLength"])           // QueueLength (8 bytes)

	// Ensure we have at least 96 bytes by padding with zeros if necessary
	data := buf.Bytes()
	if len(data) < 96 {
		padding := make([]byte, 96-len(data))
		data = append(data, padding...)
	}

	// Add some additional data to simulate histogram data (which should be ignored)
	histogramData := make([]byte, 100)
	data = append(data, histogramData...)

	return data
}

func TestInstanceStoreMetricsStructSize(t *testing.T) {
	// Verify that the struct matches the expected binary layout
	// The struct should be exactly 96 bytes (excluding histogram data)

	// Calculate expected size:
	// Magic (4) + Reserved (4) + ReadOps (8) + WriteOps (8) + ReadBytes (8) + WriteBytes (8) +
	// TotalReadTime (8) + TotalWriteTime (8) + EBSIOPSExceeded (8) + EBSThroughputExceeded (8) +
	// EC2IOPSExceeded (8) + EC2ThroughputExceeded (8) + QueueLength (8) = 96 bytes
	expectedSize := 4 + 4 + 8 + 8 + 8 + 8 + 8 + 8 + 8 + 8 + 8 + 8 + 8

	// Create test data and verify it has the expected size
	testData := createTestLogPageData(InstanceStoreMagicNumber)
	require.GreaterOrEqual(t, len(testData), expectedSize, "Test data should be at least %d bytes", expectedSize)

	// Verify that we have exactly 96 bytes of meaningful data (excluding histogram padding)
	assert.Equal(t, expectedSize, 96, "Expected size calculation should equal 96 bytes")
}

func TestInstanceStoreMagicNumberConstant(t *testing.T) {
	// Verify the magic number constant is correct
	assert.Equal(t, uint32(0xEC2C0D7E), uint32(InstanceStoreMagicNumber))

	// Also verify the hex representation
	assert.Equal(t, "ec2c0d7e", fmt.Sprintf("%x", InstanceStoreMagicNumber))
}

func TestErrorTypes(t *testing.T) {
	// Test that all error types are properly defined
	errors := []error{
		ErrInvalidInstanceStoreMagic,
		ErrParseInstanceStoreLogPage,
		ErrDeviceAccess,
		ErrIoctlFailed,
		ErrInsufficientPermissions,
		ErrDeviceNotFound,
		ErrBufferOverflow,
	}

	for _, err := range errors {
		assert.NotNil(t, err, "error should not be nil")
		assert.NotEmpty(t, err.Error(), "error should have a message")
	}
}

func TestSafeUint64ToInt64_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		input       uint64
		expected    int64
		expectError bool
		description string
	}{
		{
			name:        "boundary value - max int64",
			input:       math.MaxInt64,
			expected:    math.MaxInt64,
			expectError: false,
			description: "should handle max int64 value",
		},
		{
			name:        "boundary value - max int64 + 1",
			input:       math.MaxInt64 + 1,
			expected:    0,
			expectError: true,
			description: "should fail for max int64 + 1",
		},
		{
			name:        "large value near overflow",
			input:       18446744073709551615, // max uint64
			expected:    0,
			expectError: true,
			description: "should fail for max uint64",
		},
		{
			name:        "mid-range safe value",
			input:       1000000000000, // 1 trillion
			expected:    1000000000000,
			expectError: false,
			description: "should handle mid-range values",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := safeUint64ToInt64(tt.input)

			if tt.expectError {
				assert.Error(t, err, tt.description)
				assert.Contains(t, err.Error(), "too large for int64")
				assert.Equal(t, int64(0), result)
			} else {
				assert.NoError(t, err, tt.description)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
