// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsinstancestorenvmereceiver

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
	"github.com/aws/amazon-cloudwatch-agent/internal/ec2metadataprovider"
	"github.com/aws/amazon-cloudwatch-agent/internal/nvme"
	"github.com/aws/amazon-cloudwatch-agent/internal/retryer"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	"github.com/aws/amazon-cloudwatch-agent/receiver/awsinstancestorenvmereceiver/internal/metadata"
)

type nvmeScraper struct {
	logger           *zap.Logger
	mb               *metadata.MetricsBuilder
	nvme             nvme.DeviceInfoProvider
	metadataProvider ec2metadataprovider.MetadataProvider

	allowedDevices collections.Set[string]
}

type instanceStoreDevices struct {
	serialNumber string
	deviceNames  []string
}

type recordDataMetricFunc func(pcommon.Timestamp, int64)

func newScraper(cfg *Config, settings receiver.Settings, nvmeProvider nvme.DeviceInfoProvider, devices collections.Set[string]) *nvmeScraper {
	// Create EC2 metadata provider for getting InstanceId
	mdCredentialConfig := &configaws.CredentialConfig{}
	metadataProvider := ec2metadataprovider.NewMetadataProvider(mdCredentialConfig.Credentials(), retryer.GetDefaultRetryNumber())

	return &nvmeScraper{
		logger:           settings.Logger,
		mb:               metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
		nvme:             nvmeProvider,
		metadataProvider: metadataProvider,
		allowedDevices:   devices,
	}
}

func (s *nvmeScraper) start(_ context.Context, _ component.Host) error {
	s.logger.Debug("Starting Instance Store NVMe scraper", zap.String("receiver", metadata.Type.String()))
	return nil
}

func (s *nvmeScraper) shutdown(_ context.Context) error {
	s.logger.Debug("Shutting down Instance Store NVMe scraper", zap.String("receiver", metadata.Type.String()))
	return nil
}

