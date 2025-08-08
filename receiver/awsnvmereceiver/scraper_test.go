// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsnvmereceiver

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	"github.com/aws/amazon-cloudwatch-agent/receiver/awsnvmereceiver/internal/nvme"
)

// mockNvmeUtil is a mock implementation of the DeviceInfoProvider
type mockNvmeUtil struct {
	mock.Mock
}

func (m *mockNvmeUtil) GetAllDevices() ([]nvme.DeviceFileAttributes, error) {
	args := m.Called()
	return args.Get(0).([]nvme.DeviceFileAttributes), args.Error(1)
}

func (m *mockNvmeUtil) GetDeviceSerial(device *nvme.DeviceFileAttributes) (string, error) {
	args := m.Called(device)
	return args.String(0), args.Error(1)
}

func (m *mockNvmeUtil) GetDeviceModel(device *nvme.DeviceFileAttributes) (string, error) {
	args := m.Called(device)
	return args.String(0), args.Error(1)
}

func (m *mockNvmeUtil) DevicePath(device string) (string, error) {
	args := m.Called(device)
	return args.String(0), args.Error(1)
}

// mockGetEBSMetrics is a mock function for nvme.GetMetrics that returns EBS metrics
func mockGetEBSMetrics(_ string) (any, error) {
	return nvme.EBSMetrics{
		EBSMagic:              0x3C23B510,
		ReadOps:               100,
		WriteOps:              200,
		ReadBytes:             1024,
		WriteBytes:            2048,
		TotalReadTime:         500,
		TotalWriteTime:        600,
		EBSIOPSExceeded:       1,
		EBSThroughputExceeded: 2,
		EC2IOPSExceeded:       3,
		EC2ThroughputExceeded: 4,
		QueueLength:           5,
	}, nil
}

// mockGetInstanceStoreMetrics is a mock function for nvme.GetMetrics that returns Instance Store metrics
func mockGetInstanceStoreMetrics(_ string) (any, error) {
	return nvme.InstanceStoreMetrics{
		Magic:                 0xEC2C0D7E,
		ReadOps:               150,
		WriteOps:              250,
		ReadBytes:             1536,
		WriteBytes:            2560,
		TotalReadTime:         750,
		TotalWriteTime:        850,
		EC2IOPSExceeded:       6,
		EC2ThroughputExceeded: 7,
		QueueLength:           8,
	}, nil
}

// mockGetMetricsError is a mock function that always returns an error
func mockGetMetricsError(_ string) (any, error) {
	return nil, errors.New("failed to get metrics")
}

func TestScraper_Start(t *testing.T) {
	mockUtil := new(mockNvmeUtil)
	scraper := newScraper(createTestReceiverConfig(), receivertest.NewNopSettings(component.MustNewType("awsnvmereceiver")), mockUtil, collections.NewSet[string]())

	err := scraper.start(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err)
}

func TestScraper_Shutdown(t *testing.T) {
	mockUtil := new(mockNvmeUtil)
	scraper := newScraper(createTestReceiverConfig(), receivertest.NewNopSettings(component.MustNewType("awsnvmereceiver")), mockUtil, collections.NewSet[string]())

	err := scraper.shutdown(context.Background())
	assert.NoError(t, err)
}

func TestScraper_Scrape_NoDevices(t *testing.T) {
	mockUtil := new(mockNvmeUtil)
	mockUtil.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{}, nil)

	scraper := newScraper(createTestReceiverConfig(), receivertest.NewNopSettings(component.MustNewType("awsnvmereceiver")), mockUtil, collections.NewSet[string]())

	metrics, err := scraper.scrape(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())

	mockUtil.AssertExpectations(t)
}

func TestScraper_Scrape_GetAllDevicesError(t *testing.T) {
	mockUtil := new(mockNvmeUtil)
	mockUtil.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{}, errors.New("failed to get devices"))

	scraper := newScraper(createTestReceiverConfig(), receivertest.NewNopSettings(component.MustNewType("awsnvmereceiver")), mockUtil, collections.NewSet[string]())

	_, err := scraper.scrape(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get devices")

	mockUtil.AssertExpectations(t)
}

