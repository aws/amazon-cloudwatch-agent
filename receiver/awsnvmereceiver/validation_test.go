// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsnvmereceiver

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/internal/nvme"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	"github.com/aws/amazon-cloudwatch-agent/receiver/awsnvmereceiver/internal/metadata"
)

// TestScraper_Validation_EBSMetrics tests comprehensive EBS metrics validation
func TestScraper_Validation_EBSMetrics(t *testing.T) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"*"},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	settings.Logger = zap.NewNop() // Use nop logger to avoid log output during tests
	mockNvme := &MockDeviceInfoProvider{}
	deviceSet := collections.NewSet("*")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)

	tests := []struct {
		name        string
		metrics     nvme.EBSMetrics
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid_ebs_metrics",
			metrics: nvme.EBSMetrics{
				EBSMagic:              nvme.EBSMagicNumber,
				ReadOps:               1000,
				WriteOps:              500,
				ReadBytes:             1024000,
				WriteBytes:            512000,
				TotalReadTime:         5000000,
				TotalWriteTime:        2500000,
				EBSIOPSExceeded:       10,
				EBSThroughputExceeded: 5,
				EC2IOPSExceeded:       2,
				EC2ThroughputExceeded: 1,
				QueueLength:           3,
			},
			expectError: false,
		},
		{
			name: "invalid_magic_number",
			metrics: nvme.EBSMetrics{
				EBSMagic: 0x12345678, // Invalid magic number
			},
			expectError: true,
			errorMsg:    "invalid EBS magic number",
		},
		{
			name: "zero_operations_valid",
			metrics: nvme.EBSMetrics{
				EBSMagic:   nvme.EBSMagicNumber,
				ReadOps:    0,
				WriteOps:   0,
				ReadBytes:  0,
				WriteBytes: 0,
			},
			expectError: false, // Zero operations should be valid (unused device)
		},
		{
			name: "bytes_without_operations_suspicious",
			metrics: nvme.EBSMetrics{
				EBSMagic:   nvme.EBSMagicNumber,
				ReadOps:    0,
				WriteOps:   0,
				ReadBytes:  1024000, // Bytes but no operations
				WriteBytes: 512000,
			},
			expectError: false, // Should not error, just warn
		},
		{
			name: "operations_without_bytes_suspicious",
			metrics: nvme.EBSMetrics{
				EBSMagic:   nvme.EBSMagicNumber,
				ReadOps:    1000, // Operations but no bytes
				WriteOps:   500,
				ReadBytes:  0,
				WriteBytes: 0,
			},
			expectError: false, // Should not error, just warn
		},
		{
			name: "high_average_read_time_suspicious",
			metrics: nvme.EBSMetrics{
				EBSMagic:      nvme.EBSMagicNumber,
				ReadOps:       1,
				TotalReadTime: 2e12, // 2 seconds per operation - suspicious
			},
			expectError: false, // Should not error, just warn
		},
		{
			name: "high_average_write_time_suspicious",
			metrics: nvme.EBSMetrics{
				EBSMagic:       nvme.EBSMagicNumber,
				WriteOps:       1,
				TotalWriteTime: 2e12, // 2 seconds per operation - suspicious
			},
			expectError: false, // Should not error, just warn
		},
		{
			name: "reasonable_average_times",
			metrics: nvme.EBSMetrics{
				EBSMagic:       nvme.EBSMagicNumber,
				ReadOps:        1000,
				WriteOps:       500,
				TotalReadTime:  1e9, // 1ms per operation - reasonable
				TotalWriteTime: 5e8, // 1ms per operation - reasonable
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := scraper.validateEBSMetrics(&tt.metrics, "/dev/nvme0n1")
			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestScraper_Validation_InstanceStoreMetrics tests comprehensive Instance Store metrics validation
func TestScraper_Validation_InstanceStoreMetrics(t *testing.T) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"*"},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	settings.Logger = zap.NewNop() // Use nop logger to avoid log output during tests
	mockNvme := &MockDeviceInfoProvider{}
	deviceSet := collections.NewSet("*")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)

	tests := []struct {
		name        string
		metrics     nvme.InstanceStoreMetrics
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid_instance_store_metrics",
			metrics: nvme.InstanceStoreMetrics{
				Magic:                 nvme.InstanceStoreMagicNumber,
				ReadOps:               2000,
				WriteOps:              1000,
				ReadBytes:             2048000,
				WriteBytes:            1024000,
				TotalReadTime:         10000000,
				TotalWriteTime:        5000000,
				EC2IOPSExceeded:       4,
				EC2ThroughputExceeded: 2,
				QueueLength:           5,
				NumHistograms:         2,
				NumBins:               64,
			},
			expectError: false,
		},
		{
			name: "invalid_magic_number",
			metrics: nvme.InstanceStoreMetrics{
				Magic: 0x12345678, // Invalid magic number
			},
			expectError: true,
			errorMsg:    "invalid Instance Store magic number",
		},
		{
			name: "zero_operations_valid",
			metrics: nvme.InstanceStoreMetrics{
				Magic:      nvme.InstanceStoreMagicNumber,
				ReadOps:    0,
				WriteOps:   0,
				ReadBytes:  0,
				WriteBytes: 0,
			},
			expectError: false, // Zero operations should be valid (unused device)
		},
		{
			name: "bytes_without_operations_suspicious",
			metrics: nvme.InstanceStoreMetrics{
				Magic:      nvme.InstanceStoreMagicNumber,
				ReadOps:    0,
				WriteOps:   0,
				ReadBytes:  2048000, // Bytes but no operations
				WriteBytes: 1024000,
			},
			expectError: false, // Should not error, just warn
		},
		{
			name: "operations_without_bytes_suspicious",
			metrics: nvme.InstanceStoreMetrics{
				Magic:      nvme.InstanceStoreMagicNumber,
				ReadOps:    2000, // Operations but no bytes
				WriteOps:   1000,
				ReadBytes:  0,
				WriteBytes: 0,
			},
			expectError: false, // Should not error, just warn
		},
		{
			name: "high_average_read_time_suspicious",
			metrics: nvme.InstanceStoreMetrics{
				Magic:         nvme.InstanceStoreMagicNumber,
				ReadOps:       1,
				TotalReadTime: 2e12, // 2 seconds per operation - suspicious
			},
			expectError: false, // Should not error, just warn
		},
		{
			name: "high_average_write_time_suspicious",
			metrics: nvme.InstanceStoreMetrics{
				Magic:          nvme.InstanceStoreMagicNumber,
				WriteOps:       1,
				TotalWriteTime: 2e12, // 2 seconds per operation - suspicious
			},
			expectError: false, // Should not error, just warn
		},
		{
			name: "histograms_without_bins_suspicious",
			metrics: nvme.InstanceStoreMetrics{
				Magic:         nvme.InstanceStoreMagicNumber,
				NumHistograms: 2,
				NumBins:       0, // Histograms but no bins
			},
			expectError: false, // Should not error, just warn
		},
		{
			name: "bins_without_histograms_valid",
			metrics: nvme.InstanceStoreMetrics{
				Magic:         nvme.InstanceStoreMagicNumber,
				NumHistograms: 0,
				NumBins:       64, // Bins but no histograms - could be valid
			},
			expectError: false,
		},
		{
			name: "reasonable_average_times",
			metrics: nvme.InstanceStoreMetrics{
				Magic:          nvme.InstanceStoreMagicNumber,
				ReadOps:        2000,
				WriteOps:       1000,
				TotalReadTime:  2e9, // 1ms per operation - reasonable
				TotalWriteTime: 1e9, // 1ms per operation - reasonable
			},
			expectError: false,
		},
		{
			name: "extreme_values_within_bounds",
			metrics: nvme.InstanceStoreMetrics{
				Magic:                 nvme.InstanceStoreMagicNumber,
				ReadOps:               1e11, // Large but within reasonable bounds
				WriteOps:              1e11,
				ReadBytes:             1e17, // Large but within reasonable bounds
				WriteBytes:            1e17,
				TotalReadTime:         1e17, // Large but within reasonable bounds
				TotalWriteTime:        1e17,
				EC2IOPSExceeded:       1e11,
				EC2ThroughputExceeded: 1e11,
				QueueLength:           1e5,
				NumHistograms:         9,   // Within bounds
				NumBins:               255, // Within bounds
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := scraper.validateInstanceStoreMetrics(&tt.metrics, "/dev/nvme0n1")
			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestScraper_Validation_MetricRecording tests metric recording with overflow protection
func TestScraper_Validation_MetricRecording(t *testing.T) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"*"},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	settings.Logger = zap.NewNop() // Use nop logger to avoid log output during tests
	mockNvme := &MockDeviceInfoProvider{}
	deviceSet := collections.NewSet("*")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)

	tests := []struct {
		name           string
		value          uint64
		expectRecorded bool
	}{
		{
			name:           "valid_small_value",
			value:          1000,
			expectRecorded: true,
		},
		{
			name:           "valid_large_value",
			value:          9223372036854775807, // math.MaxInt64
			expectRecorded: true,
		},
		{
			name:           "overflow_value",
			value:          9223372036854775808, // math.MaxInt64 + 1
			expectRecorded: false,
		},
		{
			name:           "max_uint64_value",
			value:          18446744073709551615, // math.MaxUint64
			expectRecorded: false,
		},
		{
			name:           "zero_value",
			value:          0,
			expectRecorded: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorded := false
			recordFn := func(ts pcommon.Timestamp, val int64) {
				recorded = true
				if tt.expectRecorded {
					assert.Equal(t, int64(tt.value), val)
				}
			}

			now := pcommon.NewTimestampFromTime(time.Now())
			scraper.recordMetric(recordFn, now, tt.value)

			assert.Equal(t, tt.expectRecorded, recorded)
		})
	}
}

