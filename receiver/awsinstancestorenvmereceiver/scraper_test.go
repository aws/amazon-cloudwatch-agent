// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsinstancestorenvmereceiver

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/aws/amazon-cloudwatch-agent/internal/nvme"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
)

func TestRecordMetric(t *testing.T) {
	tests := []struct {
		name          string
		value         uint64
		expectSkipped bool
		expectError   bool
	}{
		{
			name:          "valid value",
			value:         1000,
			expectSkipped: false,
			expectError:   false,
		},
		{
			name:          "max int64 value",
			value:         9223372036854775807, // math.MaxInt64
			expectSkipped: false,
			expectError:   false,
		},
		{
			name:          "overflow value",
			value:         9223372036854775808, // math.MaxInt64 + 1
			expectSkipped: true,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test scraper
			settings := receivertest.NewNopSettings(component.MustNewType("awsinstancestorenvmereceiver"))
			devices := collections.NewSet[string]()

			// Create a minimal scraper for testing recordMetric
			scraper := &nvmeScraper{
				logger:         settings.Logger,
				allowedDevices: devices,
			}

			// Mock function to track if it was called
			called := false
			mockRecordFn := func(ts pcommon.Timestamp, val int64) {
				called = true
				if !tt.expectSkipped {
					assert.True(t, val >= 0, "recorded value should be non-negative")
				}
			}

			// Test recordMetric
			now := pcommon.NewTimestampFromTime(time.Now())
			scraper.recordMetric(mockRecordFn, now, tt.value)

			if tt.expectSkipped {
				assert.False(t, called, "record function should not be called for overflow values")
			} else {
				assert.True(t, called, "record function should be called for valid values")
			}
		})
	}
}