func TestScraper_Scrape_EBSSuccess(t *testing.T) {
	t.Cleanup(func() {
		getMetrics = nvme.GetMetrics
	})
	getMetrics = mockGetEBSMetrics

	device1, err := nvme.ParseNvmeDeviceFileName("nvme0n1")
	require.NoError(t, err)

	mockUtil := new(mockNvmeUtil)
	mockUtil.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{device1}, nil)
	mockUtil.On("GetDeviceSerial", &device1).Return("vol1234567890abcdef", nil)
	mockUtil.On("GetDeviceModel", &device1).Return("Amazon Elastic Block Store", nil)
	mockUtil.On("DevicePath", "nvme0n1").Return("/dev/nvme0n1", nil)

	// Allow all devices with empty map
	scraper := newScraper(createTestReceiverConfig(), receivertest.NewNopSettings(component.MustNewType("awsnvmereceiver")), mockUtil, collections.NewSet[string]("*"))

	metrics, err := scraper.scrape(context.Background())
	assert.NoError(t, err)

	// Verify metrics
	assert.Equal(t, 1, metrics.ResourceMetrics().Len())

	rm := metrics.ResourceMetrics().At(0)
	assert.Equal(t, "vol-1234567890abcdef", rm.Resource().Attributes().AsRaw()["VolumeId"])

	// Check metric values
	ilm := rm.ScopeMetrics().At(0).Metrics()
	assert.Equal(t, 11, ilm.Len()) // We expect 11 metrics based on the scraper implementation

	// Verify specific metrics
	verifySumMetric(t, ilm, "diskio_ebs_total_read_ops", 100)
	verifySumMetric(t, ilm, "diskio_ebs_total_write_ops", 200)
	verifySumMetric(t, ilm, "diskio_ebs_total_read_bytes", 1024)
	verifySumMetric(t, ilm, "diskio_ebs_total_write_bytes", 2048)
	verifySumMetric(t, ilm, "diskio_ebs_total_read_time", 500)
	verifySumMetric(t, ilm, "diskio_ebs_total_write_time", 600)
	verifySumMetric(t, ilm, "diskio_ebs_volume_performance_exceeded_iops", 1)
	verifySumMetric(t, ilm, "diskio_ebs_volume_performance_exceeded_tp", 2)
	verifySumMetric(t, ilm, "diskio_ebs_ec2_instance_performance_exceeded_iops", 3)
	verifySumMetric(t, ilm, "diskio_ebs_ec2_instance_performance_exceeded_tp", 4)
	verifyGaugeMetric(t, ilm, "diskio_ebs_volume_queue_length", 5)

	mockUtil.AssertExpectations(t)
}

