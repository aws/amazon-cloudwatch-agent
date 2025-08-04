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

	"github.com/aws/amazon-cloudwatch-agent/internal/nvme"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	"github.com/aws/amazon-cloudwatch-agent/receiver/awsnvmereceiver/internal/metadata"
)

func TestScraper_RetryLogic_DeviceTypeDetection(t *testing.T) {
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

	t.Run("successful retry after recoverable error", func(t *testing.T) {
		// Mock device type detection to fail twice with recoverable error, then succeed
		mockNvme.On("DetectDeviceType", &device).Return("", errors.New("device or resource busy")).Once()
		mockNvme.On("DetectDeviceType", &device).Return("", errors.New("device or resource busy")).Once()
		mockNvme.On("DetectDeviceType", &device).Return("ebs", nil).Once()

		start := time.Now()
		deviceType, err := scraper.detectDeviceTypeWithRetry(&device)
		duration := time.Since(start)

		assert.NoError(t, err)
		assert.Equal(t, "ebs", deviceType)
		// Should have taken some time due to retries (at least 300ms for 2 retries)
		assert.Greater(t, duration, 200*time.Millisecond)

		mockNvme.AssertExpectations(t)
		// Reset mock for next test
		mockNvme.ExpectedCalls = nil
	})

	t.Run("fail fast on non-recoverable error", func(t *testing.T) {
		// Mock device type detection to fail with non-recoverable error
		mockNvme.On("DetectDeviceType", &device).Return("", errors.New("only supported on Linux")).Once()

		start := time.Now()
		deviceType, err := scraper.detectDeviceTypeWithRetry(&device)
		duration := time.Since(start)

		assert.Error(t, err)
		assert.Empty(t, deviceType)
		assert.Contains(t, err.Error(), "device type detection failed")
		// Should fail fast without retries (less than 50ms)
		assert.Less(t, duration, 50*time.Millisecond)

		mockNvme.AssertExpectations(t)
		// Reset mock for next test
		mockNvme.ExpectedCalls = nil
	})

	t.Run("exhaust all retries", func(t *testing.T) {
		// Mock device type detection to always fail with recoverable error
		mockNvme.On("DetectDeviceType", &device).Return("", errors.New("permission denied")).Times(3)

		start := time.Now()
		deviceType, err := scraper.detectDeviceTypeWithRetry(&device)
		duration := time.Since(start)

		assert.Error(t, err)
		assert.Empty(t, deviceType)
		assert.Contains(t, err.Error(), "after 3 attempts")
		// Should have taken time for all retries (at least 300ms for 3 attempts)
		assert.Greater(t, duration, 200*time.Millisecond)

		mockNvme.AssertExpectations(t)
		// Reset mock for next test
		mockNvme.ExpectedCalls = nil
	})
}

func TestScraper_RetryLogic_MetricsRetrieval(t *testing.T) {
	// Note: This test focuses on the retry logic structure rather than actual metrics retrieval
	// since the actual GetEBSMetrics and GetInstanceStoreMetrics functions are platform-specific
	// and will return platform errors on non-Linux systems during testing.

	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"*"},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	mockNvme := &MockDeviceInfoProvider{}
	deviceSet := collections.NewSet("*")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)

	t.Run("retry logic structure validation", func(t *testing.T) {
		// Test that the retry logic is properly structured by checking error classification
		recoverableErrors := []error{
			errors.New("permission denied"),
			errors.New("device or resource busy"),
			errors.New("I/O error"),
			errors.New("timeout"),
		}

		nonRecoverableErrors := []error{
			errors.New("only supported on Linux"),
			errors.New("invalid device format"),
		}

		for _, err := range recoverableErrors {
			assert.True(t, scraper.isRecoverableError(err), "Error should be recoverable: %v", err)
		}

		for _, err := range nonRecoverableErrors {
			assert.False(t, scraper.isRecoverableError(err), "Error should not be recoverable: %v", err)
		}
	})
}

func TestScraper_ErrorRecovery_GracefulDegradation(t *testing.T) {
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

	t.Run("partial device failure - continue with working devices", func(t *testing.T) {
		// Create two test devices
		device1 := createTestDevice(0, 1, "nvme0n1")
		device2 := createTestDevice(1, 1, "nvme1n1")
		devices := []nvme.DeviceFileAttributes{device1, device2}

		// Mock device discovery
		mockNvme.On("GetAllDevices").Return(devices, nil)

		// First device fails type detection
		mockNvme.On("DetectDeviceType", &device1).Return("", errors.New("unknown device type"))

		// Second device succeeds
		mockNvme.On("DetectDeviceType", &device2).Return("ebs", nil)
		mockNvme.On("GetDeviceSerial", &device2).Return("vol-67890", nil)

		devicesByController, err := scraper.getDevicesByController()

		assert.NoError(t, err)
		assert.Equal(t, 1, len(devicesByController)) // Only one device should be discovered
		assert.Equal(t, "ebs", devicesByController[1].deviceType)
		assert.Equal(t, "vol--67890", devicesByController[1].serialNumber) // EBS serial formatting adds vol- prefix

		mockNvme.AssertExpectations(t)
	})

	t.Run("metadata service failure - use fallback", func(t *testing.T) {
		// Mock metadata service failure
		mockMetadata.On("InstanceID", mock.Anything).Return("", errors.New("metadata service unavailable"))

		ctx := context.Background()
		instanceID, err := scraper.getInstanceIDWithFallback(ctx)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "metadata service unavailable")
		assert.Contains(t, instanceID, "unknown") // Should provide fallback

		mockMetadata.AssertExpectations(t)
	})
}

func TestScraper_ErrorRecovery_MetricsValidation(t *testing.T) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"*"},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	mockNvme := &MockDeviceInfoProvider{}
	deviceSet := collections.NewSet("*")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)

	t.Run("continue processing despite validation warnings", func(t *testing.T) {
		// Create metrics with suspicious values (bytes without operations)
		suspiciousMetrics := &nvme.EBSMetrics{
			EBSMagic:   0x3C23B510,
			ReadOps:    0,    // No operations
			WriteOps:   0,    // No operations
			ReadBytes:  1024, // But has bytes
			WriteBytes: 2048, // But has bytes
		}

		// Validation should not fail, just log warnings
		err := scraper.validateEBSMetrics(suspiciousMetrics, "/dev/nvme0n1")
		assert.NoError(t, err) // Should not fail, just log warnings
	})

	t.Run("overflow detection in metric recording", func(t *testing.T) {
		recordCalled := false
		mockRecordFn := func(ts pcommon.Timestamp, val int64) {
			recordCalled = true
		}

		// Test with overflow value
		overflowValue := uint64(18446744073709551615) // max uint64
		scraper.recordMetricWithContext("test_metric", mockRecordFn, pcommon.NewTimestampFromTime(time.Now()), overflowValue, "/dev/nvme0n1")

		assert.False(t, recordCalled, "Should not record metric with overflow value")

		// Test with valid value
		recordCalled = false
		validValue := uint64(1000)
		scraper.recordMetricWithContext("test_metric", mockRecordFn, pcommon.NewTimestampFromTime(time.Now()), validValue, "/dev/nvme0n1")

		assert.True(t, recordCalled, "Should record metric with valid value")
	})
}
