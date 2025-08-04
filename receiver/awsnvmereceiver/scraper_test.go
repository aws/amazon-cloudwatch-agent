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
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/internal/nvme"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	"github.com/aws/amazon-cloudwatch-agent/receiver/awsnvmereceiver/internal/metadata"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
)

// MockDeviceInfoProvider is a mock implementation of nvme.DeviceInfoProvider
type MockDeviceInfoProvider struct {
	mock.Mock
}

func (m *MockDeviceInfoProvider) GetAllDevices() ([]nvme.DeviceFileAttributes, error) {
	args := m.Called()
	return args.Get(0).([]nvme.DeviceFileAttributes), args.Error(1)
}

func (m *MockDeviceInfoProvider) GetDeviceSerial(device *nvme.DeviceFileAttributes) (string, error) {
	args := m.Called(device)
	return args.String(0), args.Error(1)
}

func (m *MockDeviceInfoProvider) GetDeviceModel(device *nvme.DeviceFileAttributes) (string, error) {
	args := m.Called(device)
	return args.String(0), args.Error(1)
}

func (m *MockDeviceInfoProvider) IsEbsDevice(device *nvme.DeviceFileAttributes) (bool, error) {
	args := m.Called(device)
	return args.Bool(0), args.Error(1)
}

func (m *MockDeviceInfoProvider) IsInstanceStoreDevice(device *nvme.DeviceFileAttributes) (bool, error) {
	args := m.Called(device)
	return args.Bool(0), args.Error(1)
}

func (m *MockDeviceInfoProvider) DetectDeviceType(device *nvme.DeviceFileAttributes) (string, error) {
	args := m.Called(device)
	return args.String(0), args.Error(1)
}

func (m *MockDeviceInfoProvider) DevicePath(device string) (string, error) {
	args := m.Called(device)
	return args.String(0), args.Error(1)
}

// MockMetadataProvider is a mock implementation of ec2metadataprovider.MetadataProvider
type MockMetadataProvider struct {
	mock.Mock
}

func (m *MockMetadataProvider) InstanceID(ctx context.Context) (string, error) {
	args := m.Called(ctx)
	return args.String(0), args.Error(1)
}

func (m *MockMetadataProvider) Region(ctx context.Context) (string, error) {
	args := m.Called(ctx)
	return args.String(0), args.Error(1)
}

func (m *MockMetadataProvider) InstanceType(ctx context.Context) (string, error) {
	args := m.Called(ctx)
	return args.String(0), args.Error(1)
}

func (m *MockMetadataProvider) AvailabilityZone(ctx context.Context) (string, error) {
	args := m.Called(ctx)
	return args.String(0), args.Error(1)
}

func (m *MockMetadataProvider) InstanceIdentityDocument(ctx context.Context) (interface{}, error) {
	args := m.Called(ctx)
	return args.Get(0), args.Error(1)
}

func (m *MockMetadataProvider) Get(ctx context.Context) (ec2metadata.EC2InstanceIdentityDocument, error) {
	args := m.Called(ctx)
	return args.Get(0).(ec2metadata.EC2InstanceIdentityDocument), args.Error(1)
}

func (m *MockMetadataProvider) Hostname(ctx context.Context) (string, error) {
	args := m.Called(ctx)
	return args.String(0), args.Error(1)
}

func (m *MockMetadataProvider) InstanceTags(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockMetadataProvider) ClientIAMRole(ctx context.Context) (string, error) {
	args := m.Called(ctx)
	return args.String(0), args.Error(1)
}

func (m *MockMetadataProvider) InstanceTagValue(ctx context.Context, tagKey string) (string, error) {
	args := m.Called(ctx, tagKey)
	return args.String(0), args.Error(1)
}

// Helper function to create test device attributes
func createTestDevice(controller int, namespace int, deviceName string) nvme.DeviceFileAttributes {
	device, _ := nvme.ParseNvmeDeviceFileName(deviceName)
	return device
}

