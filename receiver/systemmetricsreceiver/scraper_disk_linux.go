//go:build linux

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package systemmetricsreceiver

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

const (
	metricAggDiskUsed = "aggregate_disk_used"
	metricAggDiskFree = "aggregate_disk_free"
)

type diskScraper struct {
	logger *zap.Logger
	ps     PS
}

func newDiskScraper(logger *zap.Logger, ps PS) *diskScraper {
	return &diskScraper{logger: logger, ps: ps}
}

func (s *diskScraper) Name() string { return "disk" }

func (s *diskScraper) Scrape(ctx context.Context, metrics pmetric.Metrics) error {
	parts, err := s.ps.DiskUsage(ctx)
	if err != nil {
		s.logger.Debug("Failed to read disk usage", zap.Error(err))
		return nil
	}
	if len(parts) == 0 {
		return nil
	}

	var totalUsed, totalFree uint64
	for _, du := range parts {
		totalUsed += du.Used
		totalFree += du.Free
	}

	now := pcommon.NewTimestampFromTime(time.Now())
	rm := metrics.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	addGaugeDP(sm.Metrics().AppendEmpty(), metricAggDiskUsed, "Bytes", float64(totalUsed), now)
	addGaugeDP(sm.Metrics().AppendEmpty(), metricAggDiskFree, "Bytes", float64(totalFree), now)
	return nil
}
