// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsebsnvmereceiver

import (
	"context"

	"github.com/aws/amazon-cloudwatch-agent/receiver/awsebsnvmereceiver/internal/metadata"
	"github.com/aws/amazon-cloudwatch-agent/receiver/awsebsnvmereceiver/internal/nvme"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	otelscraper "go.opentelemetry.io/collector/scraper"
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		scraperhelper.NewDefaultControllerConfig(),
		metadata.DefaultMetricsBuilderConfig(),
	}
}

func createMetricsReceiver(
	_ context.Context,
	settings receiver.Settings,
	baseCfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	cfg := baseCfg.(*Config)
	nvmeScraper := newScraper(cfg, settings, &nvme.NvmeUtil{})
	scraper, err := otelscraper.NewMetrics(nvmeScraper.scrape, otelscraper.WithStart(nvmeScraper.start), otelscraper.WithShutdown(nvmeScraper.shutdown))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(
		&cfg.ControllerConfig, settings, consumer,
		scraperhelper.AddScraper(metadata.Type, scraper),
	)
}
