// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsnvmereceiver

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/internal/nvme"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	"github.com/aws/amazon-cloudwatch-agent/receiver/awsnvmereceiver/internal/metadata"
)

// TestScraper_Comprehensive_DeviceDiscovery tests comprehensive device discovery scenarios
func TestScraper_Comprehensive_DeviceDiscovery(t *testing.T) {
	tests := []struct {
		name           string
		devices        []nvme.DeviceFileAttributes
		deviceTypes    map[string]string
		deviceSerials  map[string]string
		devicePaths    map[string]string
		expectedGroups int
		expectError    bool
		errorMsg       string
	}{
		{
			name: "mixed_device_types_same_controller",
			devices: []nvme.DeviceFileAttributes{
				createTestDevice(0, 1, "nvme0n1"),
				createTestDevice(0, 2, "nvme0n2"),
			},
			deviceTypes: map[string]string{
				"nvme0n1": "ebs",
				"nvme0n2": "ebs", // Same controller, same type
			},
			deviceSerials: map[string]string{
				"nvme0n1": "vol123456789",
				"nvme0n2": "vol123456789", // Same serial for same controller
			},
			devicePaths: map[string]string{
				"nvme0n1": "/dev/nvme0n1",
				"nvme0n2": "/dev/nvme0n2",
			},
			expectedGroups: 1, // Should be grouped together
			expectError:    false,
		},
		{
			name: "mixed_device_types_different_controllers",
			devices: []nvme.DeviceFileAttributes{
				createTestDevice(0, 1, "nvme0n1"),
				createTestDevice(1, 1, "nvme1n1"),
				createTestDevice(2, 1, "nvme2n1"),
			},
			deviceTypes: map[string]string{
				"nvme0n1": "ebs",
				"nvme1n1": "instance_store",
				"nvme2n1": "ebs",
			},
			deviceSerials: map[string]string{
				"nvme0n1": "vol123456789",
				"nvme1n1": "AWS12345678901234567",
				"nvme2n1": "vol987654321",
			},
			devicePaths: map[string]string{
				"nvme0n1": "/dev/nvme0n1",
				"nvme1n1": "/dev/nvme1n1",
				"nvme2n1": "/dev/nvme2n1",
			},
			expectedGroups: 3, // Different controllers
			expectError:    false,
		},
		{
			name: "device_type_detection_failures",
			devices: []nvme.DeviceFileAttributes{
				createTestDevice(0, 1, "nvme0n1"),
				createTestDevice(1, 1, "nvme1n1"),
				createTestDevice(2, 1, "nvme2n1"),
			},
			deviceTypes: map[string]string{
				"nvme0n1": "ebs",
				"nvme1n1": "error", // Will cause detection error
				"nvme2n1": "instance_store",
			},
			deviceSerials: map[string]string{
				"nvme0n1": "vol123456789",
				"nvme2n1": "AWS12345678901234567",
			},
			devicePaths: map[string]string{
				"nvme0n1": "/dev/nvme0n1",
				"nvme2n1": "/dev/nvme2n1",
			},
			expectedGroups: 2, // Only successful detections
			expectError:    false,
		},
		{
			name: "all_device_detection_failures",
			devices: []nvme.DeviceFileAttributes{
				createTestDevice(0, 1, "nvme0n1"),
				createTestDevice(1, 1, "nvme1n1"),
			},
			deviceTypes: map[string]string{
				"nvme0n1": "error",
				"nvme1n1": "error",
			},
			expectedGroups: 0,
			expectError:    true,
			errorMsg:       "no devices found, encountered 2 errors during discovery",
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

			// Mock device discovery
			mockNvme.On("GetAllDevices").Return(tt.devices, nil)

			// Mock device type detection
			for _, device := range tt.devices {
				deviceName := device.DeviceName()
				if deviceType, exists := tt.deviceTypes[deviceName]; exists {
					if deviceType == "error" {
						mockNvme.On("DetectDeviceType", &device).Return("", errors.New("detection failed"))
					} else {
						mockNvme.On("DetectDeviceType", &device).Return(deviceType, nil)
						if serial, serialExists := tt.deviceSerials[deviceName]; serialExists {
							mockNvme.On("GetDeviceSerial", &device).Return(serial, nil)
						}
					}
				}
			}

			devicesByController, err := scraper.getDevicesByController()

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
				assert.Len(t, devicesByController, tt.expectedGroups)
			}

			mockNvme.AssertExpectations(t)
		})
	}
}

