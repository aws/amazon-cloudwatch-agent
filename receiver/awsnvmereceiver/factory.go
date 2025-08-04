// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsnvmereceiver

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	otelscraper "go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/internal/nvme"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	"github.com/aws/amazon-cloudwatch-agent/receiver/awsnvmereceiver/internal/metadata"
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{},
	}
}

func createMetricsReceiver(
	_ context.Context,
	settings receiver.Settings,
	baseCfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	cfg := baseCfg.(*Config)

	// Check platform support early and provide graceful degradation
	nvmeUtil := &nvme.Util{}
	if !isPlatformSupported(nvmeUtil, settings.Logger) {
		settings.Logger.Info("NVMe metrics collection is not supported on this platform, receiver will be disabled",
			zap.String("receiver", metadata.Type.String()))

		// Return a no-op receiver that doesn't fail but doesn't collect metrics
		return newNoOpReceiver(settings, consumer)
	}

	nvmeScraper := newScraper(cfg, settings, nvmeUtil, collections.NewSet(cfg.Devices...))
	scraper, err := otelscraper.NewMetrics(nvmeScraper.scrape, otelscraper.WithStart(nvmeScraper.start), otelscraper.WithShutdown(nvmeScraper.shutdown))
	if err != nil {
		return nil, fmt.Errorf("failed to create NVMe metrics scraper: %w", err)
	}

	return scraperhelper.NewMetricsController(
		&cfg.ControllerConfig, settings, consumer,
		scraperhelper.AddScraper(metadata.Type, scraper),
	)
}

// isPlatformSupported checks if the current platform supports NVMe operations
func isPlatformSupported(nvmeUtil nvme.DeviceInfoProvider, logger *zap.Logger) bool {
	// Try a simple device discovery to check platform support
	_, err := nvmeUtil.GetAllDevices()
	if err != nil {
		// Check if this is a platform support error
		if strings.Contains(err.Error(), "only supported on Linux") {
			logger.Debug("platform does not support NVMe operations", zap.Error(err))
			return false
		}
		// Other errors don't necessarily mean platform is unsupported
		logger.Debug("error during platform support check, assuming supported", zap.Error(err))
	}
	return true
}

// noOpReceiver is a receiver that does nothing but satisfies the interface
type noOpReceiver struct {
	settings receiver.Settings
	consumer consumer.Metrics
}

// newNoOpReceiver creates a no-op receiver for unsupported platforms
func newNoOpReceiver(settings receiver.Settings, consumer consumer.Metrics) (receiver.Metrics, error) {
	return &noOpReceiver{
		settings: settings,
		consumer: consumer,
	}, nil
}

func (r *noOpReceiver) Start(ctx context.Context, host component.Host) error {
	r.settings.Logger.Debug("no-op NVMe receiver started (platform not supported)")
	return nil
}

func (r *noOpReceiver) Shutdown(ctx context.Context) error {
	r.settings.Logger.Debug("no-op NVMe receiver shutdown")
	return nil
}

// Note: The actual scraper implementation is in scraper.go
// This factory creates the scraper instance and wires it into the OpenTelemetry pipeline
