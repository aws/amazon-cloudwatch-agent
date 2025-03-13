// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsebsnvmereceiver

import (
	"context"
	"fmt"
	"os"
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

const (
	nvmeDevicePrefix     = "nvme"
	devDirectoryPath     = "/dev"
	nvmeSysDirectoryPath = "/sys/class/nvme"
)

type nvmeScraper struct {
	logger *zap.Logger
	mb     *metadata.MetricsBuilder
}

// TODO: move this into internal/nvme probably
type ebsDevice struct {
	deviceName string
	devicePath string
	volumeId   string
}

// TODO: probably do not need start & shutdown logic
func (s *nvmeScraper) start(_ context.Context, _ component.Host) error {
	s.logger.Debug("Starting NVMe scraper", zap.String("receiver", metadata.Type.String()))
	return nil
}

func (s *nvmeScraper) shutdown(_ context.Context) error {
	s.logger.Debug("Shutting down NVMe scraper", zap.String("receiver", metadata.Type.String()))
	return nil
}

// TODO: clean up log messages
func (s *nvmeScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	now := pcommon.NewTimestampFromTime(time.Now())

	ebsDevices, err := s.getEbsDevices()
	if err != nil {
		return pmetric.NewMetrics(), err
	}

	for _, device := range ebsDevices {
		metrics, err := nvme.GetMetrics(device.devicePath)
		if err != nil {
			s.logger.Info("unable to get metrics for device", zap.String("device", device.deviceName), zap.Error(err))
			continue
		}
		s.logger.Info("emitting metrics for device", zap.String("device", device.deviceName), zap.Int("read_ops", int(metrics.ReadOps)), zap.String("volumeId", device.volumeId))

		rb := s.mb.NewResourceBuilder()
		s.mb.RecordTotalReadOpsDataPoint(now, int64(metrics.ReadOps))
		rb.SetVolumeID(device.volumeId)
		s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}

	return s.mb.Emit(), nil
}

func (s *nvmeScraper) getEbsDevices() (map[int]ebsDevice, error) {
	allNvmeDevices, err := getNvmeDevices()
	if err != nil {
		return nil, err
	}

	devices := make(map[int]ebsDevice)

	for _, device := range allNvmeDevices {
		attr, err := nvme.ParseNvmeDeviceFileName(device)
		if err != nil || attr.Controller() == -1 || attr.Namespace() == -1 {
			s.logger.Debug("skipping invalid device", zap.String("device", device))
			continue
		}

		// Skip if we already have a device we can use
		if _, ok := devices[attr.Controller()]; ok {
			continue
		}

		// TODO: we can probably add a check for the model too

		serial, err := getNvmeDeviceSerial(fmt.Sprintf("nvme%d", attr.Controller()))
		if err != nil {
			s.logger.Debug("unable to get serial number of device", zap.String("device", device))
			continue
		}

		if serial[:3] != "vol" {
			s.logger.Debug("device is not prefixed with vol", zap.String("device", device))
			continue
		}

		devices[attr.Controller()] = ebsDevice{
			deviceName: device,
			devicePath: fmt.Sprintf("/dev/%s", device),
			volumeId:   fmt.Sprintf("vol-%s", serial[2:]),
		}
	}

	return devices, nil
}

// TODO: move to internal/nvme/
func getNvmeDevices() ([]string, error) {
	entries, err := os.ReadDir(devDirectoryPath)
	if err != nil {
		return nil, err
	}

	devices := []string{}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), nvmeDevicePrefix) {
			devices = append(devices, entry.Name())
		}
	}

	return devices, nil
}

// TODO: move to internal/nvme/
func getNvmeDeviceSerial(device string) (string, error) {
	data, err := os.ReadFile(fmt.Sprintf("/sys/class/nvme/%s/serial", device))
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(data), "\n"), nil
}

// Use mountinfo to get NVMe devices
// func getNvmeDevices() ([]string, error) {
// 	devices := []string{}
//
// 	infos, err := mountinfo.GetMounts(sourceFilter(nvmeDevicePrefix))
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	for _, info := range infos {
// 		devices = append(devices, info.Source)
// 	}
//
// 	return devices, nil
// }
// func sourceFilter(prefix string) mountinfo.FilterFunc {
// 	return func(m *mountinfo.Info) (bool, bool) {
// 		skip := !strings.HasPrefix(m.Source, prefix)
// 		return skip, false
// 	}
// }

// Another option is to read `/sys/class/nvme/nvme{id}`. Inside has a serial file
// which will have the volume ID
// func getNvmeDevices() ([]string, error) {
// }

func newScraper(cfg *Config, settings receiver.Settings) *nvmeScraper {
	return &nvmeScraper{
		logger: settings.TelemetrySettings.Logger,
		mb:     metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
	}
}