// TestScraper_Validation_SafeUint64ToInt64 tests the safe conversion function
func TestScraper_Validation_SafeUint64ToInt64(t *testing.T) {
	tests := []struct {
		name        string
		value       uint64
		expectError bool
		expected    int64
	}{
		{
			name:        "valid_small_value",
			value:       1000,
			expectError: false,
			expected:    1000,
		},
		{
			name:        "valid_max_int64",
			value:       9223372036854775807, // math.MaxInt64
			expectError: false,
			expected:    9223372036854775807,
		},
		{
			name:        "overflow_max_int64_plus_one",
			value:       9223372036854775808, // math.MaxInt64 + 1
			expectError: true,
		},
		{
			name:        "overflow_max_uint64",
			value:       18446744073709551615, // math.MaxUint64
			expectError: true,
		},
		{
			name:        "zero_value",
			value:       0,
			expectError: false,
			expected:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := nvme.SafeUint64ToInt64(tt.value)
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

// TestScraper_Validation_DeviceTypeDetection tests device type detection validation
func TestScraper_Validation_DeviceTypeDetection(t *testing.T) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"*"},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	settings.Logger = zap.NewNop() // Use nop logger to avoid log output during tests
	mockNvme := &MockDeviceInfoProvider{}
	deviceSet := collections.NewSet("*")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)

	tests := []struct {
		name           string
		mockResponses  []mockResponse
		expectedResult string
		expectError    bool
	}{
		{
			name: "successful_detection_first_attempt",
			mockResponses: []mockResponse{
				{result: "ebs", err: nil},
			},
			expectedResult: "ebs",
			expectError:    false,
		},
		{
			name: "successful_detection_after_retry",
			mockResponses: []mockResponse{
				{result: "", err: errors.New("permission denied")}, // Recoverable error
				{result: "instance_store", err: nil},               // Success on retry
			},
			expectedResult: "instance_store",
			expectError:    false,
		},
		{
			name: "non_recoverable_error_no_retry",
			mockResponses: []mockResponse{
				{result: "", err: errors.New("invalid device type")}, // Non-recoverable error
			},
			expectError: true,
		},
		{
			name: "exhaust_all_retries",
			mockResponses: []mockResponse{
				{result: "", err: errors.New("permission denied")}, // Recoverable error
				{result: "", err: errors.New("permission denied")}, // Recoverable error
				{result: "", err: errors.New("permission denied")}, // Recoverable error - exhaust retries
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			device := createTestDevice(0, 1, "nvme0n1")

			// Set up mock calls based on expected responses
			call := mockNvme.On("DetectDeviceType", &device)
			for i, response := range tt.mockResponses {
				if i == len(tt.mockResponses)-1 {
					// Last call
					call = call.Return(response.result, response.err)
				} else {
					// Intermediate calls
					call = call.Return(response.result, response.err).Once()
					call = mockNvme.On("DetectDeviceType", &device)
				}
			}

			result, err := scraper.detectDeviceTypeWithRetry(&device)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}

			mockNvme.AssertExpectations(t)

			// Reset mock for next test
			mockNvme.ExpectedCalls = nil
			mockNvme.Calls = nil
		})
	}
}