func TestSafeUint64ToInt64(t *testing.T) {
	tests := []struct {
		name        string
		value       uint64
		expected    int64
		expectError bool
	}{
		{
			name:        "zero value",
			value:       0,
			expected:    0,
			expectError: false,
		},
		{
			name:        "small positive value",
			value:       1000,
			expected:    1000,
			expectError: false,
		},
		{
			name:        "max int64 value",
			value:       9223372036854775807, // math.MaxInt64
			expected:    9223372036854775807,
			expectError: false,
		},
		{
			name:        "overflow value",
			value:       9223372036854775808, // math.MaxInt64 + 1
			expected:    0,
			expectError: true,
		},
		{
			name:        "max uint64 value",
			value:       18446744073709551615, // math.MaxUint64
			expected:    0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := safeUint64ToInt64(tt.value)

			if tt.expectError {
				assert.Error(t, err)
				assert.Equal(t, int64(0), result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestScraperLifecycle(t *testing.T) {
	// Create test scraper
	settings := receivertest.NewNopSettings(component.MustNewType("awsinstancestorenvmereceiver"))
	devices := collections.NewSet[string]()

	// Create a minimal scraper for testing lifecycle
	scraper := &nvmeScraper{
		logger:         settings.Logger,
		allowedDevices: devices,
	}

	ctx := context.Background()

	// Test start
	err := scraper.start(ctx, nil)
	assert.NoError(t, err)

	// Test shutdown
	err = scraper.shutdown(ctx)
	assert.NoError(t, err)
}

func TestNewScraper(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	settings := receivertest.NewNopSettings(component.MustNewType("awsinstancestorenvmereceiver"))
	devices := collections.NewSet[string]()
	devices.Add("*")

	// Test that newScraper creates a scraper with the expected fields
	scraper := newScraper(cfg, settings, nil, devices)

	assert.NotNil(t, scraper)
	assert.NotNil(t, scraper.logger)
	assert.NotNil(t, scraper.mb)
	assert.NotNil(t, scraper.metadataProvider)
	assert.NotNil(t, scraper.allowedDevices)
	assert.True(t, scraper.allowedDevices.Contains("*"))
}

func TestRecordMetricWithName(t *testing.T) {
	tests := []struct {
		name           string
		value          uint64
		metricName     string
		expectSkipped  bool
		expectError    bool
		expectedReturn int
	}{
		{
			name:           "valid value",
			value:          1000,
			metricName:     "test_metric",
			expectSkipped:  false,
			expectError:    false,
			expectedReturn: 1,
		},
		{
			name:           "max int64 value",
			value:          9223372036854775807, // math.MaxInt64
			metricName:     "test_metric",
			expectSkipped:  false,
			expectError:    false,
			expectedReturn: 1,
		},
		{
			name:           "overflow value",
			value:          9223372036854775808, // math.MaxInt64 + 1
			metricName:     "overflow_metric",
			expectSkipped:  true,
			expectError:    true,
			expectedReturn: 0,
		},
		{
			name:           "max uint64 value",
			value:          18446744073709551615, // math.MaxUint64
			metricName:     "max_uint64_metric",
			expectSkipped:  true,
			expectError:    true,
			expectedReturn: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test scraper
			settings := receivertest.NewNopSettings(component.MustNewType("awsinstancestorenvmereceiver"))
			devices := collections.NewSet[string]()

			scraper := &nvmeScraper{
				logger:         settings.Logger,
				allowedDevices: devices,
			}

			// Mock function to track if it was called
			called := false
			mockRecordFn := func(ts pcommon.Timestamp, val int64) {
				called = true
				if !tt.expectSkipped {
					assert.True(t, val >= 0, "recorded value should be non-negative")
				}
			}

			// Test recordMetricWithName
			now := pcommon.NewTimestampFromTime(time.Now())
			result := scraper.recordMetricWithName(mockRecordFn, now, tt.value, tt.metricName)

			assert.Equal(t, tt.expectedReturn, result, "should return expected value")

			if tt.expectSkipped {
				assert.False(t, called, "record function should not be called for overflow values")
			} else {
				assert.True(t, called, "record function should be called for valid values")
			}
		})
	}
}

// mockDeviceInfoProvider is a mock implementation of nvme.DeviceInfoProvider for testing
type mockDeviceInfoProvider struct {
	devices                []nvme.DeviceFileAttributes
	deviceModels           map[string]string
	deviceSerials          map[string]string
	devicePaths            map[string]string
	getAllDevicesError     error
	getDeviceModelError    error
	getDeviceSerialError   error
	isInstanceStoreError   error
	devicePathError        error
	isInstanceStoreResults map[string]bool
}

func newMockDeviceInfoProvider() *mockDeviceInfoProvider {
	return &mockDeviceInfoProvider{
		devices:                []nvme.DeviceFileAttributes{},
		deviceModels:           make(map[string]string),
		deviceSerials:          make(map[string]string),
		devicePaths:            make(map[string]string),
		isInstanceStoreResults: make(map[string]bool),
	}
}

func (m *mockDeviceInfoProvider) GetAllDevices() ([]nvme.DeviceFileAttributes, error) {
	if m.getAllDevicesError != nil {
		return nil, m.getAllDevicesError
	}
	return m.devices, nil
}

func (m *mockDeviceInfoProvider) GetDeviceModel(device *nvme.DeviceFileAttributes) (string, error) {
	if m.getDeviceModelError != nil {
		return "", m.getDeviceModelError
	}
	if model, exists := m.deviceModels[device.DeviceName()]; exists {
		return model, nil
	}
	return "Unknown Model", nil
}

func (m *mockDeviceInfoProvider) GetDeviceSerial(device *nvme.DeviceFileAttributes) (string, error) {
	if m.getDeviceSerialError != nil {
		return "", m.getDeviceSerialError
	}
	if serial, exists := m.deviceSerials[device.DeviceName()]; exists {
		return serial, nil
	}
	return "unknown-serial", nil
}

func (m *mockDeviceInfoProvider) IsEbsDevice(device *nvme.DeviceFileAttributes) (bool, error) {
	// Not used in Instance Store receiver
	return false, nil
}

func (m *mockDeviceInfoProvider) IsInstanceStoreDevice(device *nvme.DeviceFileAttributes) (bool, error) {
	if m.isInstanceStoreError != nil {
		return false, m.isInstanceStoreError
	}
	if result, exists := m.isInstanceStoreResults[device.DeviceName()]; exists {
		return result, nil
	}
	return false, nil
}

func (m *mockDeviceInfoProvider) DevicePath(device string) (string, error) {
	if m.devicePathError != nil {
		return "", m.devicePathError
	}
	if path, exists := m.devicePaths[device]; exists {
		return path, nil
	}
	return fmt.Sprintf("/dev/%s", device), nil
}

func TestGetInstanceStoreDevicesByController_ErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		setupMock      func(*mockDeviceInfoProvider)
		allowedDevices []string
		expectError    bool
		expectedCount  int
		description    string
	}{
		{
			name: "GetAllDevices fails",
			setupMock: func(m *mockDeviceInfoProvider) {
				m.getAllDevicesError = errors.New("failed to read device directory")
			},
			allowedDevices: []string{"*"},
			expectError:    true,
			expectedCount:  0,
			description:    "should return error when device discovery fails",
		},
		{
			name: "no devices found",
			setupMock: func(m *mockDeviceInfoProvider) {
				m.devices = []nvme.DeviceFileAttributes{}
			},
			allowedDevices: []string{"*"},
			expectError:    false,
			expectedCount:  0,
			description:    "should handle empty device list gracefully",
		},
		{
			name: "IsInstanceStoreDevice fails for all devices",
			setupMock: func(m *mockDeviceInfoProvider) {
				device1, _ := nvme.ParseNvmeDeviceFileName("nvme0n1")
				device2, _ := nvme.ParseNvmeDeviceFileName("nvme1n1")
				m.devices = []nvme.DeviceFileAttributes{device1, device2}
				m.isInstanceStoreError = errors.New("permission denied")
			},
			allowedDevices: []string{"*"},
			expectError:    true,
			expectedCount:  0,
			description:    "should return error when all device validations fail",
		},
		{
			name: "GetDeviceSerial fails but continues",
			setupMock: func(m *mockDeviceInfoProvider) {
				device1, _ := nvme.ParseNvmeDeviceFileName("nvme0n1")
				m.devices = []nvme.DeviceFileAttributes{device1}
				m.isInstanceStoreResults["nvme0n1"] = true
				m.getDeviceSerialError = errors.New("failed to read serial")
			},
			allowedDevices: []string{"*"},
			expectError:    false,
			expectedCount:  1,
			description:    "should continue with placeholder serial when serial read fails",
		},
		{
			name: "mixed success and failure",
			setupMock: func(m *mockDeviceInfoProvider) {
				device1, _ := nvme.ParseNvmeDeviceFileName("nvme0n1")
				device2, _ := nvme.ParseNvmeDeviceFileName("nvme1n1")
				device3, _ := nvme.ParseNvmeDeviceFileName("nvme2n1")
				m.devices = []nvme.DeviceFileAttributes{device1, device2, device3}

				// First device succeeds
				m.isInstanceStoreResults["nvme0n1"] = true
				m.deviceSerials["nvme0n1"] = "serial-001"

				// Second device is not Instance Store
				m.isInstanceStoreResults["nvme1n1"] = false

				// Third device succeeds
				m.isInstanceStoreResults["nvme2n1"] = true
				m.deviceSerials["nvme2n1"] = "serial-003"
			},
			allowedDevices: []string{"*"},
			expectError:    false,
			expectedCount:  2,
			description:    "should handle mixed success and failure scenarios",
		},
		{
			name: "device filtering works correctly",
			setupMock: func(m *mockDeviceInfoProvider) {
				device1, _ := nvme.ParseNvmeDeviceFileName("nvme0n1")
				device2, _ := nvme.ParseNvmeDeviceFileName("nvme1n1")
				m.devices = []nvme.DeviceFileAttributes{device1, device2}

				// Both are Instance Store devices
				m.isInstanceStoreResults["nvme0n1"] = true
				m.isInstanceStoreResults["nvme1n1"] = true
				m.deviceSerials["nvme0n1"] = "serial-001"
				m.deviceSerials["nvme1n1"] = "serial-002"
			},
			allowedDevices: []string{"nvme0n1"}, // Only allow first device
			expectError:    false,
			expectedCount:  1,
			description:    "should respect device filtering configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock provider and set it up
			mockProvider := newMockDeviceInfoProvider()
			tt.setupMock(mockProvider)

			// Create scraper with mock provider
			settings := receivertest.NewNopSettings(component.MustNewType("awsinstancestorenvmereceiver"))
			devices := collections.NewSet[string]()
			for _, device := range tt.allowedDevices {
				devices.Add(device)
			}

			scraper := &nvmeScraper{
				logger:         settings.Logger,
				nvme:           mockProvider,
				allowedDevices: devices,
			}

			// Test getInstanceStoreDevicesByController
			result, err := scraper.getInstanceStoreDevicesByController()

			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}

			assert.Equal(t, tt.expectedCount, len(result), "should return expected number of devices")
		})
	}
}