func TestScraper_Scrape_InstanceStoreSuccess(t *testing.T) {
	t.Cleanup(func() {
		getMetrics = nvme.GetMetrics
	})
	getMetrics = mockGetInstanceStoreMetrics

	device1, err := nvme.ParseNvmeDeviceFileName("nvme0n1")
	require.NoError(t, err)

	mockUtil := new(mockNvmeUtil)
	mockUtil.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{device1}, nil)
	mockUtil.On("GetDeviceSerial", &device1).Return("AWS1234567890abcdef0", nil)
	mockUtil.On("GetDeviceModel", &device1).Return("Amazon EC2 NVMe Instance Storage", nil)
	mockUtil.On("DevicePath", "nvme0n1").Return("/dev/nvme0n1", nil)

	// Allow all devices
	scraper := newScraper(createTestReceiverConfig(), receivertest.NewNopSettings(component.MustNewType("awsnvmereceiver")), mockUtil, collections.NewSet[string]("*"))

	metrics, err := scraper.scrape(context.Background())
	assert.NoError(t, err)

	// Verify metrics
	assert.Equal(t, 1, metrics.ResourceMetrics().Len())

	rm := metrics.ResourceMetrics().At(0)
	assert.Equal(t, "AWS1234567890abcdef0", rm.Resource().Attributes().AsRaw()["SerialId"])

	// Check metric values
	ilm := rm.ScopeMetrics().At(0).Metrics()
	assert.Equal(t, 9, ilm.Len()) // We expect 9 metrics for Instance Store

	// Verify specific metrics
	verifySumMetric(t, ilm, "diskio_instance_store_total_read_ops", 150)
	verifySumMetric(t, ilm, "diskio_instance_store_total_write_ops", 250)
	verifySumMetric(t, ilm, "diskio_instance_store_total_read_bytes", 1536)
	verifySumMetric(t, ilm, "diskio_instance_store_total_write_bytes", 2560)
	verifySumMetric(t, ilm, "diskio_instance_store_total_read_time", 750)
	verifySumMetric(t, ilm, "diskio_instance_store_total_write_time", 850)
	verifySumMetric(t, ilm, "diskio_instance_store_performance_exceeded_iops", 6)
	verifySumMetric(t, ilm, "diskio_instance_store_performance_exceeded_tp", 7)
	verifyGaugeMetric(t, ilm, "diskio_instance_store_volume_queue_length", 8)

	mockUtil.AssertExpectations(t)
}

func TestScraper_Scrape_UnknownDeviceModel(t *testing.T) {
	device1, err := nvme.ParseNvmeDeviceFileName("nvme0n1")
	require.NoError(t, err)

	mockUtil := new(mockNvmeUtil)
	mockUtil.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{device1}, nil)
	mockUtil.On("GetDeviceSerial", &device1).Return("unknown-serial", nil)
	mockUtil.On("GetDeviceModel", &device1).Return("Unknown Storage Device", nil)

	scraper := newScraper(createTestReceiverConfig(), receivertest.NewNopSettings(component.MustNewType("awsnvmereceiver")), mockUtil, collections.NewSet[string]("*"))

	metrics, err := scraper.scrape(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())

	mockUtil.AssertExpectations(t)
}

func TestScraper_Scrape_GetDeviceModelError(t *testing.T) {
	device1, err := nvme.ParseNvmeDeviceFileName("nvme0n1")
	require.NoError(t, err)

	mockUtil := new(mockNvmeUtil)
	mockUtil.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{device1}, nil)
	mockUtil.On("GetDeviceSerial", &device1).Return("vol1234567890abcdef", nil)
	mockUtil.On("GetDeviceModel", &device1).Return("", errors.New("failed to get device model"))

	scraper := newScraper(createTestReceiverConfig(), receivertest.NewNopSettings(component.MustNewType("awsnvmereceiver")), mockUtil, collections.NewSet[string]("*"))

	metrics, err := scraper.scrape(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())

	mockUtil.AssertExpectations(t)
}

func TestScraper_Scrape_GetDeviceSerialError(t *testing.T) {
	device1, err := nvme.ParseNvmeDeviceFileName("nvme0n1")
	require.NoError(t, err)

	mockUtil := new(mockNvmeUtil)
	mockUtil.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{device1}, nil)
	mockUtil.On("GetDeviceSerial", &device1).Return("", errors.New("failed to get serial"))

	scraper := newScraper(createTestReceiverConfig(), receivertest.NewNopSettings(component.MustNewType("awsnvmereceiver")), mockUtil, collections.NewSet[string]("*"))

	metrics, err := scraper.scrape(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())

	mockUtil.AssertExpectations(t)
}

func TestScraper_Scrape_InvalidEBSSerialPrefix(t *testing.T) {
	device1, err := nvme.ParseNvmeDeviceFileName("nvme0n1")
	require.NoError(t, err)

	mockUtil := new(mockNvmeUtil)
	mockUtil.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{device1}, nil)
	mockUtil.On("GetDeviceSerial", &device1).Return("invalid-serial", nil)
	mockUtil.On("GetDeviceModel", &device1).Return("Amazon Elastic Block Store", nil)

	scraper := newScraper(createTestReceiverConfig(), receivertest.NewNopSettings(component.MustNewType("awsnvmereceiver")), mockUtil, collections.NewSet[string]("*"))

	metrics, err := scraper.scrape(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())

	mockUtil.AssertExpectations(t)
}

