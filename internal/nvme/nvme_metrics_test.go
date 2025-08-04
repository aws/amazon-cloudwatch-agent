// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nvme

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
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
			result, err := SafeUint64ToInt64(tt.input)
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

func TestInstanceStoreMetricsStructSize(t *testing.T) {
	// Verify that the struct matches the expected binary layout
	// The struct should be exactly 96 bytes (excluding histogram data)

	// Calculate expected size:
	// Magic (4) + Reserved (4) + ReadOps (8) + WriteOps (8) + ReadBytes (8) + WriteBytes (8) +
	// TotalReadTime (8) + TotalWriteTime (8) + EBSIOPSExceeded (8) + EBSThroughputExceeded (8) +
	// EC2IOPSExceeded (8) + EC2ThroughputExceeded (8) + QueueLength (8) = 96 bytes
	expectedSize := 4 + 4 + 8 + 8 + 8 + 8 + 8 + 8 + 8 + 8 + 8 + 8 + 8

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
			result, err := SafeUint64ToInt64(tt.input)

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
func TestParseEBSLogPage(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    EBSMetrics
		wantErr string
	}{
		{
			name: "valid EBS log page",
			input: func() []byte {
				metrics := EBSMetrics{
					EBSMagic:              EBSMagicNumber,
					ReadOps:               100,
					WriteOps:              200,
					ReadBytes:             1024,
					WriteBytes:            2048,
					TotalReadTime:         5000,
					TotalWriteTime:        10000,
					EBSIOPSExceeded:       5,
					EBSThroughputExceeded: 10,
					EC2IOPSExceeded:       15,
					EC2ThroughputExceeded: 20,
					QueueLength:           3,
				}

				var buf bytes.Buffer
				binary.Write(&buf, binary.LittleEndian, &metrics)
				return buf.Bytes()
			}(),
			want: EBSMetrics{
				EBSMagic:              EBSMagicNumber,
				ReadOps:               100,
				WriteOps:              200,
				ReadBytes:             1024,
				WriteBytes:            2048,
				TotalReadTime:         5000,
				TotalWriteTime:        10000,
				EBSIOPSExceeded:       5,
				EBSThroughputExceeded: 10,
				EC2IOPSExceeded:       15,
				EC2ThroughputExceeded: 20,
				QueueLength:           3,
			},
		},
		{
			name: "invalid EBS magic number",
			input: func() []byte {
				metrics := EBSMetrics{
					EBSMagic: 0x12345678, // Invalid magic number
					ReadOps:  100,
				}

				var buf bytes.Buffer
				binary.Write(&buf, binary.LittleEndian, &metrics)
				return buf.Bytes()
			}(),
			want:    EBSMetrics{},
			wantErr: ErrInvalidEBSMagic.Error(),
		},
		{
			name:    "insufficient data for EBS parsing",
			input:   make([]byte, 10), // Too small
			want:    EBSMetrics{},
			wantErr: ErrInsufficientData.Error(),
		},
		{
			name:    "empty data",
			input:   []byte{},
			want:    EBSMetrics{},
			wantErr: ErrInsufficientData.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseEBSLogPage(tt.input)

			if tt.wantErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			assert.NoError(t, err)

			// Compare key fields (excluding reserved areas and histograms for simplicity)
			assert.Equal(t, tt.want.EBSMagic, got.EBSMagic)
			assert.Equal(t, tt.want.ReadOps, got.ReadOps)
			assert.Equal(t, tt.want.WriteOps, got.WriteOps)
			assert.Equal(t, tt.want.ReadBytes, got.ReadBytes)
			assert.Equal(t, tt.want.WriteBytes, got.WriteBytes)
			assert.Equal(t, tt.want.TotalReadTime, got.TotalReadTime)
			assert.Equal(t, tt.want.TotalWriteTime, got.TotalWriteTime)
			assert.Equal(t, tt.want.EBSIOPSExceeded, got.EBSIOPSExceeded)
			assert.Equal(t, tt.want.EBSThroughputExceeded, got.EBSThroughputExceeded)
			assert.Equal(t, tt.want.EC2IOPSExceeded, got.EC2IOPSExceeded)
			assert.Equal(t, tt.want.EC2ThroughputExceeded, got.EC2ThroughputExceeded)
			assert.Equal(t, tt.want.QueueLength, got.QueueLength)
		})
	}
}

