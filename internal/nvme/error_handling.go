// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nvme

import (
	"errors"
	"fmt"
	"strings"
	"syscall"
)

// Enhanced error types for better error handling and recovery
var (
	ErrPlatformUnsupported = errors.New("NVMe operations are not supported on this platform")
	ErrDeviceAccessDenied  = errors.New("device access denied - insufficient permissions")
	ErrDeviceBusy          = errors.New("device is busy or temporarily unavailable")
	ErrDeviceTimeout       = errors.New("device operation timed out")
	ErrCorruptedData       = errors.New("corrupted or invalid data detected")
	ErrMetricOverflow      = errors.New("metric value overflow detected")
	ErrInvalidDeviceState  = errors.New("device is in an invalid state")
	ErrTemporaryFailure    = errors.New("temporary failure - operation may succeed if retried")
)

// ErrorCategory represents different categories of errors for monitoring and recovery
type ErrorCategory string

const (
	ErrorCategoryPlatform   ErrorCategory = "platform"
	ErrorCategoryPermission ErrorCategory = "permission"
	ErrorCategoryDevice     ErrorCategory = "device"
	ErrorCategoryData       ErrorCategory = "data"
	ErrorCategoryNetwork    ErrorCategory = "network"
	ErrorCategoryTemporary  ErrorCategory = "temporary"
	ErrorCategoryUnknown    ErrorCategory = "unknown"
)

// ErrorInfo provides detailed information about an error for better handling
type ErrorInfo struct {
	Category    ErrorCategory
	Recoverable bool
	RetryAfter  int // seconds, 0 means no specific retry time
	Context     map[string]string
}