func TestScraper_Scrape_GetMetricsError(t *testing.T) {
	t.Cleanup(func() {
		getMetrics = nvme.GetMetrics
	})
	getMetrics = mockGetMetricsError

	device1, err := nvme.ParseNvmeDeviceFileName("nvme0n1")
	require.NoError(t, err)

	mockUtil := new(mockNvmeUtil)
	mockUtil.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{device1}, nil)
	mockUtil.On("GetDeviceSerial", &device1).Return("vol1234567890abcdef", nil)
	mockUtil.On("GetDeviceModel", &device1).Return("Amazon Elastic Block Store", nil)
	mockUtil.On("DevicePath", "nvme0n1").Return("/dev/nvme0n1", nil)

	// Create a test logger to capture log messages
	core, observedLogs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)

	settings := receivertest.NewNopSettings(component.MustNewType("awsnvmereceiver"))
	settings.TelemetrySettings.Logger = logger

	scraper := newScraper(createTestReceiverConfig(), settings, mockUtil, collections.NewSet[string]("*"))

	metrics, err := scraper.scrape(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())

	foundLogMessage := false
	for _, log := range observedLogs.All() {
		if log.Message == "unable to get metrics for device" {
			foundLogMessage = true
			break
		}
	}
	assert.True(t, foundLogMessage, "Expected to find log about unable to get metrics")

	mockUtil.AssertExpectations(t)
}

func TestScraper_Scrape_MultipleDevices_EBSAndInstanceStore(t *testing.T) {
	t.Cleanup(func() {
		getMetrics = nvme.GetMetrics
	})

	// Mock function that returns different metrics based on device path
	getMetrics = func(devicePath string) (any, error) {
		if devicePath == "/dev/nvme0n1" {
			return mockGetEBSMetrics(devicePath)
		} else if devicePath == "/dev/nvme1n1" {
			return mockGetInstanceStoreMetrics(devicePath)
		}
		return nil, errors.New("unknown device")
	}

	device1, err := nvme.ParseNvmeDeviceFileName("nvme0n1")
	require.NoError(t, err)

	device2, err := nvme.ParseNvmeDeviceFileName("nvme1n1")
	require.NoError(t, err)

	mockUtil := new(mockNvmeUtil)
	mockUtil.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{device1, device2}, nil)
	mockUtil.On("GetDeviceSerial", &device1).Return("vol1234567890abcdef", nil)
	mockUtil.On("GetDeviceModel", &device1).Return("Amazon Elastic Block Store", nil)
	mockUtil.On("DevicePath", "nvme0n1").Return("/dev/nvme0n1", nil)
	mockUtil.On("GetDeviceSerial", &device2).Return("AWS0987654321fedcba0", nil)
	mockUtil.On("GetDeviceModel", &device2).Return("Amazon EC2 NVMe Instance Storage", nil)
	mockUtil.On("DevicePath", "nvme1n1").Return("/dev/nvme1n1", nil)

	scraper := newScraper(createTestReceiverConfig(), receivertest.NewNopSettings(component.MustNewType("awsnvmereceiver")), mockUtil, collections.NewSet[string]("*"))

	metrics, err := scraper.scrape(context.Background())
	assert.NoError(t, err)

	assert.Equal(t, 2, metrics.ResourceMetrics().Len())

	// Verify we have both EBS and Instance Store metrics
	foundEBS := false
	foundInstanceStore := false

	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		attrs := rm.Resource().Attributes().AsRaw()

		if _, hasVolumeID := attrs["VolumeId"]; hasVolumeID {
			foundEBS = true
			assert.Equal(t, "vol-1234567890abcdef", attrs["VolumeId"])
		}

		if _, hasSerialID := attrs["SerialId"]; hasSerialID {
			foundInstanceStore = true
			assert.Equal(t, "AWS0987654321fedcba0", attrs["SerialId"])
		}
	}

	assert.True(t, foundEBS, "Expected to find EBS metrics")
	assert.True(t, foundInstanceStore, "Expected to find Instance Store metrics")

	mockUtil.AssertExpectations(t)
}