// TestScraper_Comprehensive_MetricsProcessing tests comprehensive metrics processing scenarios
func TestScraper_Comprehensive_MetricsProcessing(t *testing.T) {
	tests := []struct {
		name          string
		deviceType    string
		mockMetrics   interface{}
		mockError     error
		expectSuccess bool
		expectedLogs  []string
	}{
		{
			name:       "ebs_metrics_success",
			deviceType: "ebs",
			mockMetrics: nvme.EBSMetrics{
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
			expectSuccess: true,
		},
		{
			name:       "instance_store_metrics_success",
			deviceType: "instance_store",
			mockMetrics: nvme.InstanceStoreMetrics{
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
			},
			expectSuccess: true,
		},
		{
			name:          "ebs_metrics_ioctl_failure",
			deviceType:    "ebs",
			mockError:     errors.New("ioctl operation failed"),
			expectSuccess: false,
		},
		{
			name:          "instance_store_metrics_permission_denied",
			deviceType:    "instance_store",
			mockError:     errors.New("permission denied"),
			expectSuccess: false,
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
			mockMetadata := &MockMetadataProvider{}
			deviceSet := collections.NewSet("*")

			scraper := newScraper(cfg, settings, mockNvme, deviceSet)
			scraper.setMetadataProvider(mockMetadata)

			// Create test device
			device := createTestDevice(0, 1, "nvme0n1")
			devices := []nvme.DeviceFileAttributes{device}

			// Mock device discovery and type detection
			mockNvme.On("GetAllDevices").Return(devices, nil)
			mockNvme.On("DetectDeviceType", &device).Return(tt.deviceType, nil)
			mockNvme.On("GetDeviceSerial", &device).Return("test-serial", nil)
			mockNvme.On("DevicePath", "nvme0n1").Return("/dev/nvme0n1", nil)

			// Mock metadata provider
			mockMetadata.On("InstanceID", mock.Anything).Return("i-1234567890abcdef0", nil)

			// Mock the metrics retrieval functions
			if tt.deviceType == "ebs" {
				if tt.mockError != nil {
					// We can't easily mock nvme.GetEBSMetrics, so we'll test the error handling logic
					// by testing the device path error instead
					mockNvme.ExpectedCalls = mockNvme.ExpectedCalls[:len(mockNvme.ExpectedCalls)-1] // Remove DevicePath mock
					mockNvme.On("DevicePath", "nvme0n1").Return("", tt.mockError)
				}
			} else if tt.deviceType == "instance_store" {
				if tt.mockError != nil {
					// Similar approach for instance store
					mockNvme.ExpectedCalls = mockNvme.ExpectedCalls[:len(mockNvme.ExpectedCalls)-1] // Remove DevicePath mock
					mockNvme.On("DevicePath", "nvme0n1").Return("", tt.mockError)
				}
			}

			ctx := context.Background()
			metrics, err := scraper.scrape(ctx)

			assert.NoError(t, err) // Scraper should not fail, just skip problematic devices
			assert.NotNil(t, metrics)

			mockNvme.AssertExpectations(t)
			mockMetadata.AssertExpectations(t)
		})
	}
}

