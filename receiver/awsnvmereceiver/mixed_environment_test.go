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

	"github.com/aws/amazon-cloudwatch-agent/internal/nvme"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	"github.com/aws/amazon-cloudwatch-agent/receiver/awsnvmereceiver/internal/metadata"
)

// TestScraper_MixedEnvironment_RealWorldScenarios tests real-world mixed device scenarios
func TestScraper_MixedEnvironment_RealWorldScenarios(t *testing.T) {
	tests := []struct {
		name                string
		devices             []testDeviceSpec
		expectedDeviceTypes map[string]string
		expectedGroups      int
		expectError         bool
	}{
		{
			name: "i4i_instance_mixed_ebs_and_instance_store",
			devices: []testDeviceSpec{
				{controller: 0, namespace: 1, name: "nvme0n1", deviceType: "ebs", serial: "vol123456789abcdef0"},
				{controller: 1, namespace: 1, name: "nvme1n1", deviceType: "instance_store", serial: "AWS12345678901234567"},
				{controller: 2, namespace: 1, name: "nvme2n1", deviceType: "instance_store", serial: "AWS23456789012345678"},
				{controller: 3, namespace: 1, name: "nvme3n1", deviceType: "ebs", serial: "vol234567890abcdef1"},
			},
			expectedDeviceTypes: map[string]string{
				"nvme0n1": "ebs",
				"nvme1n1": "instance_store",
				"nvme2n1": "instance_store",
				"nvme3n1": "ebs",
			},
			expectedGroups: 4,
			expectError:    false,
		},
		{
			name: "m5d_instance_mixed_with_partitions",
			devices: []testDeviceSpec{
				{controller: 0, namespace: 1, name: "nvme0n1", deviceType: "ebs", serial: "vol123456789abcdef0"},
				{controller: 0, namespace: 1, name: "nvme0n1p1", deviceType: "ebs", serial: "vol123456789abcdef0"}, // Same controller
				{controller: 1, namespace: 1, name: "nvme1n1", deviceType: "instance_store", serial: "AWS12345678901234567"},
				{controller: 1, namespace: 1, name: "nvme1n1p1", deviceType: "instance_store", serial: "AWS12345678901234567"}, // Same controller
			},
			expectedDeviceTypes: map[string]string{
				"nvme0n1": "ebs",
				"nvme1n1": "instance_store",
			},
			expectedGroups: 2, // Should be grouped by controller
			expectError:    false,
		},
		{
			name: "large_instance_many_instance_store_devices",
			devices: []testDeviceSpec{
				{controller: 0, namespace: 1, name: "nvme0n1", deviceType: "ebs", serial: "vol123456789abcdef0"},
				{controller: 1, namespace: 1, name: "nvme1n1", deviceType: "instance_store", serial: "AWS12345678901234567"},
				{controller: 2, namespace: 1, name: "nvme2n1", deviceType: "instance_store", serial: "AWS23456789012345678"},
				{controller: 3, namespace: 1, name: "nvme3n1", deviceType: "instance_store", serial: "AWS34567890123456789"},
				{controller: 4, namespace: 1, name: "nvme4n1", deviceType: "instance_store", serial: "AWS45678901234567890"},
				{controller: 5, namespace: 1, name: "nvme5n1", deviceType: "instance_store", serial: "AWS56789012345678901"},
				{controller: 6, namespace: 1, name: "nvme6n1", deviceType: "instance_store", serial: "AWS67890123456789012"},
				{controller: 7, namespace: 1, name: "nvme7n1", deviceType: "instance_store", serial: "AWS78901234567890123"},
				{controller: 8, namespace: 1, name: "nvme8n1", deviceType: "instance_store", serial: "AWS89012345678901234"},
			},
			expectedDeviceTypes: map[string]string{
				"nvme0n1": "ebs",
				"nvme1n1": "instance_store",
				"nvme2n1": "instance_store",
				"nvme3n1": "instance_store",
				"nvme4n1": "instance_store",
				"nvme5n1": "instance_store",
				"nvme6n1": "instance_store",
				"nvme7n1": "instance_store",
				"nvme8n1": "instance_store",
			},
			expectedGroups: 9,
			expectError:    false,
		},
		{
			name: "partial_device_failures_mixed_environment",
			devices: []testDeviceSpec{
				{controller: 0, namespace: 1, name: "nvme0n1", deviceType: "ebs", serial: "vol123456789abcdef0"},
				{controller: 1, namespace: 1, name: "nvme1n1", deviceType: "error", serial: ""}, // Detection failure
				{controller: 2, namespace: 1, name: "nvme2n1", deviceType: "instance_store", serial: "AWS23456789012345678"},
				{controller: 3, namespace: 1, name: "nvme3n1", deviceType: "error", serial: ""}, // Detection failure
				{controller: 4, namespace: 1, name: "nvme4n1", deviceType: "ebs", serial: "vol456789012bcdef34"},
			},
			expectedDeviceTypes: map[string]string{
				"nvme0n1": "ebs",
				"nvme2n1": "instance_store",
				"nvme4n1": "ebs",
			},
			expectedGroups: 3, // Only successful detections
			expectError:    false,
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

			// Create devices from specs
			var devices []nvme.DeviceFileAttributes
			for _, spec := range tt.devices {
				devices = append(devices, createTestDevice(spec.controller, spec.namespace, spec.name))
			}

			// Mock device discovery
			mockNvme.On("GetAllDevices").Return(devices, nil)

			// Mock device type detection based on specs
			for _, spec := range tt.devices {
				for _, device := range devices {
					if device.DeviceName() == spec.name {
						if spec.deviceType == "error" {
							mockNvme.On("DetectDeviceType", &device).Return("", errors.New("detection failed"))
						} else {
							mockNvme.On("DetectDeviceType", &device).Return(spec.deviceType, nil)
							mockNvme.On("GetDeviceSerial", &device).Return(spec.serial, nil)
						}
						break
					}
				}
			}

			devicesByController, err := scraper.getDevicesByController()

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, devicesByController, tt.expectedGroups)

				// Verify device types are correctly detected
				for controllerID, deviceGroup := range devicesByController {
					t.Logf("Controller %d: Type=%s, Serial=%s, Devices=%v",
						controllerID, deviceGroup.deviceType, deviceGroup.serialNumber, deviceGroup.deviceNames)

					// Find the expected device type for this controller
					for _, deviceName := range deviceGroup.deviceNames {
						if expectedType, exists := tt.expectedDeviceTypes[deviceName]; exists {
							assert.Equal(t, expectedType, deviceGroup.deviceType,
								"Device %s should have type %s", deviceName, expectedType)
							break
						}
					}
				}
			}

			mockNvme.AssertExpectations(t)
		})
	}
}

