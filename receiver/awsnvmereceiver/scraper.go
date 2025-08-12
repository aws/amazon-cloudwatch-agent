// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsnvmereceiver

import (
	"context"
	"fmt"
	"math"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	"github.com/aws/amazon-cloudwatch-agent/receiver/awsnvmereceiver/internal/ebs"
	"github.com/aws/amazon-cloudwatch-agent/receiver/awsnvmereceiver/internal/instancestore"
	"github.com/aws/amazon-cloudwatch-agent/receiver/awsnvmereceiver/internal/metadata"
	"github.com/aws/amazon-cloudwatch-agent/receiver/awsnvmereceiver/internal/nvme"
)

// DeviceTypeScraper defines type-specific behavior for scraping EBS or Instance Store metrics.
type DeviceTypeScraper interface {
	Model() string
	DeviceType() string
	Identifier(serial string) (string, error)
	SetResourceAttribute(rb *metadata.ResourceBuilder, identifier string)
	// RecordMetrics accepts a typed record function and the typed NVMe metrics interface.
	RecordMetrics(recordMetric nvme.RecordMetricFunc, mb *metadata.MetricsBuilder, ts pcommon.Timestamp, metrics nvme.NVMeMetrics)
	IsEnabled(m *metadata.MetricsConfig) bool
	ParseRawData(data []byte) (nvme.NVMeMetrics, error)
}

type nvmeScraper struct {
	logger *zap.Logger
	mb     *metadata.MetricsBuilder
	nvme   nvme.DeviceInfoProvider

	allowedDevices  collections.Set[string]
	typeScrapers    []DeviceTypeScraper
	scrapersByModel map[string]DeviceTypeScraper
}

type nvmeDevices struct {
	scraper     DeviceTypeScraper
	deviceType  string
	identifier  string
	deviceNames []string
}

// For unit testing
var getRawData = nvme.GetRawData

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

	// Controllers are grouped by ID because metrics (from log pages) are aggregated at the controller level and identical across all namespaces/partitions on that controller.
	// The outer loop processes each unique controller group to emit metrics once per controller.
	// The inner loop tries each namespace path in the group sequentially until we find one accessible via ioctl, then emits and breaks.
	// This avoids duplication while handling permission issues on some namespaces.
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
			rawData, err := getRawData(devicePath)
			if err != nil {
				s.logger.Debug("unable to get raw data for device", zap.String("device", device), zap.Error(err))
				continue
			}

			metrics, err := nvmeDevices.scraper.ParseRawData(rawData)
			if err != nil {
				s.logger.Debug("unable to parse raw data for device", zap.String("device", device), zap.Error(err))
				continue
			}

			foundWorkingDevice = true

			rb := s.mb.NewResourceBuilder()
			nvmeDevices.scraper.SetResourceAttribute(rb, nvmeDevices.identifier)
			nvmeDevices.scraper.RecordMetrics(s.recordMetric, s.mb, now, metrics)

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

		// Check if device is allowed (either wildcard "*" or explicitly allowed)
		if !s.allowedDevices.Contains("*") && !s.allowedDevices.Contains(deviceName) {
			s.logger.Debug("skipping un-allowed device", zap.String("device", deviceName))
			continue
		}

		controllerID := device.Controller()

		// If we already processed this controller, just add deviceName to list and continue
		if entry, exists := devices[controllerID]; exists {
			entry.deviceNames = append(entry.deviceNames, deviceName)
			s.logger.Debug("skipping unnecessary device validation steps", zap.String("device", deviceName))
			continue
		}

		serial, err := s.nvme.GetDeviceSerial(&device)
		if err != nil {
			s.logger.Debug("unable to get serial number", zap.String("device", deviceName), zap.Error(err))
			continue
		}

		model, err := s.nvme.GetDeviceModel(&device)
		if err != nil {
			s.logger.Debug("unable to get device model", zap.String("device", deviceName), zap.Error(err))
			continue
		}

		scraper, ok := s.scrapersByModel[model]
		if !ok {
			s.logger.Debug("skipping unknown device model", zap.String("device", deviceName), zap.String("model", model))
			continue
		}

		identifier, err := scraper.Identifier(serial)
		if err != nil {
			s.logger.Debug("invalid identifier for model, skipping device", zap.String("device", deviceName), zap.String("serial", serial), zap.Error(err))
			continue
		}

		// Add device grouped by controller
		devices[controllerID] = &nvmeDevices{
			scraper:     scraper,
			deviceType:  scraper.DeviceType(),
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

	allScrapers := []DeviceTypeScraper{
		ebs.NewScraper(),
		instancestore.NewScraper(),
	}

	var enabledScrapers []DeviceTypeScraper
	for _, ts := range allScrapers {
		if ts.IsEnabled(&cfg.MetricsBuilderConfig.Metrics) {
			enabledScrapers = append(enabledScrapers, ts)
		}
	}
	scraper.typeScrapers = enabledScrapers

	// build fast lookup map model -> scraper
	scraper.scrapersByModel = make(map[string]DeviceTypeScraper, len(enabledScrapers))
	for _, ts := range enabledScrapers {
		scraper.scrapersByModel[ts.Model()] = ts
	}

	return scraper
}

func (s *nvmeScraper) recordMetric(recordFn func(pcommon.Timestamp, int64), ts pcommon.Timestamp, val uint64) {
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