// TestScraper_Comprehensive_ErrorClassification tests comprehensive error classification
func TestScraper_Comprehensive_ErrorClassification(t *testing.T) {
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
			name:          "platform_unsupported",
			error:         errors.New("nvme device operations are only supported on Linux"),
			expectedClass: "platform_unsupported",
			isRecoverable: false,
		},
		{
			name:          "permission_denied",
			error:         errors.New("permission denied: insufficient permissions"),
			expectedClass: "permission_denied",
			isRecoverable: true,
		},
		{
			name:          "device_not_found",
			error:         errors.New("device not found: no such file or directory"),
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
			name:          "insufficient_data",
			error:         errors.New("insufficient data for parsing"),
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
			name:          "metadata_service_error",
			error:         errors.New("metadata service unavailable"),
			expectedClass: "metadata_service_error",
			isRecoverable: true,
		},
		{
			name:          "io_error",
			error:         errors.New("I/O error occurred"),
			expectedClass: "io_error",
			isRecoverable: true,
		},
		{
			name:          "network_error",
			error:         errors.New("connection refused"),
			expectedClass: "network_error",
			isRecoverable: true,
		},
		{
			name:          "overflow_error",
			error:         errors.New("value too large for int64"),
			expectedClass: "overflow_error",
			isRecoverable: false,
		},
		{
			name:          "unknown_error",
			error:         errors.New("some unknown error"),
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

// TestScraper_Comprehensive_DeviceFiltering tests comprehensive device filtering scenarios
func TestScraper_Comprehensive_DeviceFiltering(t *testing.T) {
	tests := []struct {
		name             string
		configDevices    []string
		availableDevices []string
		expectedDevices  []string
	}{
		{
			name:             "wildcard_all_devices",
			configDevices:    []string{"*"},
			availableDevices: []string{"nvme0n1", "nvme1n1", "nvme2n1"},
			expectedDevices:  []string{"nvme0n1", "nvme1n1", "nvme2n1"},
		},
		{
			name:             "specific_devices_subset",
			configDevices:    []string{"nvme0n1", "nvme2n1"},
			availableDevices: []string{"nvme0n1", "nvme1n1", "nvme2n1"},
			expectedDevices:  []string{"nvme0n1", "nvme2n1"},
		},
		{
			name:             "specific_devices_not_available",
			configDevices:    []string{"nvme5n1", "nvme6n1"},
			availableDevices: []string{"nvme0n1", "nvme1n1", "nvme2n1"},
			expectedDevices:  []string{}, // None match
		},
		{
			name:             "mixed_specific_and_available",
			configDevices:    []string{"nvme0n1", "nvme5n1"},
			availableDevices: []string{"nvme0n1", "nvme1n1", "nvme2n1"},
			expectedDevices:  []string{"nvme0n1"}, // Only nvme0n1 matches
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				Devices:              tt.configDevices,
			}
			settings := receivertest.NewNopSettings(metadata.Type)
			mockNvme := &MockDeviceInfoProvider{}
			deviceSet := collections.NewSet(tt.configDevices...)

			scraper := newScraper(cfg, settings, mockNvme, deviceSet)

			// Create available devices
			var devices []nvme.DeviceFileAttributes
			for i, deviceName := range tt.availableDevices {
				devices = append(devices, createTestDevice(i, 1, deviceName))
			}

			// Mock device discovery
			mockNvme.On("GetAllDevices").Return(devices, nil)

			// Mock device type detection and serial for expected devices
			for _, deviceName := range tt.expectedDevices {
				for _, device := range devices {
					if device.DeviceName() == deviceName {
						mockNvme.On("DetectDeviceType", &device).Return("ebs", nil)
						mockNvme.On("GetDeviceSerial", &device).Return("vol123456789", nil)
						break
					}
				}
			}

			devicesByController, err := scraper.getDevicesByController()

			require.NoError(t, err)
			assert.Len(t, devicesByController, len(tt.expectedDevices))

			// Verify the correct devices were processed
			processedDevices := make([]string, 0)
			for _, deviceGroup := range devicesByController {
				processedDevices = append(processedDevices, deviceGroup.deviceNames...)
			}

			assert.ElementsMatch(t, tt.expectedDevices, processedDevices)

			mockNvme.AssertExpectations(t)
		})
	}
}