func TestScrape_ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		setupMock   func(*mockDeviceInfoProvider)
		expectError bool
		description string
	}{
		{
			name: "device discovery fails",
			setupMock: func(m *mockDeviceInfoProvider) {
				m.getAllDevicesError = errors.New("device discovery failed")
			},
			expectError: true,
			description: "should return error when device discovery fails",
		},
		{
			name: "no devices found",
			setupMock: func(m *mockDeviceInfoProvider) {
				m.devices = []nvme.DeviceFileAttributes{}
			},
			expectError: false,
			description: "should handle no devices gracefully",
		},
		{
			name: "all devices fail validation",
			setupMock: func(m *mockDeviceInfoProvider) {
				device1, _ := nvme.ParseNvmeDeviceFileName("nvme0n1")
				m.devices = []nvme.DeviceFileAttributes{device1}
				m.isInstanceStoreResults["nvme0n1"] = false
			},
			expectError: false,
			description: "should handle all devices being non-Instance Store",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock provider and set it up
			mockProvider := newMockDeviceInfoProvider()
			tt.setupMock(mockProvider)

			// Create scraper with mock provider
			cfg := createDefaultConfig().(*Config)
			settings := receivertest.NewNopSettings(component.MustNewType("awsinstancestorenvmereceiver"))
			devices := collections.NewSet[string]()
			devices.Add("*")

			scraper := newScraper(cfg, settings, mockProvider, devices)

			// Test scrape
			ctx := context.Background()
			metrics, err := scraper.scrape(ctx)

			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
				assert.NotNil(t, metrics, "should return metrics even on partial failures")
			}
		})
	}
}
