// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsebsnvmereceiver

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
	"github.com/aws/amazon-cloudwatch-agent/receiver/awsebsnvmereceiver/internal/metadata"
	"github.com/aws/amazon-cloudwatch-agent/receiver/awsebsnvmereceiver/internal/nvme"
)

type nvmeScraper struct {
	logger *zap.Logger
	mb     *metadata.MetricsBuilder
	nvme   nvme.DeviceInfoProvider

	allowedDevices collections.Set[string]
}

type ebsDevices struct {
	volumeID    string
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

	ebsDevicesByController, err := s.getEbsDevicesByController()
	if err != nil {
		return pmetric.NewMetrics(), err
	}

	now := pcommon.NewTimestampFromTime(time.Now())

	for id, ebsDevices := range ebsDevicesByController {
		// Some devices are owned by root:root, root:disk, etc, so the agent will attempt to
		// retrieve the metric for a device (grouped by controller ID) until the first
		// success
		foundWorkingDevice := false

		for _, device := range ebsDevices.deviceNames {
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
			rb.SetVolumeID(ebsDevices.volumeID)

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

			s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
		}

		if foundWorkingDevice {
			s.logger.Debug("emitted metrics for nvme device with controller id", zap.Int("controllerID", id), zap.String("volumeID", ebsDevices.volumeID))
		} else {
			s.logger.Debug("unable to get metrics for nvme device with controller id", zap.Int("controllerID", id), zap.String("volumeID", ebsDevices.volumeID))
		}
	}

	return s.mb.Emit(), nil
}

// nvme0, nvme1, ... nvme{n} can have multiple devices with the same controller ID.
// For example nvme0n1, nvme0n1p1 are all under the controller ID 0. The metrics
// are the same based on the controller ID. We also do not want to duplicate metrics
// so we group the devices by the controller ID.
func (s *nvmeScraper) getEbsDevicesByController() (map[int]*ebsDevices, error) {
	allNvmeDevices, err := s.nvme.GetAllDevices()
	if err != nil {
		return nil, err
	}

	devices := make(map[int]*ebsDevices)

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

		// NVMe device with the same controller ID was already seen. We do not need to repeat the work of
		// retrieving the volume ID and validating if it's an EBS device
		if entry, seenController := devices[device.Controller()]; seenController {
			entry.deviceNames = append(entry.deviceNames, deviceName)
			s.logger.Debug("skipping unnecessary device validation steps", zap.String("device", deviceName))
			continue
		}

		isEbs, err := s.nvme.IsEbsDevice(&device)
		if err != nil || !isEbs {
			s.logger.Debug("skipping non-ebs nvme device", zap.String("device", deviceName), zap.Error(err))
			continue
		}

		serial, err := s.nvme.GetDeviceSerial(&device)
		if err != nil {
			s.logger.Debug("unable to get serial number of device", zap.String("device", deviceName), zap.Error(err))
			continue
		}

		// The serial should begin with vol and have content after the vol prefix
		if !strings.HasPrefix(serial, "vol") || len(serial) < 4 {
			s.logger.Debug("device serial is not a valid volume id", zap.String("device", deviceName), zap.String("serial", serial))
			continue
		}

		devices[device.Controller()] = &ebsDevices{
			deviceNames: []string{deviceName},
			volumeID:    fmt.Sprintf("vol-%s", serial[3:]),
		}
	}

	return devices, nil
}

func newScraper(cfg *Config,
	settings receiver.Settings,
	nvme nvme.DeviceInfoProvider,
	allowedDevices collections.Set[string],
) *nvmeScraper {
	return &nvmeScraper{
		logger:         settings.TelemetrySettings.Logger,
		mb:             metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
		nvme:           nvme,
		allowedDevices: allowedDevices,
	}
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