func TestScraper_Scrape_FilteredDevices(t *testing.T) {
	t.Cleanup(func() {
		getMetrics = nvme.GetMetrics
	})
	getMetrics = mockGetEBSMetrics

	device1, err := nvme.ParseNvmeDeviceFileName("nvme0n1")
	require.NoError(t, err)

	device2, err := nvme.ParseNvmeDeviceFileName("nvme1n1")
	require.NoError(t, err)

	mockUtil := new(mockNvmeUtil)
	mockUtil.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{device1, device2}, nil)

	mockUtil.On("GetDeviceSerial", &device1).Return("vol0987654321fedcba", nil)
	mockUtil.On("GetDeviceModel", &device1).Return("Amazon Elastic Block Store", nil)
	mockUtil.On("DevicePath", "nvme0n1").Return("/dev/nvme0n1", nil)

	core, observedLogs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)

	settings := receivertest.NewNopSettings(component.MustNewType("awsnvmereceiver"))
	settings.TelemetrySettings.Logger = logger

	// Only allow nvme0n1
	scraper := newScraper(createTestReceiverConfig(), settings, mockUtil, collections.NewSet[string]("nvme0n1"))

	metrics, err := scraper.scrape(context.Background())
	assert.NoError(t, err)

	// We should get one set of metrics because of nvme0n1
	assert.Equal(t, 1, metrics.ResourceMetrics().Len())

	// Verify that we logged about skipping nvme1n1
	foundSkipLog := false
	for _, log := range observedLogs.All() {
		if log.Message == "skipping un-allowed device" && log.ContextMap()["device"] == "nvme1n1" {
			foundSkipLog = true
			break
		}
	}
	assert.True(t, foundSkipLog, "Expected to find log about skipping un-allowed device")

	mockUtil.AssertExpectations(t)
}

func TestScraper_Scrape_MultipleDevicesSameController(t *testing.T) {
	t.Cleanup(func() {
		getMetrics = nvme.GetMetrics
	})
	getMetrics = mockGetEBSMetrics

	device1, err := nvme.ParseNvmeDeviceFileName("nvme0n1")
	require.NoError(t, err)

	device2, err := nvme.ParseNvmeDeviceFileName("nvme0n1p1")
	require.NoError(t, err)

	mockUtil := new(mockNvmeUtil)
	mockUtil.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{device1, device2}, nil)
	mockUtil.On("GetDeviceSerial", &device1).Return("vol1234567890abcdef", nil)
	mockUtil.On("GetDeviceModel", &device1).Return("Amazon Elastic Block Store", nil)
	mockUtil.On("DevicePath", "nvme0n1").Return("/dev/nvme0n1", nil)

	core, observedLogs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)

	settings := receivertest.NewNopSettings(component.MustNewType("awsnvmereceiver"))
	settings.TelemetrySettings.Logger = logger

	scraper := newScraper(createTestReceiverConfig(), settings, mockUtil, collections.NewSet[string]("*"))

	metrics, err := scraper.scrape(context.Background())
	assert.NoError(t, err)

	// Should only get one set of metrics for the controller
	assert.Equal(t, 1, metrics.ResourceMetrics().Len())

	// Verify that we logged about skipping unnecessary validation for the second device
	foundSkipLog := false
	for _, log := range observedLogs.All() {
		if log.Message == "skipping unnecessary device validation steps" && log.ContextMap()["device"] == "nvme0n1p1" {
			foundSkipLog = true
			break
		}
	}
	assert.True(t, foundSkipLog, "Expected to find log about skipping unnecessary device validation steps")

	mockUtil.AssertExpectations(t)
}