// ClassifyError analyzes an error and returns detailed error information
func ClassifyError(err error) ErrorInfo {
	if err == nil {
		return ErrorInfo{Category: ErrorCategoryUnknown, Recoverable: false}
	}

	errorMsg := strings.ToLower(err.Error())
	context := make(map[string]string)

	// Platform support errors
	if strings.Contains(errorMsg, "only supported on linux") ||
		strings.Contains(errorMsg, "unsupported platform") ||
		errors.Is(err, ErrPlatformUnsupported) {
		return ErrorInfo{
			Category:    ErrorCategoryPlatform,
			Recoverable: false,
			Context:     context,
		}
	}

	// Permission errors
	if strings.Contains(errorMsg, "permission denied") ||
		strings.Contains(errorMsg, "insufficient permissions") ||
		strings.Contains(errorMsg, "cap_sys_admin") ||
		errors.Is(err, ErrDeviceAccessDenied) {
		context["suggestion"] = "ensure CAP_SYS_ADMIN capability or run as root"
		return ErrorInfo{
			Category:    ErrorCategoryPermission,
			Recoverable: true,
			RetryAfter:  30, // Retry after 30 seconds in case permissions are fixed
			Context:     context,
		}
	}

	// Device access errors
	if strings.Contains(errorMsg, "device not found") ||
		strings.Contains(errorMsg, "no such file or directory") ||
		errors.Is(err, ErrDeviceNotFound) {
		return ErrorInfo{
			Category:    ErrorCategoryDevice,
			Recoverable: true,
			RetryAfter:  60, // Device might be hot-plugged
			Context:     context,
		}
	}

	if strings.Contains(errorMsg, "device or resource busy") ||
		strings.Contains(errorMsg, "busy") ||
		errors.Is(err, ErrDeviceBusy) {
		context["suggestion"] = "device may be in use by another process"
		return ErrorInfo{
			Category:    ErrorCategoryDevice,
			Recoverable: true,
			RetryAfter:  10, // Short retry for busy devices
			Context:     context,
		}
	}

	if strings.Contains(errorMsg, "timeout") ||
		errors.Is(err, ErrDeviceTimeout) {
		return ErrorInfo{
			Category:    ErrorCategoryDevice,
			Recoverable: true,
			RetryAfter:  15,
			Context:     context,
		}
	}

	// Data parsing and validation errors
	if strings.Contains(errorMsg, "invalid magic number") ||
		strings.Contains(errorMsg, "insufficient data") ||
		strings.Contains(errorMsg, "buffer overflow") ||
		strings.Contains(errorMsg, "corrupted") ||
		errors.Is(err, ErrInvalidEBSMagic) ||
		errors.Is(err, ErrInvalidInstanceStoreMagic) ||
		errors.Is(err, ErrInsufficientData) ||
		errors.Is(err, ErrBufferOverflow) ||
		errors.Is(err, ErrCorruptedData) {
		return ErrorInfo{
			Category:    ErrorCategoryData,
			Recoverable: true,
			RetryAfter:  5, // Quick retry for data issues
			Context:     context,
		}
	}

	// ioctl operation errors
	if strings.Contains(errorMsg, "ioctl") ||
		errors.Is(err, ErrIoctlFailed) {
		// Try to extract more specific information from ioctl errors
		if strings.Contains(errorMsg, "einval") {
			context["errno"] = "EINVAL"
			context["suggestion"] = "invalid ioctl parameters or unsupported operation"
		} else if strings.Contains(errorMsg, "eio") {
			context["errno"] = "EIO"
			context["suggestion"] = "I/O error - device may be failing"
		}
		return ErrorInfo{
			Category:    ErrorCategoryDevice,
			Recoverable: true,
			RetryAfter:  5,
			Context:     context,
		}
	}

	// Network-related errors (for metadata service)
	if strings.Contains(errorMsg, "connection refused") ||
		strings.Contains(errorMsg, "network unreachable") ||
		strings.Contains(errorMsg, "timeout") {
		return ErrorInfo{
			Category:    ErrorCategoryNetwork,
			Recoverable: true,
			RetryAfter:  30,
			Context:     context,
		}
	}

	// Temporary failures
	if strings.Contains(errorMsg, "temporarily unavailable") ||
		strings.Contains(errorMsg, "try again") ||
		errors.Is(err, ErrTemporaryFailure) {
		return ErrorInfo{
			Category:    ErrorCategoryTemporary,
			Recoverable: true,
			RetryAfter:  10,
			Context:     context,
		}
	}

	// Overflow errors
	if strings.Contains(errorMsg, "overflow") ||
		strings.Contains(errorMsg, "too large") ||
		errors.Is(err, ErrMetricOverflow) {
		context["suggestion"] = "metric value exceeds maximum representable value"
		return ErrorInfo{
			Category:    ErrorCategoryData,
			Recoverable: false, // Overflow is not recoverable
			Context:     context,
		}
	}

	// Default to unknown error
	return ErrorInfo{
		Category:    ErrorCategoryUnknown,
		Recoverable: false,
		Context:     context,
	}
}

// WrapError wraps an error with additional context for better debugging
func WrapError(err error, operation string, devicePath string, additionalContext map[string]string) error {
	if err == nil {
		return nil
	}

	context := make(map[string]string)
	if additionalContext != nil {
		for k, v := range additionalContext {
			context[k] = v
		}
	}
	context["operation"] = operation
	context["devicePath"] = devicePath

	return fmt.Errorf("%s failed for device %s: %w (context: %v)", operation, devicePath, err, context)
}

// IsRecoverableError determines if an error is recoverable based on its classification
func IsRecoverableError(err error) bool {
	if err == nil {
		return false
	}

	errorInfo := ClassifyError(err)
	return errorInfo.Recoverable
}

// GetRetryDelay returns the recommended retry delay for an error
func GetRetryDelay(err error) int {
	if err == nil {
		return 0
	}

	errorInfo := ClassifyError(err)
	return errorInfo.RetryAfter
}