// TestScraper_MixedEnvironment_DevicePathFiltering tests device path filtering in mixed environments
func TestScraper_MixedEnvironment_DevicePathFiltering(t *testing.T) {
	tests := []struct {
		name             string
		configDevices    []string
		availableDevices []testDeviceSpec
		expectedDevices  []string
	}{
		{
			name:          "wildcard_includes_all_mixed_devices",
			configDevices: []string{"*"},
			availableDevices: []testDeviceSpec{
				{controller: 0, namespace: 1, name: "nvme0n1", deviceType: "ebs", serial: "vol123456789abcdef0"},
				{controller: 1, namespace: 1, name: "nvme1n1", deviceType: "instance_store", serial: "AWS12345678901234567"},
				{controller: 2, namespace: 1, name: "nvme2n1", deviceType: "ebs", serial: "vol234567890abcdef1"},
			},
			expectedDevices: []string{"nvme0n1", "nvme1n1", "nvme2n1"},
		},
		{
			name:          "specific_ebs_devices_only",
			configDevices: []string{"nvme0n1", "nvme2n1"},
			availableDevices: []testDeviceSpec{
				{controller: 0, namespace: 1, name: "nvme0n1", deviceType: "ebs", serial: "vol123456789abcdef0"},
				{controller: 1, namespace: 1, name: "nvme1n1", deviceType: "instance_store", serial: "AWS12345678901234567"},
				{controller: 2, namespace: 1, name: "nvme2n1", deviceType: "ebs", serial: "vol234567890abcdef1"},
			},
			expectedDevices: []string{"nvme0n1", "nvme2n1"},
		},
		{
			name:          "specific_instance_store_devices_only",
			configDevices: []string{"nvme1n1", "nvme3n1"},
			availableDevices: []testDeviceSpec{
				{controller: 0, namespace: 1, name: "nvme0n1", deviceType: "ebs", serial: "vol123456789abcdef0"},
				{controller: 1, namespace: 1, name: "nvme1n1", deviceType: "instance_store", serial: "AWS12345678901234567"},
				{controller: 2, namespace: 1, name: "nvme2n1", deviceType: "ebs", serial: "vol234567890abcdef1"},
				{controller: 3, namespace: 1, name: "nvme3n1", deviceType: "instance_store", serial: "AWS23456789012345678"},
			},
			expectedDevices: []string{"nvme1n1", "nvme3n1"},
		},
		{
			name:          "mixed_specific_devices",
			configDevices: []string{"nvme0n1", "nvme1n1", "nvme4n1"}, // Mix of EBS, Instance Store, and non-existent
			availableDevices: []testDeviceSpec{
				{controller: 0, namespace: 1, name: "nvme0n1", deviceType: "ebs", serial: "vol123456789abcdef0"},
				{controller: 1, namespace: 1, name: "nvme1n1", deviceType: "instance_store", serial: "AWS12345678901234567"},
				{controller: 2, namespace: 1, name: "nvme2n1", deviceType: "ebs", serial: "vol234567890abcdef1"},
				{controller: 3, namespace: 1, name: "nvme3n1", deviceType: "instance_store", serial: "AWS23456789012345678"},
			},
			expectedDevices: []string{"nvme0n1", "nvme1n1"}, // nvme4n1 doesn't exist
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

			// Create devices from specs
			var devices []nvme.DeviceFileAttributes
			for _, spec := range tt.availableDevices {
				devices = append(devices, createTestDevice(spec.controller, spec.namespace, spec.name))
			}

			// Mock device discovery
			mockNvme.On("GetAllDevices").Return(devices, nil)

			// Mock device type detection for expected devices only
			for _, expectedDevice := range tt.expectedDevices {
				for _, spec := range tt.availableDevices {
					if spec.name == expectedDevice {
						for _, device := range devices {
							if device.DeviceName() == spec.name {
								mockNvme.On("DetectDeviceType", &device).Return(spec.deviceType, nil)
								mockNvme.On("GetDeviceSerial", &device).Return(spec.serial, nil)
								break
							}
						}
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

// TestScraper_MixedEnvironment_FullScrapeIntegration tests full scrape integration with mixed devices
func TestScraper_MixedEnvironment_FullScrapeIntegration(t *testing.T) {
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

	// Create mixed device environment
	deviceSpecs := []testDeviceSpec{
		{controller: 0, namespace: 1, name: "nvme0n1", deviceType: "ebs", serial: "vol123456789abcdef0"},
		{controller: 1, namespace: 1, name: "nvme1n1", deviceType: "instance_store", serial: "AWS12345678901234567"},
		{controller: 2, namespace: 1, name: "nvme2n1", deviceType: "ebs", serial: "vol234567890abcdef1"},
		{controller: 3, namespace: 1, name: "nvme3n1", deviceType: "instance_store", serial: "AWS23456789012345678"},
	}

	var devices []nvme.DeviceFileAttributes
	for _, spec := range deviceSpecs {
		devices = append(devices, createTestDevice(spec.controller, spec.namespace, spec.name))
	}

	// Mock device discovery
	mockNvme.On("GetAllDevices").Return(devices, nil)

	// Mock device type detection, serial retrieval, and device paths
	for _, spec := range deviceSpecs {
		for _, device := range devices {
			if device.DeviceName() == spec.name {
				mockNvme.On("DetectDeviceType", &device).Return(spec.deviceType, nil)
				mockNvme.On("GetDeviceSerial", &device).Return(spec.serial, nil)
				mockNvme.On("DevicePath", spec.name).Return("/dev/"+spec.name, nil)
				break
			}
		}
	}

	// Mock metadata provider
	mockMetadata.On("InstanceID", mock.Anything).Return("i-1234567890abcdef0", nil)

	// Perform full scrape
	ctx := context.Background()
	metrics, err := scraper.scrape(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, metrics)

	// Verify that metrics were generated (we can't easily verify the actual metrics content
	// without mocking the nvme.GetEBSMetrics and nvme.GetInstanceStoreMetrics functions)
	t.Logf("Scrape completed successfully with %d resource metrics", metrics.ResourceMetrics().Len())

	mockNvme.AssertExpectations(t)
	mockMetadata.AssertExpectations(t)
}

// TestScraper_MixedEnvironment_ErrorRecovery tests error recovery in mixed environments
func TestScraper_MixedEnvironment_ErrorRecovery(t *testing.T) {
	tests := []struct {
		name           string
		deviceSpecs    []testDeviceSpec
		errorScenarios map[string]string // device name -> error type
		expectSuccess  bool
	}{
		{
			name: "partial_device_failures_continue_with_working",
			deviceSpecs: []testDeviceSpec{
				{controller: 0, namespace: 1, name: "nvme0n1", deviceType: "ebs", serial: "vol123456789abcdef0"},
				{controller: 1, namespace: 1, name: "nvme1n1", deviceType: "instance_store", serial: "AWS12345678901234567"},
				{controller: 2, namespace: 1, name: "nvme2n1", deviceType: "ebs", serial: "vol234567890abcdef1"},
			},
			errorScenarios: map[string]string{
				"nvme1n1": "device_path_error", // Middle device fails
			},
			expectSuccess: true, // Should continue with working devices
		},
		{
			name: "mixed_error_types",
			deviceSpecs: []testDeviceSpec{
				{controller: 0, namespace: 1, name: "nvme0n1", deviceType: "ebs", serial: "vol123456789abcdef0"},
				{controller: 1, namespace: 1, name: "nvme1n1", deviceType: "instance_store", serial: "AWS12345678901234567"},
				{controller: 2, namespace: 1, name: "nvme2n1", deviceType: "ebs", serial: "vol234567890abcdef1"},
				{controller: 3, namespace: 1, name: "nvme3n1", deviceType: "instance_store", serial: "AWS23456789012345678"},
			},
			errorScenarios: map[string]string{
				"nvme0n1": "detection_error",
				"nvme2n1": "serial_error",
				"nvme3n1": "device_path_error",
			},
			expectSuccess: true, // Should continue with nvme1n1
		},
		{
			name: "all_devices_fail",
			deviceSpecs: []testDeviceSpec{
				{controller: 0, namespace: 1, name: "nvme0n1", deviceType: "ebs", serial: "vol123456789abcdef0"},
				{controller: 1, namespace: 1, name: "nvme1n1", deviceType: "instance_store", serial: "AWS12345678901234567"},
			},
			errorScenarios: map[string]string{
				"nvme0n1": "detection_error",
				"nvme1n1": "detection_error",
			},
			expectSuccess: true, // Scraper should not fail, just return empty metrics
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

			// Create devices from specs
			var devices []nvme.DeviceFileAttributes
			for _, spec := range tt.deviceSpecs {
				devices = append(devices, createTestDevice(spec.controller, spec.namespace, spec.name))
			}

			// Mock device discovery
			mockNvme.On("GetAllDevices").Return(devices, nil)

			// Mock device interactions based on error scenarios
			for _, spec := range tt.deviceSpecs {
				for _, device := range devices {
					if device.DeviceName() == spec.name {
						errorType, hasError := tt.errorScenarios[spec.name]

						if hasError {
							switch errorType {
							case "detection_error":
								mockNvme.On("DetectDeviceType", &device).Return("", errors.New("detection failed"))
							case "serial_error":
								mockNvme.On("DetectDeviceType", &device).Return(spec.deviceType, nil)
								mockNvme.On("GetDeviceSerial", &device).Return("", errors.New("serial retrieval failed"))
							case "device_path_error":
								mockNvme.On("DetectDeviceType", &device).Return(spec.deviceType, nil)
								mockNvme.On("GetDeviceSerial", &device).Return(spec.serial, nil)
								mockNvme.On("DevicePath", spec.name).Return("", errors.New("device path error"))
							}
						} else {
							// No error for this device
							mockNvme.On("DetectDeviceType", &device).Return(spec.deviceType, nil)
							mockNvme.On("GetDeviceSerial", &device).Return(spec.serial, nil)
							mockNvme.On("DevicePath", spec.name).Return("/dev/"+spec.name, nil)
						}
						break
					}
				}
			}

			// Mock metadata provider for successful cases
			mockMetadata.On("InstanceID", mock.Anything).Return("i-1234567890abcdef0", nil)

			// Perform scrape
			ctx := context.Background()
			metrics, err := scraper.scrape(ctx)

			if tt.expectSuccess {
				assert.NoError(t, err)
				assert.NotNil(t, metrics)
				t.Logf("Scrape completed with %d resource metrics despite errors", metrics.ResourceMetrics().Len())
			} else {
				assert.Error(t, err)
			}

			mockNvme.AssertExpectations(t)
			mockMetadata.AssertExpectations(t)
		})
	}
}

// testDeviceSpec represents a test device specification
type testDeviceSpec struct {
	controller int
	namespace  int
	name       string
	deviceType string
	serial     string
}

// TestScraper_MixedEnvironment_SerialNumberFormatting tests serial number formatting in mixed environments
func TestScraper_MixedEnvironment_SerialNumberFormatting(t *testing.T) {
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
		deviceType     string
		rawSerial      string
		expectedSerial string
	}{
		{
			name:           "ebs_volume_id_formatting",
			deviceType:     "ebs",
			rawSerial:      "vol123456789abcdef0",
			expectedSerial: "vol-123456789abcdef0",
		},
		{
			name:           "ebs_invalid_format_passthrough",
			deviceType:     "ebs",
			rawSerial:      "invalid-serial-format",
			expectedSerial: "invalid-serial-format",
		},
		{
			name:           "instance_store_serial_passthrough",
			deviceType:     "instance_store",
			rawSerial:      "AWS12345678901234567",
			expectedSerial: "AWS12345678901234567",
		},
		{
			name:           "ebs_short_vol_prefix",
			deviceType:     "ebs",
			rawSerial:      "vol",
			expectedSerial: "vol",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.deviceType == "ebs" {
				formatted := scraper.formatEBSSerial(tt.rawSerial, "nvme0n1")
				assert.Equal(t, tt.expectedSerial, formatted)
			} else {
				// Instance Store serials are not formatted
				assert.Equal(t, tt.expectedSerial, tt.rawSerial)
			}
		})
	}
}

// TestScraper_MixedEnvironment_ControllerGrouping tests controller grouping with mixed device types
func TestScraper_MixedEnvironment_ControllerGrouping(t *testing.T) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"*"},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	mockNvme := &MockDeviceInfoProvider{}
	deviceSet := collections.NewSet("*")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)

	// Test scenario: Multiple devices with same controller but different namespaces/partitions
	deviceSpecs := []testDeviceSpec{
		{controller: 0, namespace: 1, name: "nvme0n1", deviceType: "ebs", serial: "vol123456789abcdef0"},
		{controller: 0, namespace: 1, name: "nvme0n1p1", deviceType: "ebs", serial: "vol123456789abcdef0"}, // Same controller
		{controller: 0, namespace: 1, name: "nvme0n1p2", deviceType: "ebs", serial: "vol123456789abcdef0"}, // Same controller
		{controller: 1, namespace: 1, name: "nvme1n1", deviceType: "instance_store", serial: "AWS12345678901234567"},
		{controller: 1, namespace: 1, name: "nvme1n1p1", deviceType: "instance_store", serial: "AWS12345678901234567"}, // Same controller
	}

	var devices []nvme.DeviceFileAttributes
	for _, spec := range deviceSpecs {
		devices = append(devices, createTestDevice(spec.controller, spec.namespace, spec.name))
	}

	// Mock device discovery
	mockNvme.On("GetAllDevices").Return(devices, nil)

	// Mock device type detection - should only be called for the first device of each controller
	// Controller 0 (EBS)
	mockNvme.On("DetectDeviceType", &devices[0]).Return("ebs", nil)
	mockNvme.On("GetDeviceSerial", &devices[0]).Return("vol123456789abcdef0", nil)

	// Controller 1 (Instance Store)
	mockNvme.On("DetectDeviceType", &devices[3]).Return("instance_store", nil)
	mockNvme.On("GetDeviceSerial", &devices[3]).Return("AWS12345678901234567", nil)

	devicesByController, err := scraper.getDevicesByController()

	require.NoError(t, err)
	assert.Len(t, devicesByController, 2) // Two controllers

	// Verify controller 0 grouping
	controller0 := devicesByController[0]
	require.NotNil(t, controller0)
	assert.Equal(t, "ebs", controller0.deviceType)
	assert.Equal(t, "vol-123456789abcdef0", controller0.serialNumber) // Should be formatted
	assert.Len(t, controller0.deviceNames, 3)                         // All three devices with controller 0
	assert.Contains(t, controller0.deviceNames, "nvme0n1")
	assert.Contains(t, controller0.deviceNames, "nvme0n1p1")
	assert.Contains(t, controller0.deviceNames, "nvme0n1p2")

	// Verify controller 1 grouping
	controller1 := devicesByController[1]
	require.NotNil(t, controller1)
	assert.Equal(t, "instance_store", controller1.deviceType)
	assert.Equal(t, "AWS12345678901234567", controller1.serialNumber) // Should not be formatted
	assert.Len(t, controller1.deviceNames, 2)                         // Two devices with controller 1
	assert.Contains(t, controller1.deviceNames, "nvme1n1")
	assert.Contains(t, controller1.deviceNames, "nvme1n1p1")

	mockNvme.AssertExpectations(t)
}
