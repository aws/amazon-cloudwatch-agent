// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsnvmereceiver

import (
	"context"
	"fmt"
	"strings"
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
	"github.com/aws/amazon-cloudwatch-agent/receiver/awsnvmereceiver/internal/metadata"
)

// nvmeScraper implements unified scraping logic for both EBS and Instance Store NVMe devices
type nvmeScraper struct {
	logger           *zap.Logger
	mb               *metadata.MetricsBuilder
	nvmeUtil         nvme.DeviceInfoProvider
	metadataProvider ec2metadataprovider.MetadataProvider
	deviceSet        collections.Set[string]
}

// nvmeDevices represents a group of devices with the same controller ID and device type
type nvmeDevices struct {
	deviceType   string   // "ebs" or "instance_store"
	serialNumber string   // Device serial number or volume ID
	deviceNames  []string // List of device names with same controller
}

// devicesByController maps controller ID to device information
type devicesByController map[int]*nvmeDevices

// recordDataMetricFunc defines the function signature for recording metrics
type recordDataMetricFunc func(pcommon.Timestamp, int64)

// newScraper creates a new unified NVMe scraper instance
func newScraper(cfg *Config, settings receiver.Settings, nvmeUtil nvme.DeviceInfoProvider, deviceSet collections.Set[string]) *nvmeScraper {
	// Create EC2 metadata provider for getting InstanceId
	mdCredentialConfig := &configaws.CredentialConfig{}
	metadataProvider := ec2metadataprovider.NewMetadataProvider(mdCredentialConfig.Credentials(), retryer.GetDefaultRetryNumber())

	return &nvmeScraper{
		logger:           settings.TelemetrySettings.Logger,
		mb:               metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
		nvmeUtil:         nvmeUtil,
		metadataProvider: metadataProvider,
		deviceSet:        deviceSet,
	}
}

// start initializes the scraper
func (s *nvmeScraper) start(_ context.Context, _ component.Host) error {
	s.logger.Debug("Starting unified NVMe scraper", zap.String("receiver", metadata.Type.String()))
	return nil
}

// shutdown cleans up scraper resources
func (s *nvmeScraper) shutdown(_ context.Context) error {
	s.logger.Debug("Shutting down unified NVMe scraper", zap.String("receiver", metadata.Type.String()))
	return nil
}

