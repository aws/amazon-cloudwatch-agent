// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsnvmereceiver

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/internal/nvme"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	"github.com/aws/amazon-cloudwatch-agent/receiver/awsnvmereceiver/internal/metadata"
)

func TestScraper_ErrorHandling_PlatformSupport(t *testing.T) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"*"},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	mockNvme := &MockDeviceInfoProvider{}
	deviceSet := collections.NewSet("*")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)

	// Mock platform not supported error
	mockNvme.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{},
		errors.New("nvme device discovery is only supported on Linux"))

	ctx := context.Background()
	metrics, err := scraper.scrape(ctx)

	assert.NoError(t, err) // Should not fail, just return empty metrics
	assert.NotNil(t, metrics)
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())

	mockNvme.AssertExpectations(t)
}

func TestScraper_ErrorHandling_RecoverableErrors(t *testing.T) {
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
		expectRecover bool
	}{
		{
			name:          "permission denied error",
			error:         errors.New("permission denied"),
			expectRecover: true,
		},
		{
			name:          "device busy error",
			error:         errors.New("device or resource busy"),
			expectRecover: true,
		},
		{
			name:          "I/O error",
			error:         errors.New("I/O error"),
			expectRecover: true,
		},
		{
			name:          "timeout error",
			error:         errors.New("timeout"),
			expectRecover: true,
		},
		{
			name:          "non-recoverable error",
			error:         errors.New("invalid device format"),
			expectRecover: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scraper.isRecoverableError(tt.error)
			assert.Equal(t, tt.expectRecover, result)
		})
	}
}

func TestScraper_ErrorHandling_ErrorClassification(t *testing.T) {
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
	}{
		{
			name:          "platform unsupported",
			error:         errors.New("only supported on Linux"),
			expectedClass: "platform_unsupported",
		},
		{
			name:          "permission denied",
			error:         errors.New("permission denied"),
			expectedClass: "permission_denied",
		},
		{
			name:          "device not found",
			error:         errors.New("device not found"),
			expectedClass: "device_not_found",
		},
		{
			name:          "device busy",
			error:         errors.New("device or resource busy"),
			expectedClass: "device_busy",
		},
		{
			name:          "ioctl failed",
			error:         errors.New("ioctl operation failed"),
			expectedClass: "ioctl_failed",
		},
		{
			name:          "invalid magic number",
			error:         errors.New("invalid magic number"),
			expectedClass: "invalid_magic_number",
		},
		{
			name:          "insufficient data",
			error:         errors.New("insufficient data"),
			expectedClass: "data_parsing_error",
		},
		{
			name:          "device type detection failed",
			error:         errors.New("unknown device type"),
			expectedClass: "device_type_detection_failed",
		},
		{
			name:          "metadata service error",
			error:         errors.New("metadata service unavailable"),
			expectedClass: "metadata_service_error",
		},
		{
			name:          "I/O error",
			error:         errors.New("I/O error"),
			expectedClass: "io_error",
		},
		{
			name:          "network error",
			error:         errors.New("connection refused"),
			expectedClass: "network_error",
		},
		{
			name:          "overflow error",
			error:         errors.New("value too large for int64"),
			expectedClass: "overflow_error",
		},
		{
			name:          "unknown error",
			error:         errors.New("some random error"),
			expectedClass: "unknown_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scraper.classifyError(tt.error)
			assert.Equal(t, tt.expectedClass, result)
		})
	}
}

func TestScraper_ErrorHandling_DeviceTypeDetectionFailure(t *testing.T) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"*"},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	mockNvme := &MockDeviceInfoProvider{}
	deviceSet := collections.NewSet("*")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)

	// Create test device
	device := createTestDevice(0, 1, "nvme0n1")
	devices := []nvme.DeviceFileAttributes{device}

	// Mock device discovery with detection error
	mockNvme.On("GetAllDevices").Return(devices, nil)
	mockNvme.On("DetectDeviceType", &device).Return("", errors.New("device type detection failed"))

	devicesByController, err := scraper.getDevicesByController()

	assert.Error(t, err) // Should fail when no devices can be processed due to errors
	assert.Contains(t, err.Error(), "no devices found, encountered 1 errors during discovery")
	assert.Equal(t, 0, len(devicesByController)) // No devices should be discovered

	mockNvme.AssertExpectations(t)
}