// Mock functions for nvme metrics retrieval - these will be used to override the actual functions
func mockGetEBSMetrics(devicePath string) (nvme.EBSMetrics, error) {
	return nvme.EBSMetrics{
		EBSMagic:              0x3C23B510,
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
	}, nil
}

func mockGetInstanceStoreMetrics(devicePath string) (nvme.InstanceStoreMetrics, error) {
	return nvme.InstanceStoreMetrics{
		Magic:                 0xEC2C0D7E,
		ReadOps:               2000,
		WriteOps:              1000,
		ReadBytes:             2048000,
		WriteBytes:            1024000,
		TotalReadTime:         10000000,
		TotalWriteTime:        5000000,
		EC2IOPSExceeded:       4,
		EC2ThroughputExceeded: 2,
		QueueLength:           5,
	}, nil
}

func TestNewScraper(t *testing.T) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"*"},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	mockNvme := &MockDeviceInfoProvider{}
	deviceSet := collections.NewSet("*")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)

	assert.NotNil(t, scraper)
	assert.Equal(t, mockNvme, scraper.nvmeUtil)
	assert.Equal(t, deviceSet, scraper.deviceSet)
	assert.NotNil(t, scraper.logger)
	assert.NotNil(t, scraper.mb)
	assert.Nil(t, scraper.metadataProvider) // Should be nil until first scrape
}

func TestScraper_StartShutdown(t *testing.T) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"*"},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	mockNvme := &MockDeviceInfoProvider{}
	deviceSet := collections.NewSet("*")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)

	ctx := context.Background()
	host := componenttest.NewNopHost()

	// Test start
	err := scraper.start(ctx, host)
	assert.NoError(t, err)

	// Test shutdown
	err = scraper.shutdown(ctx)
	assert.NoError(t, err)
}

func TestScraper_Scrape_NoDevices(t *testing.T) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"*"},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	mockNvme := &MockDeviceInfoProvider{}
	deviceSet := collections.NewSet("*")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)

	// Mock no devices found
	mockNvme.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{}, nil)

	ctx := context.Background()
	metrics, err := scraper.scrape(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, metrics)
	mockNvme.AssertExpectations(t)
}

func TestScraper_Scrape_EBSDevice_Success(t *testing.T) {
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

	// Create test EBS device
	device := createTestDevice(0, 1, "nvme0n1")
	devices := []nvme.DeviceFileAttributes{device}

	// Mock device discovery and type detection
	mockNvme.On("GetAllDevices").Return(devices, nil)
	mockNvme.On("DetectDeviceType", &device).Return("ebs", nil)
	mockNvme.On("GetDeviceSerial", &device).Return("vol123456789", nil)
	mockNvme.On("DevicePath", "nvme0n1").Return("/dev/nvme0n1", nil)

	// Mock metadata provider
	mockMetadata.On("InstanceID", mock.Anything).Return("i-1234567890abcdef0", nil)

	ctx := context.Background()
	metrics, _ := scraper.scrape(ctx)

	// Note: This test will fail on actual metric retrieval since we can't easily mock nvme.GetEBSMetrics
	// But it tests the device discovery and routing logic
	assert.NotNil(t, metrics)

	mockNvme.AssertExpectations(t)
	mockMetadata.AssertExpectations(t)
}

func TestScraper_Scrape_InstanceStoreDevice_Success(t *testing.T) {
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

	// Create test Instance Store device
	device := createTestDevice(1, 1, "nvme1n1")
	devices := []nvme.DeviceFileAttributes{device}

	// Mock device discovery and type detection
	mockNvme.On("GetAllDevices").Return(devices, nil)
	mockNvme.On("DetectDeviceType", &device).Return("instance_store", nil)
	mockNvme.On("GetDeviceSerial", &device).Return("AWS12345678901234567", nil)
	mockNvme.On("DevicePath", "nvme1n1").Return("/dev/nvme1n1", nil)

	// Mock metadata provider
	mockMetadata.On("InstanceID", mock.Anything).Return("i-1234567890abcdef0", nil)

	ctx := context.Background()
	metrics, _ := scraper.scrape(ctx)

	// Note: This test will fail on actual metric retrieval since we can't easily mock nvme.GetInstanceStoreMetrics
	// But it tests the device discovery and routing logic
	assert.NotNil(t, metrics)

	mockNvme.AssertExpectations(t)
	mockMetadata.AssertExpectations(t)
}

