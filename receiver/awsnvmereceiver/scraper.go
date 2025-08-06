// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsnvmereceiver

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	"github.com/aws/amazon-cloudwatch-agent/receiver/awsnvmereceiver/internal/metadata"
	"github.com/aws/amazon-cloudwatch-agent/receiver/awsnvmereceiver/internal/nvme"
)

const (
	ebsModel           = "Amazon Elastic Block Store"
	instanceStoreModel = "Amazon EC2 NVMe Instance Store"
)

type nvmeScraper struct {
	logger *zap.Logger
	mb     *metadata.MetricsBuilder
	nvme   nvme.DeviceInfoProvider

	allowedDevices       collections.Set[string]
	collectEbs           bool
	collectInstanceStore bool
}

type nvmeDevices struct {
	deviceType  string // EBS or Instance Store device type
	identifier  string // volume_id for EBS, serial_id for IS
	deviceNames []string
}

type recordDataMetricFunc func(pcommon.Timestamp, int64)

// For unit testing
var getMetrics = nvme.GetMetrics

func (s *nvmeScraper) start(_ context.Context, _ component.Host) error {
	s.logger.Debug("Starting NVMe scraper", zap.String("receiver", metadata.Type.String()))
	return nil
}

func (s *nvmeScraper) shutdown(_ context.Context) error {
	s.logger.Debug("Shutting down NVMe scraper", zap.String("receiver", metadata.Type.String()))
	return nil
}

func (s *nvmeScraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	s.logger.Debug("Began scraping for NVMe metrics")

	nvmeDevicesByController, err := s.getNVMeDevicesByController()
	if err != nil {
		return pmetric.NewMetrics(), err
	}

	now := pcommon.NewTimestampFromTime(time.Now())

	for id, nvmeDevices := range nvmeDevicesByController {
		// Some devices are owned by root:root, root:disk, etc, so the agent will attempt to
		// retrieve the metric for a device (grouped by controller ID) until the first
		// success
		foundWorkingDevice := false

		for _, device := range nvmeDevices.deviceNames {
			if foundWorkingDevice {
				break
			}

			devicePath, err := s.nvme.DevicePath(device)
			if err != nil {
				s.logger.Debug("unable to get device path", zap.String("device", device), zap.Error(err))
				continue
			}
			metrics, err := getMetrics(devicePath)
			if err != nil {
				s.logger.Debug("unable to get metrics for device", zap.String("device", device), zap.Error(err))
				continue
			}

			foundWorkingDevice = true

			rb := s.mb.NewResourceBuilder()

			if nvmeDevices.deviceType == "ebs" {
				rb.SetVolumeID(nvmeDevices.identifier)
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
			} else {
				rb.SetSerialID(nvmeDevices.identifier)
				s.recordMetric(s.mb.RecordDiskioInstanceStoreTotalReadOpsDataPoint, now, metrics.ReadOps)
				s.recordMetric(s.mb.RecordDiskioInstanceStoreTotalWriteOpsDataPoint, now, metrics.WriteOps)
				s.recordMetric(s.mb.RecordDiskioInstanceStoreTotalReadBytesDataPoint, now, metrics.ReadBytes)
				s.recordMetric(s.mb.RecordDiskioInstanceStoreTotalWriteBytesDataPoint, now, metrics.WriteBytes)
				s.recordMetric(s.mb.RecordDiskioInstanceStoreTotalReadTimeDataPoint, now, metrics.TotalReadTime)
				s.recordMetric(s.mb.RecordDiskioInstanceStoreTotalWriteTimeDataPoint, now, metrics.TotalWriteTime)
				s.recordMetric(s.mb.RecordDiskioInstanceStorePerformanceExceededIopsDataPoint, now, metrics.EC2IOPSExceeded)
				s.recordMetric(s.mb.RecordDiskioInstanceStorePerformanceExceededTpDataPoint, now, metrics.EC2ThroughputExceeded)
				s.recordMetric(s.mb.RecordDiskioInstanceStoreVolumeQueueLengthDataPoint, now, metrics.QueueLength)
			}

			s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
		}

		if foundWorkingDevice {
			s.logger.Debug("emitted metrics for nvme device with controller id", zap.Int("controllerID", id), zap.String("identifier", nvmeDevices.identifier))
		} else {
			s.logger.Debug("unable to get metrics for nvme device with controller id", zap.Int("controllerID", id))
		}
	}

	return s.mb.Emit(), nil
}