// EnhanceIoctlError provides more detailed error information for ioctl failures
func EnhanceIoctlError(errno syscall.Errno, operation string, devicePath string) error {
	context := map[string]string{
		"errno":      errno.Error(),
		"operation":  operation,
		"devicePath": devicePath,
	}

	switch errno {
	case syscall.EACCES, syscall.EPERM:
		context["suggestion"] = "insufficient permissions - CAP_SYS_ADMIN capability required"
		return WrapError(ErrDeviceAccessDenied, operation, devicePath, context)
	case syscall.ENODEV:
		context["suggestion"] = "device does not support this operation"
		return WrapError(ErrDeviceNotFound, operation, devicePath, context)
	case syscall.EINVAL:
		context["suggestion"] = "invalid parameters or unsupported log page"
		return WrapError(ErrIoctlFailed, operation, devicePath, context)
	case syscall.EIO:
		context["suggestion"] = "I/O error - device may be failing"
		return WrapError(ErrIoctlFailed, operation, devicePath, context)
	case syscall.ENOTTY:
		context["suggestion"] = "device does not support NVMe ioctl operations"
		return WrapError(ErrIoctlFailed, operation, devicePath, context)
	case syscall.EBUSY:
		context["suggestion"] = "device is busy - try again later"
		return WrapError(ErrDeviceBusy, operation, devicePath, context)
	case syscall.ETIMEDOUT:
		context["suggestion"] = "operation timed out - device may be unresponsive"
		return WrapError(ErrDeviceTimeout, operation, devicePath, context)
	default:
		context["suggestion"] = "unknown ioctl error"
		return WrapError(ErrIoctlFailed, operation, devicePath, context)
	}
}

// ValidateMetricBounds performs comprehensive validation of metric values
func ValidateMetricBounds(metricName string, value uint64, devicePath string) error {
	// Define reasonable upper bounds for different metric types
	var maxValue uint64

	switch {
	case strings.Contains(metricName, "exceeded"):
		maxValue = 1000000000000 // 1 trillion exceeded events
	case strings.Contains(metricName, "queue"):
		maxValue = 1000000 // 1 million queue length
	case strings.Contains(metricName, "ops"):
		maxValue = 1000000000000000 // 1 quadrillion operations
	case strings.Contains(metricName, "bytes"):
		maxValue = 1000000000000000000 // 1 quintillion bytes (close to 1 zettabyte)
	case strings.Contains(metricName, "time"):
		maxValue = 1000000000000000000 // ~31 years in nanoseconds
	default:
		maxValue = 1000000000000000000 // Default large value
	}

	if value > maxValue {
		context := map[string]string{
			"metric":   metricName,
			"value":    fmt.Sprintf("%d", value),
			"maxValue": fmt.Sprintf("%d", maxValue),
		}
		return WrapError(ErrCorruptedData, "metric validation", devicePath, context)
	}

	return nil
}

// DetectDataCorruption performs heuristic checks for data corruption
func DetectDataCorruption(readOps, writeOps, readBytes, writeBytes uint64, devicePath string) error {
	// Check for impossible relationships that might indicate corruption
	if readBytes > 0 && readOps == 0 {
		context := map[string]string{
			"readBytes": fmt.Sprintf("%d", readBytes),
			"readOps":   fmt.Sprintf("%d", readOps),
			"issue":     "read bytes without read operations",
		}
		return WrapError(ErrCorruptedData, "data consistency check", devicePath, context)
	}

	if writeBytes > 0 && writeOps == 0 {
		context := map[string]string{
			"writeBytes": fmt.Sprintf("%d", writeBytes),
			"writeOps":   fmt.Sprintf("%d", writeOps),
			"issue":      "write bytes without write operations",
		}
		return WrapError(ErrCorruptedData, "data consistency check", devicePath, context)
	}

	// Check for extremely large average I/O sizes that might indicate corruption
	if readOps > 0 {
		avgReadSize := readBytes / readOps
		if avgReadSize > 100*1024*1024 { // 100MB average read size
			context := map[string]string{
				"avgReadSize": fmt.Sprintf("%d", avgReadSize),
				"issue":       "unusually large average read size",
			}
			return WrapError(ErrCorruptedData, "data consistency check", devicePath, context)
		}
	}

	if writeOps > 0 {
		avgWriteSize := writeBytes / writeOps
		if avgWriteSize > 100*1024*1024 { // 100MB average write size
			context := map[string]string{
				"avgWriteSize": fmt.Sprintf("%d", avgWriteSize),
				"issue":        "unusually large average write size",
			}
			return WrapError(ErrCorruptedData, "data consistency check", devicePath, context)
		}
	}

	return nil
}
