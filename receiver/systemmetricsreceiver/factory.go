// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package systemmetricsreceiver

import (
	"context"
	"math/rand"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	otelscraper "go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
)

const (
	typeStr                   = "systemmetrics"
	defaultCollectionInterval = 60 * time.Second
	maxInitialJitter          = 60 * time.Second
	stability                 = component.StabilityLevelAlpha
)

var Type = component.MustNewType(typeStr)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, stability),
	)
}

func createDefaultConfig() component.Config {
	cfg := &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
	}
	cfg.CollectionInterval = defaultCollectionInterval
	return cfg
}

func createMetricsReceiver(
	_ context.Context,
	settings receiver.Settings,
	baseCfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	cfg := baseCfg.(*Config)
	// Jitter the initial delay to stagger scrape start across hosts
	cfg.InitialDelay = cfg.InitialDelay + time.Duration(rand.Int63n(int64(maxInitialJitter))) //nolint:gosec
	s := newScraper(settings.Logger)
	scraper, err := otelscraper.NewMetrics(s.scrape, otelscraper.WithStart(s.start), otelscraper.WithShutdown(s.shutdown))
	if err != nil {
		return nil, err
	}
	return scraperhelper.NewMetricsController(
		&cfg.ControllerConfig, settings, consumer,
		scraperhelper.AddScraper(Type, scraper),
	)
}
