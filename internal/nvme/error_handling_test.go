// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nvme

import (
	"errors"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name                string
		error               error
		expectedCategory    ErrorCategory
		expectedRecoverable bool
		expectedRetryAfter  int
	}{
		{
			name:                "nil error",
			error:               nil,
			expectedCategory:    ErrorCategoryUnknown,
			expectedRecoverable: false,
			expectedRetryAfter:  0,
		},
		{
			name:                "platform unsupported",
			error:               errors.New("only supported on Linux"),
			expectedCategory:    ErrorCategoryPlatform,
			expectedRecoverable: false,
			expectedRetryAfter:  0,
		},
		{
			name:                "permission denied",
			error:               errors.New("permission denied"),
			expectedCategory:    ErrorCategoryPermission,
			expectedRecoverable: true,
			expectedRetryAfter:  30,
		},
		{
			name:                "device not found",
			error:               errors.New("device not found"),
			expectedCategory:    ErrorCategoryDevice,
			expectedRecoverable: true,
			expectedRetryAfter:  60,
		},
		{
			name:                "device busy",
			error:               errors.New("device or resource busy"),
			expectedCategory:    ErrorCategoryDevice,
			expectedRecoverable: true,
			expectedRetryAfter:  10,
		},
		{
			name:                "timeout error",
			error:               errors.New("timeout"),
			expectedCategory:    ErrorCategoryDevice,
			expectedRecoverable: true,
			expectedRetryAfter:  15,
		},
		{
			name:                "invalid magic number",
			error:               errors.New("invalid magic number"),
			expectedCategory:    ErrorCategoryData,
			expectedRecoverable: true,
			expectedRetryAfter:  5,
		},
		{
			name:                "ioctl failed",
			error:               errors.New("ioctl operation failed"),
			expectedCategory:    ErrorCategoryDevice,
			expectedRecoverable: true,
			expectedRetryAfter:  5,
		},
		{
			name:                "network error",
			error:               errors.New("connection refused"),
			expectedCategory:    ErrorCategoryNetwork,
			expectedRecoverable: true,
			expectedRetryAfter:  30,
		},
		{
			name:                "temporary failure",
			error:               errors.New("temporarily unavailable"),
			expectedCategory:    ErrorCategoryTemporary,
			expectedRecoverable: true,
			expectedRetryAfter:  10,
		},
		{
			name:                "overflow error",
			error:               errors.New("value too large for int64"),
			expectedCategory:    ErrorCategoryData,
			expectedRecoverable: false,
			expectedRetryAfter:  0,
		},
		{
			name:                "unknown error",
			error:               errors.New("some random error"),
			expectedCategory:    ErrorCategoryUnknown,
			expectedRecoverable: false,
			expectedRetryAfter:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyError(tt.error)
			assert.Equal(t, tt.expectedCategory, result.Category)
			assert.Equal(t, tt.expectedRecoverable, result.Recoverable)
			assert.Equal(t, tt.expectedRetryAfter, result.RetryAfter)
		})
	}
}

func TestWrapError(t *testing.T) {
	originalErr := errors.New("original error")
	operation := "test operation"
	devicePath := "/dev/nvme0n1"
	context := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	wrappedErr := WrapError(originalErr, operation, devicePath, context)

	assert.Error(t, wrappedErr)
	assert.Contains(t, wrappedErr.Error(), operation)
	assert.Contains(t, wrappedErr.Error(), devicePath)
	assert.Contains(t, wrappedErr.Error(), "original error")
	assert.Contains(t, wrappedErr.Error(), "key1")
	assert.Contains(t, wrappedErr.Error(), "value1")
}

func TestWrapError_NilError(t *testing.T) {
	result := WrapError(nil, "operation", "/dev/nvme0n1", nil)
	assert.NoError(t, result)
}