func (s *nvmeScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	s.logger.Debug("Began scraping for Instance Store NVMe metrics")

	instanceStoreDevicesByController, err := s.getInstanceStoreDevicesByController()
	if err != nil {
		s.logger.Error("failed to get Instance Store devices", zap.Error(err))
		return pmetric.NewMetrics(), err
	}

	if len(instanceStoreDevicesByController) == 0 {
		s.logger.Debug("no Instance Store devices found for monitoring")
		return s.mb.Emit(), nil
	}

	// Get InstanceId from EC2 metadata service
	instanceID, err := s.metadataProvider.InstanceID(ctx)
	if err != nil {
		s.logger.Warn("unable to get instance ID from metadata service, using placeholder", zap.Error(err))
		instanceID = "unknown"
	}

	now := pcommon.NewTimestampFromTime(time.Now())
	successfulDevices := 0
	totalDevices := len(instanceStoreDevicesByController)

	for controllerID, devices := range instanceStoreDevicesByController {
		// Some devices are owned by root:root, root:disk, etc, so the agent will attempt to
		// retrieve the metric for a device (grouped by controller ID) until the first success
		foundWorkingDevice := false
		var lastError error

		for _, device := range devices.deviceNames {
			if foundWorkingDevice {
				break
			}

			devicePath, err := s.nvme.DevicePath(device)
			if err != nil {
				s.logger.Debug("unable to get device path", zap.String("device", device), zap.Error(err))
				lastError = err
				continue
			}

			// Get Instance Store metrics from the device
			metrics, err := nvme.GetInstanceStoreMetrics(devicePath)
			if err != nil {
				s.logger.Debug("unable to get Instance Store metrics for device",
					zap.String("device", device),
					zap.String("devicePath", devicePath),
					zap.Error(err))
				lastError = err
				continue
			}

			foundWorkingDevice = true

			// Create resource builder and set dimensions
			rb := s.mb.NewResourceBuilder()
			rb.SetInstanceID(instanceID)
			rb.SetDevice(devicePath)
			rb.SetSerialNumber(devices.serialNumber)

			// Record all 9 Instance Store metrics with enhanced error handling
			metricsRecorded := 0
			metricsRecorded += s.recordMetricWithName(s.mb.RecordDiskioInstanceStoreTotalReadOpsDataPoint, now, metrics.ReadOps, "total_read_ops")
			metricsRecorded += s.recordMetricWithName(s.mb.RecordDiskioInstanceStoreTotalWriteOpsDataPoint, now, metrics.WriteOps, "total_write_ops")
			metricsRecorded += s.recordMetricWithName(s.mb.RecordDiskioInstanceStoreTotalReadBytesDataPoint, now, metrics.ReadBytes, "total_read_bytes")
			metricsRecorded += s.recordMetricWithName(s.mb.RecordDiskioInstanceStoreTotalWriteBytesDataPoint, now, metrics.WriteBytes, "total_write_bytes")
			metricsRecorded += s.recordMetricWithName(s.mb.RecordDiskioInstanceStoreTotalReadTimeDataPoint, now, metrics.TotalReadTime, "total_read_time")
			metricsRecorded += s.recordMetricWithName(s.mb.RecordDiskioInstanceStoreTotalWriteTimeDataPoint, now, metrics.TotalWriteTime, "total_write_time")
			metricsRecorded += s.recordMetricWithName(s.mb.RecordDiskioInstanceStoreVolumePerformanceExceededIopsDataPoint, now, metrics.EC2IOPSExceeded, "volume_performance_exceeded_iops")
			metricsRecorded += s.recordMetricWithName(s.mb.RecordDiskioInstanceStoreVolumePerformanceExceededTpDataPoint, now, metrics.EC2ThroughputExceeded, "volume_performance_exceeded_tp")
			metricsRecorded += s.recordMetricWithName(s.mb.RecordDiskioInstanceStoreVolumeQueueLengthDataPoint, now, metrics.QueueLength, "volume_queue_length")

			// Emit metrics for this resource
			s.mb.EmitForResource(metadata.WithResource(rb.Emit()))

			s.logger.Debug("successfully recorded Instance Store metrics",
				zap.String("device", device),
				zap.String("devicePath", devicePath),
				zap.Int("controllerID", controllerID),
				zap.String("serialNumber", devices.serialNumber),
				zap.Int("metricsRecorded", metricsRecorded))
		}

		if foundWorkingDevice {
			successfulDevices++
		} else {
			s.logger.Warn("failed to get metrics for Instance Store device controller",
				zap.Int("controllerID", controllerID),
				zap.String("serialNumber", devices.serialNumber),
				zap.Strings("deviceNames", devices.deviceNames),
				zap.Error(lastError))
		}
	}

	s.logger.Debug("completed Instance Store metrics scraping",
		zap.Int("successfulDevices", successfulDevices),
		zap.Int("totalDevices", totalDevices))

	if successfulDevices == 0 {
		s.logger.Warn("no Instance Store devices were successfully scraped")
	}

	return s.mb.Emit(), nil
}