func verifySumMetric(t *testing.T, metrics pmetric.MetricSlice, name string, expectedValue int64) {
	for i := 0; i < metrics.Len(); i++ {
		metric := metrics.At(i)
		if metric.Name() == name {
			assert.Equal(t, expectedValue, metric.Sum().DataPoints().At(0).IntValue())
			return
		}
	}
	t.Errorf("Metric %s not found", name)
}

func verifyGaugeMetric(t *testing.T, metrics pmetric.MetricSlice, name string, expectedValue int64) {
	for i := 0; i < metrics.Len(); i++ {
		metric := metrics.At(i)
		if metric.Name() == name {
			assert.Equal(t, expectedValue, metric.Gauge().DataPoints().At(0).IntValue())
			return
		}
	}
	t.Errorf("Metric %s not found", name)
}

// Test for the device path error case
func TestScraper_Scrape_DevicePathError(t *testing.T) {
	device1, err := nvme.ParseNvmeDeviceFileName("nvme0n1")
	require.NoError(t, err)

	mockUtil := new(mockNvmeUtil)
	mockUtil.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{device1}, nil)
	mockUtil.On("GetDeviceSerial", &device1).Return("vol1234567890abcdef", nil)
	mockUtil.On("GetDeviceModel", &device1).Return("Amazon Elastic Block Store", nil)
	mockUtil.On("DevicePath", "nvme0n1").Return("", errors.New("device path error"))

	core, observedLogs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)

	settings := receivertest.NewNopSettings(component.MustNewType("awsnvmereceiver"))
	settings.TelemetrySettings.Logger = logger

	scraper := newScraper(createTestReceiverConfig(), settings, mockUtil, collections.NewSet[string]("*"))

	metrics, err := scraper.scrape(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())

	// Verify log message about device path error
	foundLogMessage := false
	for _, log := range observedLogs.All() {
		if log.Message == "unable to get device path" {
			foundLogMessage = true
			break
		}
	}
	assert.True(t, foundLogMessage, "Expected to find log about unable to get device path")

	mockUtil.AssertExpectations(t)
}

func TestScraper_Scrape_InstanceStoreDisabled(t *testing.T) {
	t.Cleanup(func() {
		getMetrics = nvme.GetMetrics
	})
	getMetrics = mockGetInstanceStoreMetrics

	device1, err := nvme.ParseNvmeDeviceFileName("nvme0n1")
	require.NoError(t, err)

	mockUtil := new(mockNvmeUtil)
	mockUtil.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{device1}, nil)
	mockUtil.On("GetDeviceSerial", &device1).Return("AWS1234567890abcdef0", nil)
	mockUtil.On("GetDeviceModel", &device1).Return("Amazon EC2 NVMe Instance Storage", nil)

	// Create config with Instance Store metrics disabled
	cfg := createDefaultConfig().(*Config)
	// Don't enable any Instance Store metrics, only EBS metrics
	cfg.MetricsBuilderConfig.Metrics.DiskioEbsTotalReadOps.Enabled = true

	core, observedLogs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)

	settings := receivertest.NewNopSettings(component.MustNewType("awsnvmereceiver"))
	settings.TelemetrySettings.Logger = logger

	scraper := newScraper(cfg, settings, mockUtil, collections.NewSet[string]("*"))

	metrics, err := scraper.scrape(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())

	// Verify that we logged about skipping Instance Store device
	foundSkipLog := false
	for _, log := range observedLogs.All() {
		if log.Message == "skipping Instance Store device as no IS metrics enabled" {
			foundSkipLog = true
			break
		}
	}
	assert.True(t, foundSkipLog, "Expected to find log about skipping Instance Store device")

	mockUtil.AssertExpectations(t)
}