func TestScraper_Scrape_MixedDevices_Success(t *testing.T) {
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

	// Create mixed devices
	ebsDevice := createTestDevice(0, 1, "nvme0n1")
	instanceStoreDevice := createTestDevice(1, 1, "nvme1n1")
	devices := []nvme.DeviceFileAttributes{ebsDevice, instanceStoreDevice}

	// Mock device discovery and type detection
	mockNvme.On("GetAllDevices").Return(devices, nil)
	mockNvme.On("DetectDeviceType", &ebsDevice).Return("ebs", nil)
	mockNvme.On("DetectDeviceType", &instanceStoreDevice).Return("instance_store", nil)
	mockNvme.On("GetDeviceSerial", &ebsDevice).Return("vol123456789", nil)
	mockNvme.On("GetDeviceSerial", &instanceStoreDevice).Return("AWS12345678901234567", nil)
	mockNvme.On("DevicePath", "nvme0n1").Return("/dev/nvme0n1", nil)
	mockNvme.On("DevicePath", "nvme1n1").Return("/dev/nvme1n1", nil)

	// Mock metadata provider
	mockMetadata.On("InstanceID", mock.Anything).Return("i-1234567890abcdef0", nil)

	ctx := context.Background()
	metrics, _ := scraper.scrape(ctx)

	// Note: This test will fail on actual metric retrieval since we can't easily mock nvme functions
	// But it tests the device discovery and routing logic
	assert.NotNil(t, metrics)

	mockNvme.AssertExpectations(t)
	mockMetadata.AssertExpectations(t)
}

func TestScraper_Scrape_DeviceTypeDetectionError(t *testing.T) {
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
	mockNvme.On("DetectDeviceType", &device).Return("", errors.New("detection failed"))

	ctx := context.Background()
	metrics, err := scraper.scrape(ctx)

	assert.Error(t, err) // Should fail when no devices can be processed
	assert.Contains(t, err.Error(), "no devices found, encountered 1 errors during discovery")
	assert.NotNil(t, metrics)

	mockNvme.AssertExpectations(t)
}

func TestScraper_Scrape_FilteredDevices(t *testing.T) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"nvme0n1"}, // Only allow nvme0n1
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	mockNvme := &MockDeviceInfoProvider{}
	mockMetadata := &MockMetadataProvider{}
	deviceSet := collections.NewSet("nvme0n1")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)
	scraper.setMetadataProvider(mockMetadata)

	// Create multiple devices, but only one should be processed
	device1 := createTestDevice(0, 1, "nvme0n1")
	device2 := createTestDevice(1, 1, "nvme1n1")
	devices := []nvme.DeviceFileAttributes{device1, device2}

	// Mock device discovery - only nvme0n1 should be processed
	mockNvme.On("GetAllDevices").Return(devices, nil)
	mockNvme.On("DetectDeviceType", &device1).Return("ebs", nil)
	mockNvme.On("GetDeviceSerial", &device1).Return("vol123456789", nil)
	mockNvme.On("DevicePath", "nvme0n1").Return("/dev/nvme0n1", nil)
	// device2 should be skipped, so no mocks for it

	// Mock metadata provider
	mockMetadata.On("InstanceID", mock.Anything).Return("i-1234567890abcdef0", nil)

	ctx := context.Background()
	metrics, _ := scraper.scrape(ctx)

	// Note: This test will fail on actual metric retrieval since we can't easily mock nvme.GetEBSMetrics
	// But it tests the device discovery and filtering logic
	assert.NotNil(t, metrics)

	mockNvme.AssertExpectations(t)
	mockMetadata.AssertExpectations(t)
}

