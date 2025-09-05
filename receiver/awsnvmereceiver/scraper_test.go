// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsnvmereceiver

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"math"
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
	"github.com/aws/amazon-cloudwatch-agent/receiver/awsnvmereceiver/internal/ebs"
	"github.com/aws/amazon-cloudwatch-agent/receiver/awsnvmereceiver/internal/instancestore"
	"github.com/aws/amazon-cloudwatch-agent/receiver/awsnvmereceiver/internal/nvme"
)

// mockNvmeUtil is a mock implementation of the DeviceInfoProvider
type mockNvmeUtil struct {
	mock.Mock
}

func (m *mockNvmeUtil) GetAllDevices() ([]nvme.DeviceFileAttributes, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
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

func TestScraper_Start(t *testing.T) {
	core, observedLogs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)
	settings := receivertest.NewNopSettings(component.MustNewType("awsnvmereceiver"))
	settings.Logger = logger

	mockUtil := new(mockNvmeUtil)
	scraper := newScraper(createTestReceiverConfig(), settings, mockUtil, collections.NewSet[string]())

	err := scraper.start(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err)

	foundLogMessage := false
	for _, log := range observedLogs.All() {
		if log.Message == "Starting NVMe scraper" {
			foundLogMessage = true
			break
		}
	}
	assert.True(t, foundLogMessage, "Expected to find log about starting scraper")
}

func TestScraper_Shutdown(t *testing.T) {
	core, observedLogs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)
	settings := receivertest.NewNopSettings(component.MustNewType("awsnvmereceiver"))
	settings.Logger = logger

	mockUtil := new(mockNvmeUtil)
	scraper := newScraper(createTestReceiverConfig(), settings, mockUtil, collections.NewSet[string]())

	err := scraper.shutdown(context.Background())
	assert.NoError(t, err)

	foundLogMessage := false
	for _, log := range observedLogs.All() {
		if log.Message == "Shutting down NVMe scraper" {
			foundLogMessage = true
			break
		}
	}
	assert.True(t, foundLogMessage, "Expected to find log about shutting down scraper")
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
	mockUtil.On("GetAllDevices").Return(nil, errors.New("failed to get devices"))

	scraper := newScraper(createTestReceiverConfig(), receivertest.NewNopSettings(component.MustNewType("awsnvmereceiver")), mockUtil, collections.NewSet[string]())

	_, err := scraper.scrape(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get devices")

	mockUtil.AssertExpectations(t)
}

func TestScraper_Scrape_EBSSuccess(t *testing.T) {
	originalGetRawData := getRawData
	t.Cleanup(func() { getRawData = originalGetRawData })
	getRawData = func(string) ([]byte, error) {
		m := ebs.Metrics{
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
		}
		return generateEBSRawData(m), nil
	}

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

	ilm := rm.ScopeMetrics().At(0).Metrics()
	assert.Equal(t, 11, ilm.Len())

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
	originalGetRawData := getRawData
	t.Cleanup(func() { getRawData = originalGetRawData })
	getRawData = func(string) ([]byte, error) {
		m := instancestore.Metrics{
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
		}
		return generateInstanceStoreRawData(m), nil
	}

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

	settings := receivertest.NewNopSettings(component.MustNewType("awsnvmereceiver"))
	scraper := newScraper(createTestReceiverConfig(), settings, mockUtil, collections.NewSet[string]("*"))

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

func TestScraper_Scrape_GetRawDataError(t *testing.T) {
	originalGetRawData := getRawData
	t.Cleanup(func() { getRawData = originalGetRawData })
	getRawData = func(string) ([]byte, error) {
		return nil, errors.New("failed to get raw data")
	}

	device1, err := nvme.ParseNvmeDeviceFileName("nvme0n1")
	require.NoError(t, err)

	mockUtil := new(mockNvmeUtil)
	mockUtil.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{device1}, nil)
	mockUtil.On("GetDeviceSerial", &device1).Return("vol1234567890abcdef", nil)
	mockUtil.On("GetDeviceModel", &device1).Return("Amazon Elastic Block Store", nil)
	mockUtil.On("DevicePath", "nvme0n1").Return("/dev/nvme0n1", nil)

	scraper := newScraper(createTestReceiverConfig(), receivertest.NewNopSettings(component.MustNewType("awsnvmereceiver")), mockUtil, collections.NewSet[string]("*"))

	metrics, err := scraper.scrape(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())

	mockUtil.AssertExpectations(t)
}

func TestScraper_Scrape_ParseRawDataError(t *testing.T) {
	originalGetRawData := getRawData
	t.Cleanup(func() { getRawData = originalGetRawData })
	getRawData = func(string) ([]byte, error) {
		return []byte{0x00}, nil // Invalid data
	}

	device1, err := nvme.ParseNvmeDeviceFileName("nvme0n1")
	require.NoError(t, err)

	mockUtil := new(mockNvmeUtil)
	mockUtil.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{device1}, nil)
	mockUtil.On("GetDeviceSerial", &device1).Return("vol1234567890abcdef", nil)
	mockUtil.On("GetDeviceModel", &device1).Return("Amazon Elastic Block Store", nil)
	mockUtil.On("DevicePath", "nvme0n1").Return("/dev/nvme0n1", nil)

	scraper := newScraper(createTestReceiverConfig(), receivertest.NewNopSettings(component.MustNewType("awsnvmereceiver")), mockUtil, collections.NewSet[string]("*"))

	metrics, err := scraper.scrape(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())

	mockUtil.AssertExpectations(t)
}