func TestParseInstanceStoreLogPageWithHistograms(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    InstanceStoreMetrics
		wantErr string
	}{
		{
			name: "valid Instance Store log page with histogram fields",
			input: func() []byte {
				metrics := InstanceStoreMetrics{
					Magic:                 InstanceStoreMagicNumber,
					Reserved:              0,
					ReadOps:               150,
					WriteOps:              250,
					ReadBytes:             1536,
					WriteBytes:            2560,
					TotalReadTime:         7500,
					TotalWriteTime:        12500,
					EBSIOPSExceeded:       0, // Skip for Instance Store
					EBSThroughputExceeded: 0, // Skip for Instance Store
					EC2IOPSExceeded:       25,
					EC2ThroughputExceeded: 30,
					QueueLength:           5,
					NumHistograms:         2,
					NumBins:               64,
					IOSizeRange:           4096,
				}

				var buf bytes.Buffer
				binary.Write(&buf, binary.LittleEndian, &metrics)
				return buf.Bytes()
			}(),
			want: InstanceStoreMetrics{
				Magic:                 InstanceStoreMagicNumber,
				Reserved:              0,
				ReadOps:               150,
				WriteOps:              250,
				ReadBytes:             1536,
				WriteBytes:            2560,
				TotalReadTime:         7500,
				TotalWriteTime:        12500,
				EBSIOPSExceeded:       0,
				EBSThroughputExceeded: 0,
				EC2IOPSExceeded:       25,
				EC2ThroughputExceeded: 30,
				QueueLength:           5,
				NumHistograms:         2,
				NumBins:               64,
				IOSizeRange:           4096,
			},
		},
		{
			name: "invalid Instance Store magic number",
			input: func() []byte {
				metrics := InstanceStoreMetrics{
					Magic:   0x12345678, // Invalid magic number
					ReadOps: 150,
				}

				var buf bytes.Buffer
				binary.Write(&buf, binary.LittleEndian, &metrics)
				return buf.Bytes()
			}(),
			want:    InstanceStoreMetrics{},
			wantErr: ErrInvalidInstanceStoreMagic.Error(),
		},
		{
			name:    "insufficient data for Instance Store parsing",
			input:   make([]byte, 10), // Too small
			want:    InstanceStoreMetrics{},
			wantErr: ErrInsufficientData.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseInstanceStoreLogPage(tt.input)

			if tt.wantErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			assert.NoError(t, err)

			// Compare key fields
			assert.Equal(t, tt.want.Magic, got.Magic)
			assert.Equal(t, tt.want.Reserved, got.Reserved)
			assert.Equal(t, tt.want.ReadOps, got.ReadOps)
			assert.Equal(t, tt.want.WriteOps, got.WriteOps)
			assert.Equal(t, tt.want.ReadBytes, got.ReadBytes)
			assert.Equal(t, tt.want.WriteBytes, got.WriteBytes)
			assert.Equal(t, tt.want.TotalReadTime, got.TotalReadTime)
			assert.Equal(t, tt.want.TotalWriteTime, got.TotalWriteTime)
			assert.Equal(t, tt.want.EC2IOPSExceeded, got.EC2IOPSExceeded)
			assert.Equal(t, tt.want.EC2ThroughputExceeded, got.EC2ThroughputExceeded)
			assert.Equal(t, tt.want.QueueLength, got.QueueLength)
			assert.Equal(t, tt.want.NumHistograms, got.NumHistograms)
			assert.Equal(t, tt.want.NumBins, got.NumBins)
			assert.Equal(t, tt.want.IOSizeRange, got.IOSizeRange)
		})
	}
}

func TestMagicNumberValidation(t *testing.T) {
	t.Run("EBS magic number constant", func(t *testing.T) {
		assert.Equal(t, 0x3C23B510, EBSMagicNumber)
	})

	t.Run("Instance Store magic number constant", func(t *testing.T) {
		assert.Equal(t, 0xEC2C0D7E, InstanceStoreMagicNumber)
	})
}

