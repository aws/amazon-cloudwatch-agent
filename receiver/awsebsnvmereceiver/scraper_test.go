// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsebsnvmereceiver

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
	"github.com/aws/amazon-cloudwatch-agent/receiver/awsebsnvmereceiver/internal/nvme"
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

func (m *mockNvmeUtil) IsEbsDevice(device *nvme.DeviceFileAttributes) (bool, error) {
	args := m.Called(device)
	return args.Bool(0), args.Error(1)
}

func (m *mockNvmeUtil) DevicePath(device string) (string, error) {
	args := m.Called(device)
	return args.String(0), args.Error(1)
}

// mockGetMetrics is a mock function for nvme.GetMetrics
func mockGetMetrics(_ string) (nvme.EBSMetrics, error) {
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

// mockGetMetricsError is a mock function that always returns an error
func mockGetMetricsError(_ string) (nvme.EBSMetrics, error) {
	return nvme.EBSMetrics{}, errors.New("failed to get metrics")
}

func TestScraper_Start(t *testing.T) {
	mockUtil := new(mockNvmeUtil)
	scraper := newScraper(createTestReceiverConfig(), receivertest.NewNopSettings(component.MustNewType("awsebsnvmereceiver")), mockUtil, collections.NewSet[string]())

	err := scraper.start(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err)
}

func TestScraper_Shutdown(t *testing.T) {
	mockUtil := new(mockNvmeUtil)
	scraper := newScraper(createTestReceiverConfig(), receivertest.NewNopSettings(component.MustNewType("awsebsnvmereceiver")), mockUtil, collections.NewSet[string]())

	err := scraper.shutdown(context.Background())
	assert.NoError(t, err)
}

func TestScraper_Scrape_NoDevices(t *testing.T) {
	mockUtil := new(mockNvmeUtil)
	mockUtil.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{}, nil)

	scraper := newScraper(createTestReceiverConfig(), receivertest.NewNopSettings(component.MustNewType("awsebsnvmereceiver")), mockUtil, collections.NewSet[string]())

	metrics, err := scraper.scrape(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())

	mockUtil.AssertExpectations(t)
}

func TestScraper_Scrape_GetAllDevicesError(t *testing.T) {
	mockUtil := new(mockNvmeUtil)
	mockUtil.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{}, errors.New("failed to get devices"))

	scraper := newScraper(createTestReceiverConfig(), receivertest.NewNopSettings(component.MustNewType("awsebsnvmereceiver")), mockUtil, collections.NewSet[string]())

	_, err := scraper.scrape(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get devices")

	mockUtil.AssertExpectations(t)
}

func TestScraper_Scrape_Success(t *testing.T) {
	t.Cleanup(func() {
		getMetrics = nvme.GetMetrics
	})
	getMetrics = mockGetMetrics

	device1, err := nvme.ParseNvmeDeviceFileName("nvme0n1")
	require.NoError(t, err)

	mockUtil := new(mockNvmeUtil)
	mockUtil.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{device1}, nil)
	mockUtil.On("IsEbsDevice", &device1).Return(true, nil)
	mockUtil.On("GetDeviceSerial", &device1).Return("vol1234567890abcdef", nil)
	mockUtil.On("DevicePath", "nvme0n1").Return("/dev/nvme0n1", nil)

	// Allow all devices with empty map
	scraper := newScraper(createTestReceiverConfig(), receivertest.NewNopSettings(component.MustNewType("awsebsnvmereceiver")), mockUtil, collections.NewSet[string]("*"))

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

func TestScraper_Scrape_NonEbsDevice(t *testing.T) {
	device1, err := nvme.ParseNvmeDeviceFileName("nvme0n1")
	require.NoError(t, err)

	mockUtil := new(mockNvmeUtil)
	mockUtil.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{device1}, nil)
	mockUtil.On("IsEbsDevice", &device1).Return(false, nil)

	scraper := newScraper(createTestReceiverConfig(), receivertest.NewNopSettings(component.MustNewType("awsebsnvmereceiver")), mockUtil, collections.NewSet[string]("*"))

	metrics, err := scraper.scrape(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())

	mockUtil.AssertExpectations(t)
}

func TestScraper_Scrape_IsEbsDeviceError(t *testing.T) {
	device1, err := nvme.ParseNvmeDeviceFileName("nvme0n1")
	require.NoError(t, err)

	mockUtil := new(mockNvmeUtil)
	mockUtil.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{device1}, nil)
	mockUtil.On("IsEbsDevice", &device1).Return(false, errors.New("failed to check device"))

	scraper := newScraper(createTestReceiverConfig(), receivertest.NewNopSettings(component.MustNewType("awsebsnvmereceiver")), mockUtil, collections.NewSet[string]("*"))

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
	mockUtil.On("IsEbsDevice", &device1).Return(true, nil)
	mockUtil.On("GetDeviceSerial", &device1).Return("", errors.New("failed to get serial"))

	scraper := newScraper(createTestReceiverConfig(), receivertest.NewNopSettings(component.MustNewType("awsebsnvmereceiver")), mockUtil, collections.NewSet[string]("*"))

	metrics, err := scraper.scrape(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())

	mockUtil.AssertExpectations(t)
}