// nvme0, nvme1, ... nvme{n} can have multiple devices with the same controller ID.
// For example nvme0n1, nvme0n1p1 are all under the controller ID 0. The metrics
// are the same based on the controller ID. We also do not want to duplicate metrics
// so we group the devices by the controller ID.
func (s *nvmeScraper) getNVMeDevicesByController() (map[int]*nvmeDevices, error) {
	allNvmeDevices, err := s.nvme.GetAllDevices()
	if err != nil {
		return nil, err
	}

	devices := make(map[int]*nvmeDevices)

	for _, device := range allNvmeDevices {
		deviceName := device.DeviceName()

		// Check if all devices should be collected. Otherwise check if defined by user
		hasAsterisk := s.allowedDevices.Contains("*")
		if !hasAsterisk {
			if isAllowed := s.allowedDevices.Contains(deviceName); !isAllowed {
				s.logger.Debug("skipping un-allowed device", zap.String("device", deviceName))
				continue
			}
		}

		controllerID := device.Controller()
		// NVMe device with the same controller ID was already seen. We do not need to repeat the work of
		// retrieving the volume ID and validating if it's an EBS/IS device
		if entry, seenController := devices[controllerID]; seenController {
			entry.deviceNames = append(entry.deviceNames, deviceName)
			s.logger.Debug("skipping unnecessary device validation steps", zap.String("device", deviceName))
			continue
		}

		serial, err := s.nvme.GetDeviceSerial(&device)
		if err != nil {
			s.logger.Debug("unable to get serial number of device", zap.String("device", deviceName), zap.Error(err))
			continue
		}

		model, err := s.nvme.GetDeviceModel(&device)
		if err != nil {
			s.logger.Debug("unable to get device model", zap.String("device", deviceName), zap.Error(err))
			continue
		}

		var deviceType, identifier string
		switch model {
		case ebsModel:
			if !s.collectEbs {
				s.logger.Debug("skipping EBS device as no EBS metrics enabled", zap.String("device", deviceName))
				continue
			}
			deviceType = "ebs"
			if !strings.HasPrefix(serial, "vol") || len(serial) < 4 {
				s.logger.Debug("device serial is not a valid volume id", zap.String("device", deviceName), zap.String("serial", serial))
				continue
			}
			identifier = fmt.Sprintf("vol-%s", serial[3:])
		case instanceStoreModel: // Verify if there is a prefix requirement like ebs- but I don't there is ???!!!
			if !s.collectInstanceStore {
				s.logger.Debug("skipping Instance Store device as no IS metrics enabled", zap.String("device", deviceName))
				continue
			}
			deviceType = "instance_store"
			identifier = serial
		default:
			s.logger.Debug("skipping unknown device model", zap.String("device", deviceName), zap.String("model", model))
			continue
		}

		devices[controllerID] = &nvmeDevices{
			deviceType:  deviceType,
			identifier:  identifier,
			deviceNames: []string{deviceName},
		}
	}

	return devices, nil
}

func newScraper(cfg *Config,
	settings receiver.Settings,
	nvme nvme.DeviceInfoProvider,
	allowedDevices collections.Set[string],
) *nvmeScraper {
	scraper := &nvmeScraper{
		logger:         settings.TelemetrySettings.Logger,
		mb:             metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
		nvme:           nvme,
		allowedDevices: allowedDevices,
	}

	scraper.collectEbs, scraper.collectInstanceStore = computeCollectFlags(cfg)
	return scraper
}

func (s *nvmeScraper) recordMetric(recordFn recordDataMetricFunc, ts pcommon.Timestamp, val uint64) {
	converted, err := safeUint64ToInt64(val)
	if err != nil {
		s.logger.Debug("skipping metric due to potential integer overflow")
		return
	}
	recordFn(ts, converted)
}

func safeUint64ToInt64(value uint64) (int64, error) {
	if value > math.MaxInt64 {
		return 0, fmt.Errorf("value %d is too large for int64", value)
	}
	return int64(value), nil
}

// computeCollectFlags computes whether to collect for EBS and Instance Store based on enabled metrics
func computeCollectFlags(cfg *Config) (collectEbs bool, collectInstanceStore bool) {
	m := cfg.MetricsBuilderConfig.Metrics
	collectEbs = m.DiskioEbsTotalReadOps.Enabled ||
		m.DiskioEbsTotalWriteOps.Enabled ||
		m.DiskioEbsTotalReadBytes.Enabled ||
		m.DiskioEbsTotalWriteBytes.Enabled ||
		m.DiskioEbsTotalReadTime.Enabled ||
		m.DiskioEbsTotalWriteTime.Enabled ||
		m.DiskioEbsVolumePerformanceExceededIops.Enabled ||
		m.DiskioEbsVolumePerformanceExceededTp.Enabled ||
		m.DiskioEbsEc2InstancePerformanceExceededIops.Enabled ||
		m.DiskioEbsEc2InstancePerformanceExceededTp.Enabled ||
		m.DiskioEbsVolumeQueueLength.Enabled

	collectInstanceStore = m.DiskioInstanceStoreTotalReadOps.Enabled ||
		m.DiskioInstanceStoreTotalWriteOps.Enabled ||
		m.DiskioInstanceStoreTotalReadBytes.Enabled ||
		m.DiskioInstanceStoreTotalWriteBytes.Enabled ||
		m.DiskioInstanceStoreTotalReadTime.Enabled ||
		m.DiskioInstanceStoreTotalWriteTime.Enabled ||
		m.DiskioInstanceStorePerformanceExceededIops.Enabled ||
		m.DiskioInstanceStorePerformanceExceededTp.Enabled ||
		m.DiskioInstanceStoreVolumeQueueLength.Enabled

	return
}