func TestScraper_Scrape_EBSDisabled(t *testing.T) {
	device1, err := nvme.ParseNvmeDeviceFileName("nvme0n1")
	require.NoError(t, err)

	mockUtil := new(mockNvmeUtil)
	mockUtil.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{device1}, nil)
	mockUtil.On("GetDeviceSerial", &device1).Return("vol1234567890abcdef", nil)
	mockUtil.On("GetDeviceModel", &device1).Return("Amazon Elastic Block Store", nil)

	// Create config with EBS metrics disabled
	cfg := createDefaultConfig().(*Config)
	// Disable all EBS metrics
	cfg.MetricsBuilderConfig.Metrics.DiskioEbsTotalReadOps.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.DiskioEbsTotalWriteOps.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.DiskioEbsTotalReadBytes.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.DiskioEbsTotalWriteBytes.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.DiskioEbsTotalReadTime.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.DiskioEbsTotalWriteTime.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.DiskioEbsVolumePerformanceExceededIops.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.DiskioEbsVolumePerformanceExceededTp.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.DiskioEbsEc2InstancePerformanceExceededIops.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.DiskioEbsEc2InstancePerformanceExceededTp.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.DiskioEbsVolumeQueueLength.Enabled = false
	// Enable only Instance Store metrics
	cfg.MetricsBuilderConfig.Metrics.DiskioInstanceStoreTotalReadOps.Enabled = true

	core, observedLogs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)

	settings := receivertest.NewNopSettings(component.MustNewType("awsnvmereceiver"))
	settings.TelemetrySettings.Logger = logger

	scraper := newScraper(cfg, settings, mockUtil, collections.NewSet[string]("*"))

	metrics, err := scraper.scrape(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())

	// Verify that we logged about skipping EBS device
	foundSkipLog := false
	for _, log := range observedLogs.All() {
		if log.Message == "skipping EBS device as no EBS metrics enabled" {
			foundSkipLog = true
			break
		}
	}
	assert.True(t, foundSkipLog, "Expected to find log about skipping EBS device")

	mockUtil.AssertExpectations(t)
}

func TestScraper_Scrape_UnsupportedMetricsType(t *testing.T) {
	t.Cleanup(func() {
		getMetrics = nvme.GetMetrics
	})

	// Mock function that returns an unsupported metrics type
	getMetrics = func(_ string) (any, error) {
		return "unsupported metrics type", nil
	}

	device1, err := nvme.ParseNvmeDeviceFileName("nvme0n1")
	require.NoError(t, err)

	mockUtil := new(mockNvmeUtil)
	mockUtil.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{device1}, nil)
	mockUtil.On("GetDeviceSerial", &device1).Return("vol1234567890abcdef", nil)
	mockUtil.On("GetDeviceModel", &device1).Return("Amazon Elastic Block Store", nil)
	mockUtil.On("DevicePath", "nvme0n1").Return("/dev/nvme0n1", nil)

	core, observedLogs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)

	settings := receivertest.NewNopSettings(component.MustNewType("awsnvmereceiver"))
	settings.TelemetrySettings.Logger = logger

	scraper := newScraper(createTestReceiverConfig(), settings, mockUtil, collections.NewSet[string]("*"))

	metrics, err := scraper.scrape(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())

	// Verify that we logged about unsupported metrics type
	foundLogMessage := false
	for _, log := range observedLogs.All() {
		if log.Message == "unsupported metrics type" {
			foundLogMessage = true
			break
		}
	}
	assert.True(t, foundLogMessage, "Expected to find log about unsupported metrics type")

	mockUtil.AssertExpectations(t)
}

func createTestReceiverConfig() *Config {
	cfg := createDefaultConfig().(*Config)

	// Use reflection to enable all metrics
	v := reflect.ValueOf(&cfg.MetricsBuilderConfig.Metrics).Elem()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)

		if field.Kind() == reflect.Struct {
			enabledField := field.FieldByName("Enabled")
			if enabledField.IsValid() && enabledField.CanSet() {
				enabledField.SetBool(true)
			}
		}
	}

	return cfg
}