// scrape performs the main scraping logic with device type routing
func (s *nvmeScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	s.logger.Debug("Began scraping for unified NVMe metrics")

	// Discover and group devices by controller
	devicesByController, err := s.getDevicesByController()
	if err != nil {
		s.logger.Error("failed to get devices by controller", zap.Error(err))
		return pmetric.NewMetrics(), err
	}

	if len(devicesByController) == 0 {
		s.logger.Debug("no NVMe devices found for monitoring")
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
	totalDevices := len(devicesByController)

	// Process each device group
	for controllerID, devices := range devicesByController {
		// Some devices are owned by root:root, root:disk, etc, so the agent will attempt to
		// retrieve the metric for a device (grouped by controller ID) until the first success
		foundWorkingDevice := false
		var lastError error

		for _, deviceName := range devices.deviceNames {
			if foundWorkingDevice {
				break
			}

			devicePath, err := s.nvmeUtil.DevicePath(deviceName)
			if err != nil {
				s.logger.Debug("unable to get device path",
					zap.String("device", deviceName),
					zap.Error(err))
				lastError = err
				continue
			}

			// Route to appropriate parsing function based on device type
			switch devices.deviceType {
			case "ebs":
				if err := s.processEBSDevice(devicePath, devices, instanceID, now); err != nil {
					s.logger.Debug("unable to process EBS device",
						zap.String("device", deviceName),
						zap.String("devicePath", devicePath),
						zap.Error(err))
					lastError = err
					continue
				}
			case "instance_store":
				if err := s.processInstanceStoreDevice(devicePath, devices, instanceID, now); err != nil {
					s.logger.Debug("unable to process Instance Store device",
						zap.String("device", deviceName),
						zap.String("devicePath", devicePath),
						zap.Error(err))
					lastError = err
					continue
				}
			default:
				lastError = fmt.Errorf("unknown device type: %s", devices.deviceType)
				s.logger.Error("unknown device type detected",
					zap.String("device", deviceName),
					zap.String("deviceType", devices.deviceType))
				continue
			}

			foundWorkingDevice = true
			s.logger.Debug("successfully processed device",
				zap.String("device", deviceName),
				zap.String("devicePath", devicePath),
				zap.String("deviceType", devices.deviceType),
				zap.Int("controllerID", controllerID))
		}

		if foundWorkingDevice {
			successfulDevices++
		} else {
			s.logger.Warn("failed to get metrics for device controller",
				zap.Int("controllerID", controllerID),
				zap.String("deviceType", devices.deviceType),
				zap.String("serialNumber", devices.serialNumber),
				zap.Strings("deviceNames", devices.deviceNames),
				zap.Error(lastError))
		}
	}

	s.logger.Debug("completed unified NVMe metrics scraping",
		zap.Int("successfulDevices", successfulDevices),
		zap.Int("totalDevices", totalDevices))

	if successfulDevices == 0 {
		s.logger.Warn("no NVMe devices were successfully scraped")
	}

	return s.mb.Emit(), nil
}

// getDevicesByController discovers and groups devices by controller ID with device type detection
func (s *nvmeScraper) getDevicesByController() (devicesByController, error) {
	allNvmeDevices, err := s.nvmeUtil.GetAllDevices()
	if err != nil {
		return nil, fmt.Errorf("failed to discover NVMe devices: %w", err)
	}

	if len(allNvmeDevices) == 0 {
		s.logger.Debug("no NVMe devices found on system")
		return make(devicesByController), nil
	}

	devices := make(devicesByController)
	processedDevices := 0
	skippedDevices := 0
	errorCount := 0

	for _, device := range allNvmeDevices {
		deviceName := device.DeviceName()
		processedDevices++

		// Check if all devices should be collected. Otherwise check if defined by user
		hasAsterisk := s.deviceSet.Contains("*")
		if !hasAsterisk {
			if isAllowed := s.deviceSet.Contains(deviceName); !isAllowed {
				s.logger.Debug("skipping device not in allowed list", zap.String("device", deviceName))
				skippedDevices++
				continue
			}
		}

		// NVMe device with the same controller ID was already seen. We do not need to repeat the work of
		// retrieving the serial number and validating device type
		if entry, seenController := devices[device.Controller()]; seenController {
			entry.deviceNames = append(entry.deviceNames, deviceName)
			s.logger.Debug("adding device to existing controller group",
				zap.String("device", deviceName),
				zap.Int("controllerID", device.Controller()),
				zap.String("deviceType", entry.deviceType))
			continue
		}

		// Detect device type using unified detection logic
		deviceType, err := s.nvmeUtil.DetectDeviceType(&device)
		if err != nil {
			s.logger.Debug("failed to detect device type",
				zap.String("device", deviceName),
				zap.Int("controllerID", device.Controller()),
				zap.Error(err))
			errorCount++
			continue
		}

		// Get device serial number with error handling
		serial, err := s.nvmeUtil.GetDeviceSerial(&device)
		if err != nil {
			s.logger.Warn("unable to get serial number for device",
				zap.String("device", deviceName),
				zap.String("deviceType", deviceType),
				zap.Int("controllerID", device.Controller()),
				zap.Error(err))
			// Use a placeholder serial number to allow metrics collection
			serial = fmt.Sprintf("unknown-controller-%d", device.Controller())
		}

		// For EBS devices, format the serial as volume ID
		if deviceType == "ebs" {
			// The serial should begin with vol and have content after the vol prefix
			if strings.HasPrefix(serial, "vol") && len(serial) > 3 {
				serial = fmt.Sprintf("vol-%s", serial[3:])
			} else {
				s.logger.Debug("device serial is not a valid volume id",
					zap.String("device", deviceName),
					zap.String("serial", serial))
				// Continue with original serial for metrics collection
			}
		}

		devices[device.Controller()] = &nvmeDevices{
			deviceType:   deviceType,
			serialNumber: serial,
			deviceNames:  []string{deviceName},
		}

		s.logger.Debug("discovered device",
			zap.String("device", deviceName),
			zap.String("deviceType", deviceType),
			zap.Int("controllerID", device.Controller()),
			zap.String("serialNumber", serial))
	}

	s.logger.Debug("completed device discovery",
		zap.Int("totalProcessed", processedDevices),
		zap.Int("discoveredDevices", len(devices)),
		zap.Int("skippedDevices", skippedDevices),
		zap.Int("errorCount", errorCount))

	if len(devices) == 0 && errorCount > 0 {
		return devices, fmt.Errorf("no devices found, encountered %d errors during discovery", errorCount)
	}

	return devices, nil
}

// processEBSDevice processes an EBS device and records its metrics
func (s *nvmeScraper) processEBSDevice(devicePath string, devices *nvmeDevices, instanceID string, now pcommon.Timestamp) error {
	// Get EBS metrics from the device
	metrics, err := nvme.GetEBSMetrics(devicePath)
	if err != nil {
		return fmt.Errorf("failed to get EBS metrics: %w", err)
	}

	// Create resource builder and set dimensions
	rb := s.mb.NewResourceBuilder()
	rb.SetInstanceID(instanceID)
	rb.SetDeviceType("ebs")
	rb.SetDevice(devicePath)
	rb.SetSerialNumber(devices.serialNumber)

	// Record all EBS metrics with safe conversion and overflow protection
	s.recordMetric(s.mb.RecordDiskioEbsTotalReadOpsDataPoint, now, metrics.ReadOps)
	s.recordMetric(s.mb.RecordDiskioEbsTotalWriteOpsDataPoint, now, metrics.WriteOps)
	s.recordMetric(s.mb.RecordDiskioEbsTotalReadBytesDataPoint, now, metrics.ReadBytes)
	s.recordMetric(s.mb.RecordDiskioEbsTotalWriteBytesDataPoint, now, metrics.WriteBytes)
	s.recordMetric(s.mb.RecordDiskioEbsTotalReadTimeDataPoint, now, metrics.TotalReadTime)
	s.recordMetric(s.mb.RecordDiskioEbsTotalWriteTimeDataPoint, now, metrics.TotalWriteTime)
	s.recordMetric(s.mb.RecordDiskioEbsVolumePerformanceExceededIopsDataPoint, now, metrics.EBSIOPSExceeded)
	s.recordMetric(s.mb.RecordDiskioEbsVolumePerformanceExceededTpDataPoint, now, metrics.EBSThroughputExceeded)
	s.recordMetric(s.mb.RecordDiskioEbsEc2InstancePerformanceExceededIopsDataPoint, now, metrics.EC2IOPSExceeded)
	s.recordMetric(s.mb.RecordDiskioEbsEc2InstancePerformanceExceededTpDataPoint, now, metrics.EC2ThroughputExceeded)
	s.recordMetric(s.mb.RecordDiskioEbsVolumeQueueLengthDataPoint, now, metrics.QueueLength)

	// Emit metrics for this resource
	s.mb.EmitForResource(metadata.WithResource(rb.Emit()))

	return nil
}

// processInstanceStoreDevice processes an Instance Store device and records its metrics
func (s *nvmeScraper) processInstanceStoreDevice(devicePath string, devices *nvmeDevices, instanceID string, now pcommon.Timestamp) error {
	// Get Instance Store metrics from the device
	metrics, err := nvme.GetInstanceStoreMetrics(devicePath)
	if err != nil {
		return fmt.Errorf("failed to get Instance Store metrics: %w", err)
	}

	// Create resource builder and set dimensions
	rb := s.mb.NewResourceBuilder()
	rb.SetInstanceID(instanceID)
	rb.SetDeviceType("instance_store")
	rb.SetDevice(devicePath)
	rb.SetSerialNumber(devices.serialNumber)

	// Record all Instance Store metrics with safe conversion and overflow protection
	// Note: Instance Store devices skip EBS-specific fields (EBSIOPSExceeded, EBSThroughputExceeded)
	s.recordMetric(s.mb.RecordDiskioInstanceStoreTotalReadOpsDataPoint, now, metrics.ReadOps)
	s.recordMetric(s.mb.RecordDiskioInstanceStoreTotalWriteOpsDataPoint, now, metrics.WriteOps)
	s.recordMetric(s.mb.RecordDiskioInstanceStoreTotalReadBytesDataPoint, now, metrics.ReadBytes)
	s.recordMetric(s.mb.RecordDiskioInstanceStoreTotalWriteBytesDataPoint, now, metrics.WriteBytes)
	s.recordMetric(s.mb.RecordDiskioInstanceStoreTotalReadTimeDataPoint, now, metrics.TotalReadTime)
	s.recordMetric(s.mb.RecordDiskioInstanceStoreTotalWriteTimeDataPoint, now, metrics.TotalWriteTime)
	s.recordMetric(s.mb.RecordDiskioInstanceStoreVolumePerformanceExceededIopsDataPoint, now, metrics.EC2IOPSExceeded)
	s.recordMetric(s.mb.RecordDiskioInstanceStoreVolumePerformanceExceededTpDataPoint, now, metrics.EC2ThroughputExceeded)
	s.recordMetric(s.mb.RecordDiskioInstanceStoreVolumeQueueLengthDataPoint, now, metrics.QueueLength)

	// Emit metrics for this resource
	s.mb.EmitForResource(metadata.WithResource(rb.Emit()))

	return nil
}

// recordMetric safely records a metric value with overflow protection and prefix application
func (s *nvmeScraper) recordMetric(recordFn recordDataMetricFunc, ts pcommon.Timestamp, val uint64) {
	converted, err := nvme.SafeUint64ToInt64(val)
	if err != nil {
		s.logger.Debug("skipping metric due to potential integer overflow",
			zap.Uint64("value", val),
			zap.Error(err))
		return
	}
	recordFn(ts, converted)
}
