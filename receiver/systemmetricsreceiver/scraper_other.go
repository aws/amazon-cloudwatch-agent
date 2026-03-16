//go:build !linux

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package systemmetricsreceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type hostScraper struct {
	logger *zap.Logger
}

func newScraper(logger *zap.Logger) *hostScraper {
	return &hostScraper{logger: logger}
}

func (s *hostScraper) start(_ context.Context, _ component.Host) error {
	s.logger.Info("System metrics scraper not supported on this platform")
	return nil
}

func (s *hostScraper) shutdown(_ context.Context) error {
	return nil
}

func (s *hostScraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	return pmetric.NewMetrics(), nil
}
