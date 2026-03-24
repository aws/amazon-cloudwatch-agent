//go:build linux

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package systemmetricsreceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// MetricScraper defines the interface for individual metric scrapers.
// Each scraper is responsible for collecting a specific type of metrics.
type MetricScraper interface {
	// Name returns the scraper identifier for logging/debugging.
	Name() string
	// Scrape collects metrics and appends them to the provided pmetric.Metrics.
	Scrape(ctx context.Context, metrics pmetric.Metrics) error
}

// hostScraper orchestrates multiple MetricScrapers.
type hostScraper struct {
	logger   *zap.Logger
	scrapers []MetricScraper
	ps       *SystemPS
}

func newScraper(logger *zap.Logger) *hostScraper {
	stats := &SystemPS{}
	return &hostScraper{
		logger: logger,
		scrapers: []MetricScraper{
			newCPUScraper(logger, stats),
			newMemScraper(logger, stats),
			newDiskScraper(logger, stats),
			newEthtoolScraper(logger, stats),
			newJVMScraper(logger),
		},
		ps: stats,
	}
}

func (s *hostScraper) start(_ context.Context, _ component.Host) error {
	s.logger.Info("Starting system metrics scraper", zap.Int("scraper_count", len(s.scrapers)))
	return nil
}

func (s *hostScraper) shutdown(_ context.Context) error {
	s.logger.Info("Shutting down system metrics scraper")
	s.ps.Close()
	return nil
}

func (s *hostScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	metrics := pmetric.NewMetrics()

	for _, scraper := range s.scrapers {
		if err := scraper.Scrape(ctx, metrics); err != nil {
			s.logger.Warn("Scraper failed", zap.String("scraper", scraper.Name()), zap.Error(err))
		}
	}

	return metrics, nil
}

func addGaugeDP(m pmetric.Metric, name string, unit string, value float64, now pcommon.Timestamp) {
	m.SetName(name)
	m.SetUnit(unit)
	g := m.SetEmptyGauge()
	dp := g.DataPoints().AppendEmpty()
	dp.SetTimestamp(now)
	dp.SetDoubleValue(value)
}