func TestScraper_Scrape_InvalidSerialPrefix(t *testing.T) {
	device1, err := nvme.ParseNvmeDeviceFileName("nvme0n1")
	require.NoError(t, err)

	mockUtil := new(mockNvmeUtil)
	mockUtil.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{device1}, nil)
	mockUtil.On("IsEbsDevice", &device1).Return(true, nil)
	mockUtil.On("GetDeviceSerial", &device1).Return("invalid-serial", nil)

	scraper := newScraper(createTestReceiverConfig(), receivertest.NewNopSettings(component.MustNewType("awsebsnvmereceiver")), mockUtil, collections.NewSet[string]("*"))

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
	mockUtil.On("IsEbsDevice", &device1).Return(true, nil)
	mockUtil.On("GetDeviceSerial", &device1).Return("vol1234567890abcdef", nil)
	mockUtil.On("DevicePath", "nvme0n1").Return("/dev/nvme0n1", nil)

	// Create a test logger to capture log messages
	core, observedLogs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)

	settings := receivertest.NewNopSettings(component.MustNewType("awsebsnvmereceiver"))
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

func TestScraper_Scrape_MultipleDevices(t *testing.T) {
	t.Cleanup(func() {
		getMetrics = nvme.GetMetrics
	})
	getMetrics = mockGetMetrics

	device1, err := nvme.ParseNvmeDeviceFileName("nvme0n1")
	require.NoError(t, err)

	device2, err := nvme.ParseNvmeDeviceFileName("nvme1n1")
	require.NoError(t, err)

	mockUtil := new(mockNvmeUtil)
	mockUtil.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{device1, device2}, nil)
	mockUtil.On("IsEbsDevice", &device1).Return(true, nil)
	mockUtil.On("GetDeviceSerial", &device1).Return("vol1234567890abcdef", nil)
	mockUtil.On("DevicePath", "nvme0n1").Return("/dev/nvme0n1", nil)
	mockUtil.On("IsEbsDevice", &device2).Return(true, nil)
	mockUtil.On("GetDeviceSerial", &device2).Return("vol0987654321fedcba", nil)
	mockUtil.On("DevicePath", "nvme1n1").Return("/dev/nvme1n1", nil)

	scraper := newScraper(createTestReceiverConfig(), receivertest.NewNopSettings(component.MustNewType("awsebsnvmereceiver")), mockUtil, collections.NewSet[string]("*"))

	metrics, err := scraper.scrape(context.Background())
	assert.NoError(t, err)

	assert.Equal(t, 2, metrics.ResourceMetrics().Len())

	mockUtil.AssertExpectations(t)
}

func TestScraper_Scrape_FilteredDevices(t *testing.T) {
	t.Cleanup(func() {
		getMetrics = nvme.GetMetrics
	})
	getMetrics = mockGetMetrics

	device1, err := nvme.ParseNvmeDeviceFileName("nvme0n1")
	require.NoError(t, err)

	device2, err := nvme.ParseNvmeDeviceFileName("nvme1n1")
	require.NoError(t, err)

	mockUtil := new(mockNvmeUtil)
	mockUtil.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{device1, device2}, nil)

	mockUtil.On("IsEbsDevice", &device1).Return(true, nil)
	mockUtil.On("GetDeviceSerial", &device1).Return("vol0987654321fedcba", nil)
	mockUtil.On("DevicePath", "nvme0n1").Return("/dev/nvme0n1", nil)

	core, observedLogs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)

	settings := receivertest.NewNopSettings(component.MustNewType("awsebsnvmereceiver"))
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
	getMetrics = mockGetMetrics

	device1, err := nvme.ParseNvmeDeviceFileName("nvme0n1")
	require.NoError(t, err)

	device2, err := nvme.ParseNvmeDeviceFileName("nvme0n1p1")
	require.NoError(t, err)

	mockUtil := new(mockNvmeUtil)
	mockUtil.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{device1, device2}, nil)
	mockUtil.On("IsEbsDevice", &device1).Return(true, nil)
	mockUtil.On("GetDeviceSerial", &device1).Return("vol1234567890abcdef", nil)
	mockUtil.On("DevicePath", "nvme0n1").Return("/dev/nvme0n1", nil)

	core, observedLogs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)

	settings := receivertest.NewNopSettings(component.MustNewType("awsebsnvmereceiver"))
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
	mockUtil.On("IsEbsDevice", &device1).Return(true, nil)
	mockUtil.On("GetDeviceSerial", &device1).Return("vol1234567890abcdef", nil)
	mockUtil.On("DevicePath", "nvme0n1").Return("", errors.New("device path error"))

	core, observedLogs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)

	settings := receivertest.NewNopSettings(component.MustNewType("awsebsnvmereceiver"))
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