// mockResponse represents a mock function response
type mockResponse struct {
	result string
	err    error
}

// TestScraper_Validation_PlatformSupport tests platform support detection
func TestScraper_Validation_PlatformSupport(t *testing.T) {
	tests := []struct {
		name            string
		mockError       error
		expectSupported bool
	}{
		{
			name:            "platform_supported",
			mockError:       nil,
			expectSupported: true,
		},
		{
			name:            "platform_not_supported_discovery_message",
			mockError:       errors.New("nvme device discovery is only supported on Linux"),
			expectSupported: false,
		},
		{
			name:            "platform_not_supported_operations_message",
			mockError:       errors.New("nvme device operations are only supported on Linux"),
			expectSupported: false,
		},
		{
			name:            "other_error_assume_supported",
			mockError:       errors.New("permission denied"),
			expectSupported: true,
		},
		{
			name:            "device_not_found_assume_supported",
			mockError:       errors.New("device not found"),
			expectSupported: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				Devices:              []string{"*"},
			}
			settings := receivertest.NewNopSettings(metadata.Type)
			mockNvme := &MockDeviceInfoProvider{}
			deviceSet := collections.NewSet("*")

			scraper := newScraper(cfg, settings, mockNvme, deviceSet)

			// Mock GetAllDevices to return the specified error
			mockNvme.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{}, tt.mockError)

			supported := scraper.isPlatformSupported()
			assert.Equal(t, tt.expectSupported, supported)

			mockNvme.AssertExpectations(t)
		})
	}
}