func TestScraper_ErrorHandling_SerialNumberFallback(t *testing.T) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"*"},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	mockNvme := &MockDeviceInfoProvider{}
	deviceSet := collections.NewSet("*")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)

	// Create test device
	device := createTestDevice(0, 1, "nvme0n1")
	devices := []nvme.DeviceFileAttributes{device}

	// Mock device discovery with serial number error
	mockNvme.On("GetAllDevices").Return(devices, nil)
	mockNvme.On("DetectDeviceType", &device).Return("ebs", nil)
	mockNvme.On("GetDeviceSerial", &device).Return("", errors.New("serial number unavailable"))

	devicesByController, err := scraper.getDevicesByController()

	assert.NoError(t, err)
	assert.Equal(t, 1, len(devicesByController))
	assert.Contains(t, devicesByController[0].serialNumber, "unknown-ebs-controller-0")

	mockNvme.AssertExpectations(t)
}

func TestScraper_ErrorHandling_EBSSerialFormatting(t *testing.T) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"*"},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	mockNvme := &MockDeviceInfoProvider{}
	deviceSet := collections.NewSet("*")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)

	tests := []struct {
		name           string
		inputSerial    string
		expectedSerial string
	}{
		{
			name:           "valid EBS volume ID",
			inputSerial:    "vol123456789",
			expectedSerial: "vol-123456789",
		},
		{
			name:           "invalid EBS volume ID format",
			inputSerial:    "invalid-serial",
			expectedSerial: "invalid-serial",
		},
		{
			name:           "empty serial",
			inputSerial:    "",
			expectedSerial: "",
		},
		{
			name:           "vol prefix only",
			inputSerial:    "vol",
			expectedSerial: "vol",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scraper.formatEBSSerial(tt.inputSerial, "nvme0n1")
			assert.Equal(t, tt.expectedSerial, result)
		})
	}
}

func TestScraper_ErrorHandling_MetricsValidation(t *testing.T) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"*"},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	settings.Logger = zap.NewNop() // Use nop logger to avoid log output during tests
	mockNvme := &MockDeviceInfoProvider{}
	deviceSet := collections.NewSet("*")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)

	t.Run("EBS metrics validation", func(t *testing.T) {
		// Test with suspicious metrics (bytes without operations)
		metrics := &nvme.EBSMetrics{
			ReadOps:    0,
			WriteOps:   0,
			ReadBytes:  1024, // Bytes without operations
			WriteBytes: 2048, // Bytes without operations
		}

		err := scraper.validateEBSMetrics(metrics, "/dev/nvme0n1")
		assert.NoError(t, err) // Should not fail, just log warnings
	})

	t.Run("Instance Store metrics validation", func(t *testing.T) {
		// Test with suspicious metrics (bytes without operations)
		metrics := &nvme.InstanceStoreMetrics{
			ReadOps:    0,
			WriteOps:   0,
			ReadBytes:  1024, // Bytes without operations
			WriteBytes: 2048, // Bytes without operations
		}

		err := scraper.validateInstanceStoreMetrics(metrics, "/dev/nvme1n1")
		assert.NoError(t, err) // Should not fail, just log warnings
	})
}

func TestScraper_ErrorHandling_InstanceIDFallback(t *testing.T) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"*"},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	mockNvme := &MockDeviceInfoProvider{}
	mockMetadata := &MockMetadataProvider{}
	deviceSet := collections.NewSet("*")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)
	scraper.setMetadataProvider(mockMetadata)

	// Mock metadata service failure
	mockMetadata.On("InstanceID", mock.Anything).Return("", errors.New("metadata service unavailable"))

	ctx := context.Background()
	instanceID, err := scraper.getInstanceIDWithFallback(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "metadata service unavailable")
	assert.Contains(t, instanceID, "unknown") // Should provide fallback

	mockMetadata.AssertExpectations(t)
}