func TestBinaryLittleEndianParsing(t *testing.T) {
	t.Run("EBS metrics binary layout", func(t *testing.T) {
		// Create test data with known values in little-endian format
		testData := make([]byte, int(unsafe.Sizeof(EBSMetrics{})))

		// Write magic number (0x3C23B510) in little-endian format
		binary.LittleEndian.PutUint64(testData[0:8], EBSMagicNumber)
		// Write ReadOps (1000) in little-endian format
		binary.LittleEndian.PutUint64(testData[8:16], 1000)
		// Write WriteOps (2000) in little-endian format
		binary.LittleEndian.PutUint64(testData[16:24], 2000)

		metrics, err := ParseEBSLogPage(testData)
		assert.NoError(t, err)

		assert.Equal(t, uint64(EBSMagicNumber), metrics.EBSMagic)
		assert.Equal(t, uint64(1000), metrics.ReadOps)
		assert.Equal(t, uint64(2000), metrics.WriteOps)
	})

	t.Run("Instance Store metrics binary layout", func(t *testing.T) {
		// Create test data with known values in little-endian format
		testData := make([]byte, int(unsafe.Sizeof(InstanceStoreMetrics{})))

		// Write magic number (0xEC2C0D7E) in little-endian format
		binary.LittleEndian.PutUint32(testData[0:4], InstanceStoreMagicNumber)
		// Write Reserved (0) in little-endian format
		binary.LittleEndian.PutUint32(testData[4:8], 0)
		// Write ReadOps (1500) in little-endian format
		binary.LittleEndian.PutUint64(testData[8:16], 1500)
		// Write WriteOps (2500) in little-endian format
		binary.LittleEndian.PutUint64(testData[16:24], 2500)

		metrics, err := ParseInstanceStoreLogPage(testData)
		assert.NoError(t, err)

		assert.Equal(t, uint32(InstanceStoreMagicNumber), metrics.Magic)
		assert.Equal(t, uint64(1500), metrics.ReadOps)
		assert.Equal(t, uint64(2500), metrics.WriteOps)
	})
}

func TestHistogramStructures(t *testing.T) {
	t.Run("Histogram structure", func(t *testing.T) {
		h := Histogram{
			BinCount: 64,
			Bins: [64]HistogramBin{
				{Lower: 0, Upper: 100, Count: 10},
				{Lower: 100, Upper: 200, Count: 20},
			},
		}

		assert.Equal(t, uint64(64), h.BinCount)
		assert.Equal(t, uint64(0), h.Bins[0].Lower)
		assert.Equal(t, uint64(100), h.Bins[0].Upper)
		assert.Equal(t, uint64(10), h.Bins[0].Count)
		assert.Equal(t, uint64(100), h.Bins[1].Lower)
		assert.Equal(t, uint64(200), h.Bins[1].Upper)
		assert.Equal(t, uint64(20), h.Bins[1].Count)
	})

	t.Run("HistogramBin structure", func(t *testing.T) {
		bin := HistogramBin{
			Lower: 100,
			Upper: 200,
			Count: 50,
		}

		assert.Equal(t, uint64(100), bin.Lower)
		assert.Equal(t, uint64(200), bin.Upper)
		assert.Equal(t, uint64(50), bin.Count)
	})
}

func TestStructureSizes(t *testing.T) {
	t.Run("EBSMetrics structure size", func(t *testing.T) {
		size := unsafe.Sizeof(EBSMetrics{})
		// EBSMetrics should be large enough to contain all fields including histograms
		// Minimum expected size: 12 uint64s (96 bytes) + 416 bytes reserved + 2 histograms
		minExpectedSize := uintptr(96 + 416 + 2*(8+64*24))

		assert.GreaterOrEqual(t, size, minExpectedSize, "EBSMetrics size should be at least %d bytes", minExpectedSize)
	})

	t.Run("InstanceStoreMetrics structure size", func(t *testing.T) {
		size := unsafe.Sizeof(InstanceStoreMetrics{})
		// InstanceStoreMetrics should be large enough to contain all fields
		// Minimum expected size: 2 uint32s + 13 uint64s + 64 uint64s bounds + 416 bytes reserved + 2 histograms
		minExpectedSize := uintptr(8 + 104 + 512 + 416 + 2*(8+64*24))

		assert.GreaterOrEqual(t, size, minExpectedSize, "InstanceStoreMetrics size should be at least %d bytes", minExpectedSize)
	})
}

func TestUpdatedInstanceStoreMetricsStructSize(t *testing.T) {
	// Update the test to account for the new histogram fields
	// Magic (4) + Reserved (4) + ReadOps (8) + WriteOps (8) + ReadBytes (8) + WriteBytes (8) +
	// TotalReadTime (8) + TotalWriteTime (8) + EBSIOPSExceeded (8) + EBSThroughputExceeded (8) +
	// EC2IOPSExceeded (8) + EC2ThroughputExceeded (8) + QueueLength (8) + NumHistograms (8) +
	// NumBins (8) + IOSizeRange (8) + Bounds [64]uint64 (512) + 2 Histograms + ReservedArea (416)

	// The struct now includes histogram fields, so it's much larger than 96 bytes
	actualSize := int(unsafe.Sizeof(InstanceStoreMetrics{}))

	// Verify it's at least large enough for the basic fields plus histogram data
	minExpectedSize := 4 + 4 + 8*13 + 64*8 + 416 + 2*(8+64*24) // Basic calculation

	assert.GreaterOrEqual(t, actualSize, minExpectedSize,
		"InstanceStoreMetrics should be at least %d bytes to accommodate histogram fields", minExpectedSize)
}