// getInstanceStoreDevicesByController groups Instance Store devices by controller ID to avoid duplicate metrics.
// NVMe devices with the same controller ID (e.g., nvme0n1, nvme0n1p1) share the same metrics.
func (s *nvmeScraper) getInstanceStoreDevicesByController() (map[int]*instanceStoreDevices, error) {
	allNvmeDevices, err := s.nvme.GetAllDevices()
	if err != nil {
		return nil, fmt.Errorf("failed to discover NVMe devices: %w", err)
	}

	if len(allNvmeDevices) == 0 {
		s.logger.Debug("no NVMe devices found on system")
		return make(map[int]*instanceStoreDevices), nil
	}

	devices := make(map[int]*instanceStoreDevices)
	processedDevices := 0
	skippedDevices := 0
	errorCount := 0

	for _, device := range allNvmeDevices {
		deviceName := device.DeviceName()
		processedDevices++

		// Check if all devices should be collected. Otherwise check if defined by user
		hasAsterisk := s.allowedDevices.Contains("*")
		if !hasAsterisk {
			if isAllowed := s.allowedDevices.Contains(deviceName); !isAllowed {
				s.logger.Debug("skipping device not in allowed list", zap.String("device", deviceName))
				skippedDevices++
				continue
			}
		}

		// NVMe device with the same controller ID was already seen. We do not need to repeat the work of
		// retrieving the serial number and validating if it's an Instance Store device
		if entry, seenController := devices[device.Controller()]; seenController {
			entry.deviceNames = append(entry.deviceNames, deviceName)
			s.logger.Debug("adding device to existing controller group",
				zap.String("device", deviceName),
				zap.Int("controllerID", device.Controller()))
			continue
		}

		// Validate if this is an Instance Store device with enhanced error handling
		isInstanceStore, err := s.nvme.IsInstanceStoreDevice(&device)
		if err != nil {
			s.logger.Debug("failed to validate Instance Store device",
				zap.String("device", deviceName),
				zap.Int("controllerID", device.Controller()),
				zap.Error(err))
			errorCount++
			continue
		}

		if !isInstanceStore {
			s.logger.Debug("skipping non-Instance Store NVMe device",
				zap.String("device", deviceName),
				zap.Int("controllerID", device.Controller()))
			skippedDevices++
			continue
		}

		// Get device serial number with error handling
		serial, err := s.nvme.GetDeviceSerial(&device)
		if err != nil {
			s.logger.Warn("unable to get serial number for Instance Store device",
				zap.String("device", deviceName),
				zap.Int("controllerID", device.Controller()),
				zap.Error(err))
			// Use a placeholder serial number to allow metrics collection
			serial = fmt.Sprintf("unknown-controller-%d", device.Controller())
		}

		devices[device.Controller()] = &instanceStoreDevices{
			deviceNames:  []string{deviceName},
			serialNumber: serial,
		}

		s.logger.Debug("discovered Instance Store device",
			zap.String("device", deviceName),
			zap.Int("controllerID", device.Controller()),
			zap.String("serialNumber", serial))
	}

	s.logger.Debug("completed device discovery",
		zap.Int("totalProcessed", processedDevices),
		zap.Int("instanceStoreDevices", len(devices)),
		zap.Int("skippedDevices", skippedDevices),
		zap.Int("errorCount", errorCount))

	if len(devices) == 0 && errorCount > 0 {
		return devices, fmt.Errorf("no Instance Store devices found, encountered %d errors during discovery", errorCount)
	}

	return devices, nil
}

// recordMetric safely records a metric value with overflow protection
func (s *nvmeScraper) recordMetric(recordFn recordDataMetricFunc, ts pcommon.Timestamp, val uint64) {
	converted, err := safeUint64ToInt64(val)
	if err != nil {
		s.logger.Debug("skipping metric due to potential integer overflow", zap.Uint64("value", val))
		return
	}
	recordFn(ts, converted)
}

// recordMetricWithName safely records a metric value with overflow protection and returns 1 if successful, 0 if skipped
func (s *nvmeScraper) recordMetricWithName(recordFn recordDataMetricFunc, ts pcommon.Timestamp, val uint64, metricName string) int {
	converted, err := safeUint64ToInt64(val)
	if err != nil {
		s.logger.Warn("skipping metric due to potential integer overflow",
			zap.String("metric", metricName),
			zap.Uint64("value", val),
			zap.Error(err))
		return 0
	}
	recordFn(ts, converted)
	return 1
}

// safeUint64ToInt64 converts a uint64 value to int64 with overflow detection.
// Returns an error if the value exceeds the maximum int64 value.
func safeUint64ToInt64(value uint64) (int64, error) {
	if value > 9223372036854775807 { // math.MaxInt64
		return 0, fmt.Errorf("value %d is too large for int64", value)
	}
	return int64(value), nil
}