// TestScraper_Validation_ErrorClassification tests comprehensive error classification
func TestScraper_Validation_ErrorClassification(t *testing.T) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"*"},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	mockNvme := &MockDeviceInfoProvider{}
	deviceSet := collections.NewSet("*")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)

	tests := []struct {
		name          string
		error         error
		expectedClass string
		isRecoverable bool
	}{
		{
			name:          "nil_error",
			error:         nil,
			expectedClass: "none",
			isRecoverable: false,
		},
		{
			name:          "platform_unsupported_linux",
			error:         errors.New("only supported on Linux"),
			expectedClass: "platform_unsupported",
			isRecoverable: false,
		},
		{
			name:          "platform_unsupported_general",
			error:         errors.New("unsupported platform"),
			expectedClass: "platform_unsupported",
			isRecoverable: false,
		},
		{
			name:          "permission_denied_basic",
			error:         errors.New("permission denied"),
			expectedClass: "permission_denied",
			isRecoverable: true,
		},
		{
			name:          "permission_denied_insufficient",
			error:         errors.New("insufficient permissions"),
			expectedClass: "permission_denied",
			isRecoverable: true,
		},
		{
			name:          "permission_denied_cap_sys_admin",
			error:         errors.New("requires CAP_SYS_ADMIN capability"),
			expectedClass: "permission_denied",
			isRecoverable: true,
		},
		{
			name:          "device_not_found_basic",
			error:         errors.New("device not found"),
			expectedClass: "device_not_found",
			isRecoverable: false,
		},
		{
			name:          "device_not_found_no_such_file",
			error:         errors.New("no such file or directory"),
			expectedClass: "device_not_found",
			isRecoverable: false,
		},
		{
			name:          "device_busy",
			error:         errors.New("device or resource busy"),
			expectedClass: "device_busy",
			isRecoverable: true,
		},
		{
			name:          "ioctl_failed",
			error:         errors.New("ioctl operation failed"),
			expectedClass: "ioctl_failed",
			isRecoverable: false,
		},
		{
			name:          "invalid_magic_number",
			error:         errors.New("invalid magic number detected"),
			expectedClass: "invalid_magic_number",
			isRecoverable: false,
		},
		{
			name:          "magic_validation_error",
			error:         errors.New("magic validation failed"),
			expectedClass: "invalid_magic_number",
			isRecoverable: false,
		},
		{
			name:          "insufficient_data",
			error:         errors.New("insufficient data for parsing"),
			expectedClass: "data_parsing_error",
			isRecoverable: false,
		},
		{
			name:          "buffer_overflow",
			error:         errors.New("buffer overflow detected"),
			expectedClass: "data_parsing_error",
			isRecoverable: false,
		},
		{
			name:          "device_type_detection_failed",
			error:         errors.New("device type detection failed"),
			expectedClass: "device_type_detection_failed",
			isRecoverable: false,
		},
		{
			name:          "unknown_device_type",
			error:         errors.New("unknown device type"),
			expectedClass: "device_type_detection_failed",
			isRecoverable: false,
		},
		{
			name:          "metadata_service_error",
			error:         errors.New("metadata service unavailable"),
			expectedClass: "metadata_service_error",
			isRecoverable: true,
		},
		{
			name:          "instance_id_error",
			error:         errors.New("failed to get instance id"),
			expectedClass: "metadata_service_error",
			isRecoverable: true,
		},
		{
			name:          "io_error_basic",
			error:         errors.New("I/O error occurred"),
			expectedClass: "io_error",
			isRecoverable: true,
		},
		{
			name:          "io_error_input_output",
			error:         errors.New("input/output error"),
			expectedClass: "io_error",
			isRecoverable: true,
		},
		{
			name:          "network_error_connection_refused",
			error:         errors.New("connection refused"),
			expectedClass: "network_error",
			isRecoverable: true,
		},
		{
			name:          "network_error_timeout",
			error:         errors.New("timeout occurred"),
			expectedClass: "network_error",
			isRecoverable: true,
		},
		{
			name:          "network_error_unreachable",
			error:         errors.New("network unreachable"),
			expectedClass: "network_error",
			isRecoverable: true,
		},
		{
			name:          "overflow_error_basic",
			error:         errors.New("value overflow detected"),
			expectedClass: "overflow_error",
			isRecoverable: false,
		},
		{
			name:          "overflow_error_too_large",
			error:         errors.New("value too large for int64"),
			expectedClass: "overflow_error",
			isRecoverable: false,
		},
		{
			name:          "unknown_error",
			error:         errors.New("some completely unknown error"),
			expectedClass: "unknown_error",
			isRecoverable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			classification := scraper.classifyError(tt.error)
			assert.Equal(t, tt.expectedClass, classification)

			recoverable := scraper.isRecoverableError(tt.error)
			assert.Equal(t, tt.isRecoverable, recoverable)
		})
	}
}