func TestScraper_Scrape_MultipleDevices_EBSAndInstanceStore(t *testing.T) {
	originalGetRawData := getRawData
	t.Cleanup(func() { getRawData = originalGetRawData })
	getRawData = func(devicePath string) ([]byte, error) {
		switch devicePath {
		case "/dev/nvme0n1":
			m := ebs.Metrics{
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
			}
			return generateEBSRawData(m), nil
		case "/dev/nvme1n1":
			m := instancestore.Metrics{
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
			}
			return generateInstanceStoreRawData(m), nil
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
	originalGetRawData := getRawData
	t.Cleanup(func() { getRawData = originalGetRawData })
	getRawData = func(string) ([]byte, error) {
		m := ebs.Metrics{
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
		}
		return generateEBSRawData(m), nil
	}

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
	settings.Logger = logger

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
	originalGetRawData := getRawData
	t.Cleanup(func() { getRawData = originalGetRawData })
	getRawData = func(string) ([]byte, error) {
		m := ebs.Metrics{
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
		}
		return generateEBSRawData(m), nil
	}

	device1, err := nvme.ParseNvmeDeviceFileName("nvme0n1")
	require.NoError(t, err)

	device2, err := nvme.ParseNvmeDeviceFileName("nvme0n1p1")
	require.NoError(t, err)

	mockUtil := new(mockNvmeUtil)
	mockUtil.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{device1, device2}, nil)
	mockUtil.On("GetDeviceSerial", &device1).Return("vol1234567890abcdef", nil)
	mockUtil.On("GetDeviceModel", &device1).Return("Amazon Elastic Block Store", nil)
	mockUtil.On("DevicePath", "nvme0n1").Return("/dev/nvme0n1", nil)

	scraper := newScraper(createTestReceiverConfig(), receivertest.NewNopSettings(component.MustNewType("awsnvmereceiver")), mockUtil, collections.NewSet[string]("*"))

	metrics, err := scraper.scrape(context.Background())
	assert.NoError(t, err)

	// Should only get one set of metrics for the controller
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

func TestScraper_Scrape_DevicePathError(t *testing.T) {
	device1, err := nvme.ParseNvmeDeviceFileName("nvme0n1")
	require.NoError(t, err)

	mockUtil := new(mockNvmeUtil)
	mockUtil.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{device1}, nil)
	mockUtil.On("GetDeviceSerial", &device1).Return("vol1234567890abcdef", nil)
	mockUtil.On("GetDeviceModel", &device1).Return("Amazon Elastic Block Store", nil)
	mockUtil.On("DevicePath", "nvme0n1").Return("", errors.New("device path error"))

	scraper := newScraper(createTestReceiverConfig(), receivertest.NewNopSettings(component.MustNewType("awsnvmereceiver")), mockUtil, collections.NewSet[string]("*"))

	metrics, err := scraper.scrape(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())

	mockUtil.AssertExpectations(t)
}

func TestScraper_Scrape_InstanceStoreDisabled(t *testing.T) {
	device1, err := nvme.ParseNvmeDeviceFileName("nvme0n1")
	require.NoError(t, err)

	mockUtil := new(mockNvmeUtil)
	mockUtil.On("GetAllDevices").Return([]nvme.DeviceFileAttributes{device1}, nil)
	mockUtil.On("GetDeviceSerial", &device1).Return("AWS1234567890abcdef0", nil)
	mockUtil.On("GetDeviceModel", &device1).Return("Amazon EC2 NVMe Instance Storage", nil)

	cfg := createDefaultConfig().(*Config)
	cfg.Metrics.DiskioInstanceStoreTotalReadOps.Enabled = false
	cfg.Metrics.DiskioInstanceStoreTotalWriteOps.Enabled = false
	cfg.Metrics.DiskioInstanceStoreTotalReadBytes.Enabled = false
	cfg.Metrics.DiskioInstanceStoreTotalWriteBytes.Enabled = false
	cfg.Metrics.DiskioInstanceStoreTotalReadTime.Enabled = false
	cfg.Metrics.DiskioInstanceStoreTotalWriteTime.Enabled = false
	cfg.Metrics.DiskioInstanceStorePerformanceExceededIops.Enabled = false
	cfg.Metrics.DiskioInstanceStorePerformanceExceededTp.Enabled = false
	cfg.Metrics.DiskioInstanceStoreVolumeQueueLength.Enabled = false
	cfg.Metrics.DiskioEbsTotalReadOps.Enabled = true

	core, _ := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)

	settings := receivertest.NewNopSettings(component.MustNewType("awsnvmereceiver"))
	settings.Logger = logger

	scraper := newScraper(cfg, settings, mockUtil, collections.NewSet[string]("*"))

	metrics, err := scraper.scrape(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())

	mockUtil.AssertExpectations(t)
}

func TestSafeUint64ToInt64_Overflow(t *testing.T) {
	_, err := safeUint64ToInt64(uint64(math.MaxInt64) + 1)
	assert.Error(t, err)
}

func createTestReceiverConfig() *Config {
	cfg := createDefaultConfig().(*Config)

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

func generateEBSRawData(m ebs.Metrics) []byte {
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, m)
	return buf.Bytes()
}

func generateInstanceStoreRawData(m instancestore.Metrics) []byte {
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, m)
	return buf.Bytes()
}
