// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsebsnvmereceiver

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/aws/amazon-cloudwatch-agent/receiver/awsebsnvmereceiver/internal/nvme"
)

// mockNvmeUtil is a mock implementation of the NvmeUtilInterface
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

// mockGetMetrics is a mock function for nvme.GetMetrics
func mockGetMetrics(devicePath string) (nvme.EBSMetrics, error) {
	if devicePath == "/dev/nvme0n1" {
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
	return nvme.EBSMetrics{}, errors.New("device not found")
}

// mockGetMetricsError is a mock function that always returns an error
func mockGetMetricsError(_ string) (nvme.EBSMetrics, error) {
	return nvme.EBSMetrics{}, errors.New("failed to get metrics")
}

func TestScraper_Start(t *testing.T) {
	mockUtil := new(mockNvmeUtil)
	scraper := newScraper(createDefaultConfig().(*Config), receivertest.NewNopSettings(), mockUtil)

	err := scraper.start(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err)
}

func TestScraper_Shutdown(t *testing.T) {
	mockUtil := new(mockNvmeUtil)
	scraper := newScraper(createDefaultConfig().(*Config), receivertest.NewNopSettings(), mockUtil)

	err := scraper.shutdown(context.Background())
	assert.NoError(t, err)
}

func TestScraper_Scrape_NoDevices(t *testing.T) {
	mockUtil := new(mockNvmeUtil)
	mockUtil.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{}, nil)

	scraper := newScraper(createDefaultConfig().(*Config), receivertest.NewNopSettings(), mockUtil)

	metrics, err := scraper.scrape(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())

	mockUtil.AssertExpectations(t)
}

func TestScraper_Scrape_GetAllDevicesError(t *testing.T) {
	mockUtil := new(mockNvmeUtil)
	mockUtil.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{}, errors.New("failed to get devices"))

	scraper := newScraper(createDefaultConfig().(*Config), receivertest.NewNopSettings(), mockUtil)

	_, err := scraper.scrape(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get devices")

	mockUtil.AssertExpectations(t)
}

func TestScraper_Scrape_Success(t *testing.T) {
	originalGetMetrics := getMetrics
	defer func() { getMetrics = originalGetMetrics }()
	getMetrics = mockGetMetrics

	// Create device attributes
	device1, err := nvme.ParseNvmeDeviceFileName("nvme0n1")
	require.NoError(t, err)

	mockUtil := new(mockNvmeUtil)
	mockUtil.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{device1}, nil)
	mockUtil.On("IsEbsDevice", &device1).Return(true, nil)
	mockUtil.On("GetDeviceSerial", &device1).Return("vol1234567890abcdef", nil)

	scraper := newScraper(createDefaultConfig().(*Config), receivertest.NewNopSettings(), mockUtil)

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

	scraper := newScraper(createDefaultConfig().(*Config), receivertest.NewNopSettings(), mockUtil)

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

	scraper := newScraper(createDefaultConfig().(*Config), receivertest.NewNopSettings(), mockUtil)

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

	scraper := newScraper(createDefaultConfig().(*Config), receivertest.NewNopSettings(), mockUtil)

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

	scraper := newScraper(createDefaultConfig().(*Config), receivertest.NewNopSettings(), mockUtil)

	metrics, err := scraper.scrape(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())

	mockUtil.AssertExpectations(t)
}

func TestScraper_Scrape_GetMetricsError(t *testing.T) {
	originalGetMetrics := getMetrics
	defer func() { getMetrics = originalGetMetrics }()
	getMetrics = mockGetMetricsError

	device1, err := nvme.ParseNvmeDeviceFileName("nvme0n1")
	require.NoError(t, err)

	mockUtil := new(mockNvmeUtil)
	mockUtil.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{device1}, nil)
	mockUtil.On("IsEbsDevice", &device1).Return(true, nil)
	mockUtil.On("GetDeviceSerial", &device1).Return("vol1234567890abcdef", nil)

	// Create a test logger to capture log messages
	core, observedLogs := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)

	settings := receivertest.NewNopSettings()
	settings.TelemetrySettings.Logger = logger

	scraper := newScraper(createDefaultConfig().(*Config), settings, mockUtil)

	metrics, err := scraper.scrape(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())

	// Verify log message
	assert.Equal(t, 1, observedLogs.Len())
	assert.Contains(t, observedLogs.All()[0].Message, "unable to get metrics for device")

	mockUtil.AssertExpectations(t)
}

func TestScraper_Scrape_MultipleDevices(t *testing.T) {
	originalGetMetrics := getMetrics
	defer func() { getMetrics = originalGetMetrics }()
	getMetrics = mockGetMetrics

	// Create device attributes
	device1, err := nvme.ParseNvmeDeviceFileName("nvme0n1")
	require.NoError(t, err)

	device2, err := nvme.ParseNvmeDeviceFileName("nvme1n1")
	require.NoError(t, err)

	mockUtil := new(mockNvmeUtil)
	mockUtil.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{device1, device2}, nil)
	mockUtil.On("IsEbsDevice", &device1).Return(true, nil)
	mockUtil.On("GetDeviceSerial", &device1).Return("vol1234567890abcdef", nil)
	mockUtil.On("IsEbsDevice", &device2).Return(true, nil)
	mockUtil.On("GetDeviceSerial", &device2).Return("vol0987654321fedcba", nil)

	scraper := newScraper(createDefaultConfig().(*Config), receivertest.NewNopSettings(), mockUtil)

	metrics, err := scraper.scrape(context.Background())
	assert.NoError(t, err)

	// Only device1 should produce metrics since our mock only handles /dev/nvme0n1
	assert.Equal(t, 1, metrics.ResourceMetrics().Len())

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