func TestIsRecoverableError(t *testing.T) {
	tests := []struct {
		name     string
		error    error
		expected bool
	}{
		{
			name:     "nil error",
			error:    nil,
			expected: false,
		},
		{
			name:     "recoverable error",
			error:    errors.New("permission denied"),
			expected: true,
		},
		{
			name:     "non-recoverable error",
			error:    errors.New("only supported on Linux"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRecoverableError(tt.error)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetRetryDelay(t *testing.T) {
	tests := []struct {
		name     string
		error    error
		expected int
	}{
		{
			name:     "nil error",
			error:    nil,
			expected: 0,
		},
		{
			name:     "permission error",
			error:    errors.New("permission denied"),
			expected: 30,
		},
		{
			name:     "device busy error",
			error:    errors.New("device or resource busy"),
			expected: 10,
		},
		{
			name:     "non-recoverable error",
			error:    errors.New("only supported on Linux"),
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetRetryDelay(tt.error)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEnhanceIoctlError(t *testing.T) {
	operation := "read log page"
	devicePath := "/dev/nvme0n1"

	tests := []struct {
		name          string
		errno         syscall.Errno
		expectedError error
		expectedMsg   string
	}{
		{
			name:          "EACCES",
			errno:         syscall.EACCES,
			expectedError: ErrDeviceAccessDenied,
			expectedMsg:   "insufficient permissions",
		},
		{
			name:          "EPERM",
			errno:         syscall.EPERM,
			expectedError: ErrDeviceAccessDenied,
			expectedMsg:   "insufficient permissions",
		},
		{
			name:          "ENODEV",
			errno:         syscall.ENODEV,
			expectedError: ErrDeviceNotFound,
			expectedMsg:   "device does not support",
		},
		{
			name:          "EINVAL",
			errno:         syscall.EINVAL,
			expectedError: ErrIoctlFailed,
			expectedMsg:   "invalid parameters",
		},
		{
			name:          "EIO",
			errno:         syscall.EIO,
			expectedError: ErrIoctlFailed,
			expectedMsg:   "I/O error",
		},
		{
			name:          "ENOTTY",
			errno:         syscall.ENOTTY,
			expectedError: ErrIoctlFailed,
			expectedMsg:   "does not support NVMe ioctl",
		},
		{
			name:          "EBUSY",
			errno:         syscall.EBUSY,
			expectedError: ErrDeviceBusy,
			expectedMsg:   "device is busy",
		},
		{
			name:          "ETIMEDOUT",
			errno:         syscall.ETIMEDOUT,
			expectedError: ErrDeviceTimeout,
			expectedMsg:   "operation timed out",
		},
		{
			name:          "unknown errno",
			errno:         syscall.EAGAIN,
			expectedError: ErrIoctlFailed,
			expectedMsg:   "unknown ioctl error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EnhanceIoctlError(tt.errno, operation, devicePath)

			assert.Error(t, result)
			assert.Contains(t, result.Error(), operation)
			assert.Contains(t, result.Error(), devicePath)
			assert.Contains(t, result.Error(), tt.expectedMsg)
			assert.True(t, errors.Is(result, tt.expectedError))
		})
	}
}

func TestValidateMetricBounds(t *testing.T) {
	devicePath := "/dev/nvme0n1"

	tests := []struct {
		name        string
		metricName  string
		value       uint64
		expectError bool
	}{
		{
			name:        "valid ops metric",
			metricName:  "total_read_ops",
			value:       1000000,
			expectError: false,
		},
		{
			name:        "invalid ops metric - too large",
			metricName:  "total_read_ops",
			value:       2000000000000000, // Exceeds 1e15 limit
			expectError: true,
		},
		{
			name:        "valid bytes metric",
			metricName:  "total_read_bytes",
			value:       1000000000000, // 1TB
			expectError: false,
		},
		{
			name:        "invalid bytes metric - too large",
			metricName:  "total_read_bytes",
			value:       2000000000000000000, // Exceeds limit
			expectError: true,
		},
		{
			name:        "valid time metric",
			metricName:  "total_read_time",
			value:       1000000000000000, // ~11 days in nanoseconds
			expectError: false,
		},
		{
			name:        "invalid time metric - too large",
			metricName:  "total_read_time",
			value:       2000000000000000000, // Exceeds limit
			expectError: true,
		},
		{
			name:        "valid exceeded metric",
			metricName:  "volume_performance_exceeded_iops",
			value:       1000,
			expectError: false,
		},
		{
			name:        "invalid exceeded metric - too large",
			metricName:  "volume_performance_exceeded_iops",
			value:       2000000000000, // 2 trillion, exceeds 1 trillion limit
			expectError: true,
		},
		{
			name:        "valid queue metric",
			metricName:  "volume_queue_length",
			value:       100,
			expectError: false,
		},
		{
			name:        "invalid queue metric - too large",
			metricName:  "volume_queue_length",
			value:       2000000, // Exceeds 1e6 limit
			expectError: true,
		},
		{
			name:        "valid unknown metric",
			metricName:  "unknown_metric",
			value:       100000000000000000,
			expectError: false,
		},
		{
			name:        "invalid unknown metric - too large",
			metricName:  "unknown_metric",
			value:       2000000000000000000, // Exceeds default limit
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMetricBounds(tt.metricName, tt.value, devicePath)

			if tt.expectError {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, ErrCorruptedData))
				assert.Contains(t, err.Error(), tt.metricName)
				assert.Contains(t, err.Error(), devicePath)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDetectDataCorruption(t *testing.T) {
	devicePath := "/dev/nvme0n1"

	tests := []struct {
		name        string
		readOps     uint64
		writeOps    uint64
		readBytes   uint64
		writeBytes  uint64
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid data - no operations",
			readOps:     0,
			writeOps:    0,
			readBytes:   0,
			writeBytes:  0,
			expectError: false,
		},
		{
			name:        "valid data - normal operations",
			readOps:     1000,
			writeOps:    500,
			readBytes:   1024000,
			writeBytes:  512000,
			expectError: false,
		},
		{
			name:        "corruption - read bytes without read ops",
			readOps:     0,
			writeOps:    500,
			readBytes:   1024000,
			writeBytes:  512000,
			expectError: true,
			errorMsg:    "read bytes without read operations",
		},
		{
			name:        "corruption - write bytes without write ops",
			readOps:     1000,
			writeOps:    0,
			readBytes:   1024000,
			writeBytes:  512000,
			expectError: true,
			errorMsg:    "write bytes without write operations",
		},
		{
			name:        "corruption - extremely large average read size",
			readOps:     1,
			writeOps:    1,
			readBytes:   200 * 1024 * 1024, // 200MB for 1 operation
			writeBytes:  1024,
			expectError: true,
			errorMsg:    "unusually large average read size",
		},
		{
			name:        "corruption - extremely large average write size",
			readOps:     1,
			writeOps:    1,
			readBytes:   1024,
			writeBytes:  200 * 1024 * 1024, // 200MB for 1 operation
			expectError: true,
			errorMsg:    "unusually large average write size",
		},
		{
			name:        "valid - reasonable average sizes",
			readOps:     1000,
			writeOps:    500,
			readBytes:   50 * 1024 * 1024, // 50KB average
			writeBytes:  25 * 1024 * 1024, // 50KB average
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := DetectDataCorruption(tt.readOps, tt.writeOps, tt.readBytes, tt.writeBytes, devicePath)

			if tt.expectError {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, ErrCorruptedData))
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Contains(t, err.Error(), devicePath)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestErrorConstants(t *testing.T) {
	// Test that all error constants are properly defined
	errors := []error{
		ErrPlatformUnsupported,
		ErrDeviceAccessDenied,
		ErrDeviceBusy,
		ErrDeviceTimeout,
		ErrCorruptedData,
		ErrMetricOverflow,
		ErrInvalidDeviceState,
		ErrTemporaryFailure,
	}

	for _, err := range errors {
		assert.NotNil(t, err, "error should not be nil")
		assert.NotEmpty(t, err.Error(), "error should have a message")
	}
}

func TestErrorCategories(t *testing.T) {
	// Test that all error categories are properly defined
	categories := []ErrorCategory{
		ErrorCategoryPlatform,
		ErrorCategoryPermission,
		ErrorCategoryDevice,
		ErrorCategoryData,
		ErrorCategoryNetwork,
		ErrorCategoryTemporary,
		ErrorCategoryUnknown,
	}

	for _, category := range categories {
		assert.NotEmpty(t, string(category), "category should not be empty")
	}
}

func TestErrorInfo(t *testing.T) {
	// Test ErrorInfo structure
	info := ErrorInfo{
		Category:    ErrorCategoryDevice,
		Recoverable: true,
		RetryAfter:  30,
		Context:     map[string]string{"key": "value"},
	}

	assert.Equal(t, ErrorCategoryDevice, info.Category)
	assert.True(t, info.Recoverable)
	assert.Equal(t, 30, info.RetryAfter)
	assert.Equal(t, "value", info.Context["key"])
}
