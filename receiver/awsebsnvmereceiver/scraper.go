// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsebsnvmereceiver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/receiver/awsebsnvmereceiver/internal/metadata"
	"github.com/aws/amazon-cloudwatch-agent/receiver/awsebsnvmereceiver/internal/nvme"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

type nvmeScraper struct {
	logger *zap.Logger
	mb     *metadata.MetricsBuilder
	nvme   nvme.NvmeUtilInterface
}

type ebsDevice struct {
	deviceName string
	devicePath string
	volumeId   string
}

var getMetrics = nvme.GetMetrics

func (s *nvmeScraper) start(_ context.Context, _ component.Host) error {
	s.logger.Debug("Starting NVMe scraper", zap.String("receiver", metadata.Type.String()))
	return nil
}

func (s *nvmeScraper) shutdown(_ context.Context) error {
	s.logger.Debug("Shutting down NVMe scraper", zap.String("receiver", metadata.Type.String()))
	return nil
}

func (s *nvmeScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	ebsDevices, err := s.getEbsDevices()
	if err != nil {
		return pmetric.NewMetrics(), err
	}

	now := pcommon.NewTimestampFromTime(time.Now())

	for _, device := range ebsDevices {
		metrics, err := getMetrics(device.devicePath)
		if err != nil {
			s.logger.Info("unable to get metrics for device", zap.String("device", device.deviceName), zap.Error(err))
			continue
		}

		rb := s.mb.NewResourceBuilder()
		rb.SetVolumeID(device.volumeId)

		s.mb.RecordDiskioEbsTotalReadOpsDataPoint(now, int64(metrics.ReadOps))
		s.mb.RecordDiskioEbsTotalWriteOpsDataPoint(now, int64(metrics.WriteOps))
		s.mb.RecordDiskioEbsTotalReadBytesDataPoint(now, int64(metrics.ReadBytes))
		s.mb.RecordDiskioEbsTotalWriteBytesDataPoint(now, int64(metrics.WriteBytes))
		s.mb.RecordDiskioEbsTotalReadTimeDataPoint(now, int64(metrics.TotalReadTime))
		s.mb.RecordDiskioEbsTotalWriteTimeDataPoint(now, int64(metrics.TotalWriteTime))
		s.mb.RecordDiskioEbsVolumePerformanceExceededIopsDataPoint(now, int64(metrics.EBSIOPSExceeded))
		s.mb.RecordDiskioEbsVolumePerformanceExceededTpDataPoint(now, int64(metrics.EBSThroughputExceeded))
		s.mb.RecordDiskioEbsEc2InstancePerformanceExceededIopsDataPoint(now, int64(metrics.EC2IOPSExceeded))
		s.mb.RecordDiskioEbsEc2InstancePerformanceExceededTpDataPoint(now, int64(metrics.EC2ThroughputExceeded))
		s.mb.RecordDiskioEbsVolumeQueueLengthDataPoint(now, int64(metrics.QueueLength))

		s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}

	return s.mb.Emit(), nil
}

func (s *nvmeScraper) getEbsDevices() (map[int]ebsDevice, error) {
	allNvmeDevices, err := s.nvme.GetAllDevices()
	if err != nil {
		return nil, err
	}

	devices := make(map[int]ebsDevice)

	for _, device := range allNvmeDevices {
		var deviceName string
		if name, err := device.DeviceName(); err == nil {
			deviceName = name
		}

		// nvme0, nvme1, ... nvme{n} are owned by root:root. Device files with
		// namespace (e.g. nvme0n1, nvme0n2) are owned by root:disk. We skip attempting to open the former.
		if device.Namespace() == -1 {
			s.logger.Debug("skipping invalid device", zap.String("device", deviceName))
			continue
		}

		// Skip if we already have a device file we can use
		if _, ok := devices[device.Controller()]; ok {
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

		devices[device.Controller()] = ebsDevice{
			deviceName: deviceName,
			devicePath: fmt.Sprintf("%s/%s", nvme.DevDirectoryPath, deviceName),
			volumeId:   fmt.Sprintf("vol-%s", serial[3:]),
		}
	}

	return devices, nil
}

func newScraper(cfg *Config, settings receiver.Settings, nvme nvme.NvmeUtilInterface) *nvmeScraper {
	return &nvmeScraper{
		logger: settings.TelemetrySettings.Logger,
		mb:     metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
		nvme:   nvme,
	}
}