func TestFactory_ErrorHandling_PlatformSupport(t *testing.T) {
	logger := zap.NewNop()

	t.Run("platform supported", func(t *testing.T) {
		mockNvme := &MockDeviceInfoProvider{}
		mockNvme.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{}, nil)

		result := isPlatformSupported(mockNvme, logger)
		assert.True(t, result)

		mockNvme.AssertExpectations(t)
	})

	t.Run("platform not supported", func(t *testing.T) {
		mockNvme := &MockDeviceInfoProvider{}
		mockNvme.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{},
			errors.New("nvme device discovery is only supported on Linux"))

		result := isPlatformSupported(mockNvme, logger)
		assert.False(t, result)

		mockNvme.AssertExpectations(t)
	})

	t.Run("other error - assume supported", func(t *testing.T) {
		mockNvme := &MockDeviceInfoProvider{}
		mockNvme.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{},
			errors.New("permission denied"))

		result := isPlatformSupported(mockNvme, logger)
		assert.True(t, result) // Should assume supported for non-platform errors

		mockNvme.AssertExpectations(t)
	})
}

func TestNoOpReceiver(t *testing.T) {
	settings := receivertest.NewNopSettings(metadata.Type)

	receiver, err := newNoOpReceiver(settings, nil)
	assert.NoError(t, err)
	assert.NotNil(t, receiver)

	ctx := context.Background()

	// Test start
	err = receiver.Start(ctx, nil)
	assert.NoError(t, err)

	// Test shutdown
	err = receiver.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestScraper_ErrorHandling_OverflowDetection(t *testing.T) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"*"},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	settings.Logger = zap.NewNop() // Use nop logger to avoid log output during tests
	mockNvme := &MockDeviceInfoProvider{}
	deviceSet := collections.NewSet("*")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)

	// Test overflow detection in recordMetricWithContext
	overflowValue := uint64(18446744073709551615) // max uint64
	called := false

	scraper.recordMetricWithContext("test_metric", func(ts pcommon.Timestamp, val int64) {
		called = true
		t.Error("Should not call record function for overflow value")
	}, pcommon.NewTimestampFromTime(time.Now()), overflowValue, "/dev/nvme0n1")

	assert.False(t, called, "Should not call record function for overflow value")
}

func TestScraper_ErrorHandling_DevicePathErrors(t *testing.T) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"*"},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	mockNvme := &MockDeviceInfoProvider{}
	mockMetadata := &MockMetadataProvider{}
	deviceSet := collections.NewSet("*")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)
	scraper.setMetadataProvider(mockMetadata)

	// Create test device
	device := createTestDevice(0, 1, "nvme0n1")
	devices := []nvme.DeviceFileAttributes{device}

	// Mock device discovery with device path error
	mockNvme.On("GetAllDevices").Return(devices, nil)
	mockNvme.On("DetectDeviceType", &device).Return("ebs", nil)
	mockNvme.On("GetDeviceSerial", &device).Return("vol123456789", nil)
	mockNvme.On("DevicePath", "nvme0n1").Return("", errors.New("device path error"))

	// Mock metadata provider
	mockMetadata.On("InstanceID", mock.Anything).Return("i-1234567890abcdef0", nil)

	ctx := context.Background()
	metrics, err := scraper.scrape(ctx)

	assert.NoError(t, err) // Should not fail, just skip the device
	assert.NotNil(t, metrics)
	assert.Equal(t, 0, metrics.ResourceMetrics().Len()) // No metrics should be emitted

	mockNvme.AssertExpectations(t)
	mockMetadata.AssertExpectations(t)
}