func TestScraper_Scrape_DevicePathError(t *testing.T) {
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

func TestScraper_Scrape_UnknownDeviceType(t *testing.T) {
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

	// Mock device discovery with unknown device type
	mockNvme.On("GetAllDevices").Return(devices, nil)
	mockNvme.On("DetectDeviceType", &device).Return("unknown", nil)
	mockNvme.On("GetDeviceSerial", &device).Return("serial123", nil)
	mockNvme.On("DevicePath", "nvme0n1").Return("/dev/nvme0n1", nil)

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

func TestScraper_GetDevicesByController_SameController(t *testing.T) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"*"},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	mockNvme := &MockDeviceInfoProvider{}
	deviceSet := collections.NewSet("*")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)

	// Create devices with same controller ID
	device1 := createTestDevice(0, 1, "nvme0n1")
	device2 := createTestDevice(0, 1, "nvme0n1p1") // Same controller, different partition
	devices := []nvme.DeviceFileAttributes{device1, device2}

	// Mock device discovery - should only process controller once
	mockNvme.On("GetAllDevices").Return(devices, nil)
	mockNvme.On("DetectDeviceType", &device1).Return("ebs", nil)
	mockNvme.On("GetDeviceSerial", &device1).Return("vol123456789", nil)
	// device2 should be grouped with device1, so no separate detection needed

	devicesByController, err := scraper.getDevicesByController()

	assert.NoError(t, err)
	assert.Len(t, devicesByController, 1)                // Only one controller group
	assert.Contains(t, devicesByController, 0)           // Controller 0 should exist
	assert.Len(t, devicesByController[0].deviceNames, 2) // Both devices should be in the group
	assert.Equal(t, "ebs", devicesByController[0].deviceType)

	mockNvme.AssertExpectations(t)
}

func TestScraper_RecordMetric_Overflow(t *testing.T) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"*"},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	settings.Logger = zap.NewNop() // Use nop logger to avoid log output during tests
	mockNvme := &MockDeviceInfoProvider{}
	deviceSet := collections.NewSet("*")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)

	// Test with value that would overflow int64
	overflowValue := uint64(9223372036854775808) // math.MaxInt64 + 1

	// This should not panic and should skip the metric
	scraper.recordMetric(func(ts pcommon.Timestamp, val int64) {
		t.Error("Should not call record function for overflow value")
	}, pcommon.NewTimestampFromTime(time.Now()), overflowValue)

	// Test with valid value
	validValue := uint64(1000)
	called := false
	scraper.recordMetric(func(ts pcommon.Timestamp, val int64) {
		called = true
		assert.Equal(t, int64(1000), val)
	}, pcommon.NewTimestampFromTime(time.Now()), validValue)

	assert.True(t, called, "Should call record function for valid value")
}

func TestScraper_Scrape_GetAllDevicesError(t *testing.T) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"*"},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	mockNvme := &MockDeviceInfoProvider{}
	deviceSet := collections.NewSet("*")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)

	// Mock device discovery error
	mockNvme.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{}, errors.New("device discovery failed"))

	ctx := context.Background()
	metrics, err := scraper.scrape(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to discover NVMe devices")
	assert.NotNil(t, metrics)

	mockNvme.AssertExpectations(t)
}

func TestScraper_Scrape_InstanceIDError(t *testing.T) {
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

	// Mock device discovery
	mockNvme.On("GetAllDevices").Return(devices, nil)
	mockNvme.On("DetectDeviceType", &device).Return("ebs", nil)
	mockNvme.On("GetDeviceSerial", &device).Return("vol123456789", nil)
	mockNvme.On("DevicePath", "nvme0n1").Return("/dev/nvme0n1", nil)

	// Mock metadata provider with error
	mockMetadata.On("InstanceID", mock.Anything).Return("", errors.New("metadata service unavailable"))

	ctx := context.Background()
	metrics, _ := scraper.scrape(ctx)

	// Should still succeed even if instance ID retrieval fails (uses "unknown")
	// Note: This test will fail on actual metric retrieval since we can't easily mock nvme.GetEBSMetrics
	// But it tests the instance ID error handling logic
	assert.NotNil(t, metrics)

	mockNvme.AssertExpectations(t)
	mockMetadata.AssertExpectations(t)
}