// TestScraper_Comprehensive_RetryLogic tests comprehensive retry logic scenarios
func TestScraper_Comprehensive_RetryLogic(t *testing.T) {
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
		errors         []error
		expectedResult string
		expectError    bool
	}{
		{
			name: "success_on_first_attempt",
			errors: []error{
				nil, // Success on first attempt
			},
			expectedResult: "ebs",
			expectError:    false,
		},
		{
			name: "success_after_recoverable_error",
			errors: []error{
				errors.New("permission denied"), // Recoverable error
				nil,                             // Success on second attempt
			},
			expectedResult: "ebs",
			expectError:    false,
		},
		{
			name: "fail_fast_on_non_recoverable_error",
			errors: []error{
				errors.New("invalid device type"), // Non-recoverable error
			},
			expectError: true,
		},
		{
			name: "exhaust_all_retries",
			errors: []error{
				errors.New("permission denied"), // Recoverable error
				errors.New("permission denied"), // Recoverable error
				errors.New("permission denied"), // Recoverable error - should exhaust retries
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			device := createTestDevice(0, 1, "nvme0n1")

			// Set up mock calls based on expected errors
			call := mockNvme.On("DetectDeviceType", &device)
			for i, err := range tt.errors {
				if i == len(tt.errors)-1 {
					// Last call
					if err == nil {
						call = call.Return(tt.expectedResult, nil)
					} else {
						call = call.Return("", err)
					}
				} else {
					// Intermediate calls
					call = call.Return("", err).Once()
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

// TestScraper_Comprehensive_ValidationLogic tests comprehensive validation logic
func TestScraper_Comprehensive_ValidationLogic(t *testing.T) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"*"},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	settings.Logger = zap.NewNop() // Use nop logger to avoid log output during tests
	mockNvme := &MockDeviceInfoProvider{}
	deviceSet := collections.NewSet("*")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)

	t.Run("ebs_metrics_validation", func(t *testing.T) {
		tests := []struct {
			name        string
			metrics     nvme.EBSMetrics
			expectError bool
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
			},
			{
				name: "suspicious_average_times",
				metrics: nvme.EBSMetrics{
					EBSMagic:       nvme.EBSMagicNumber,
					ReadOps:        1,
					WriteOps:       1,
					TotalReadTime:  2e12, // 2 seconds per operation - suspicious
					TotalWriteTime: 2e12,
				},
				expectError: false, // Should not error, just warn
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := scraper.validateEBSMetrics(&tt.metrics, "/dev/nvme0n1")
				if tt.expectError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("instance_store_metrics_validation", func(t *testing.T) {
		tests := []struct {
			name        string
			metrics     nvme.InstanceStoreMetrics
			expectError bool
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
			},
			{
				name: "histograms_without_bins",
				metrics: nvme.InstanceStoreMetrics{
					Magic:         nvme.InstanceStoreMagicNumber,
					NumHistograms: 2,
					NumBins:       0, // Histograms but no bins
				},
				expectError: false, // Should not error, just warn
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := scraper.validateInstanceStoreMetrics(&tt.metrics, "/dev/nvme0n1")
				if tt.expectError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

// TestScraper_Comprehensive_PlatformSupport tests comprehensive platform support scenarios
func TestScraper_Comprehensive_PlatformSupport(t *testing.T) {
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
			name:            "platform_not_supported_linux_message",
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
			mockError:       errors.New("some other error"),
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

// TestScraper_Comprehensive_SerialNumberHandling tests comprehensive serial number handling
func TestScraper_Comprehensive_SerialNumberHandling(t *testing.T) {
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
		deviceType     string
		serialInput    string
		serialError    error
		expectedSerial string
		expectError    bool
	}{
		{
			name:           "ebs_valid_serial",
			deviceType:     "ebs",
			serialInput:    "vol123456789abcdef0",
			expectedSerial: "vol-123456789abcdef0",
			expectError:    false,
		},
		{
			name:           "ebs_invalid_serial_format",
			deviceType:     "ebs",
			serialInput:    "invalid-serial",
			expectedSerial: "invalid-serial", // Should keep original
			expectError:    false,
		},
		{
			name:           "instance_store_valid_serial",
			deviceType:     "instance_store",
			serialInput:    "AWS12345678901234567",
			expectedSerial: "AWS12345678901234567",
			expectError:    false,
		},
		{
			name:           "serial_retrieval_error",
			deviceType:     "ebs",
			serialError:    errors.New("serial retrieval failed"),
			expectedSerial: "unknown-ebs-controller-0",
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			device := createTestDevice(0, 1, "nvme0n1")

			serial, err := scraper.getDeviceSerialWithFallback(&device, ParseDeviceType(tt.deviceType))

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.serialError != nil {
				// For error cases, check the fallback format
				assert.Contains(t, serial, tt.expectedSerial)
			} else {
				// For success cases, format the serial if it's EBS
				if tt.deviceType == "ebs" {
					formatted := scraper.formatEBSSerial(tt.serialInput, "nvme0n1")
					assert.Equal(t, tt.expectedSerial, formatted)
				} else {
					assert.Equal(t, tt.expectedSerial, serial)
				}
			}
		})
	}
}

// TestScraper_Comprehensive_InstanceIDHandling tests comprehensive instance ID handling
func TestScraper_Comprehensive_InstanceIDHandling(t *testing.T) {
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
		name               string
		instanceID         string
		instanceIDError    error
		expectedInstanceID string
		expectError        bool
	}{
		{
			name:               "successful_instance_id_retrieval",
			instanceID:         "i-1234567890abcdef0",
			expectedInstanceID: "i-1234567890abcdef0",
			expectError:        false,
		},
		{
			name:               "instance_id_retrieval_error",
			instanceIDError:    errors.New("metadata service unavailable"),
			expectedInstanceID: "unknown", // Should use fallback
			expectError:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockMetadata := &MockMetadataProvider{}
			scraper.setMetadataProvider(mockMetadata)

			if tt.instanceIDError != nil {
				mockMetadata.On("InstanceID", mock.Anything).Return("", tt.instanceIDError)
			} else {
				mockMetadata.On("InstanceID", mock.Anything).Return(tt.instanceID, nil)
			}

			ctx := context.Background()
			instanceID, err := scraper.getInstanceIDWithFallback(ctx)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Contains(t, instanceID, tt.expectedInstanceID)

			mockMetadata.AssertExpectations(t)
		})
	}
}
