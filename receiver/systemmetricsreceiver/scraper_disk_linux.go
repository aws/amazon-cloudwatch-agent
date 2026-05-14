//go:build linux

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package systemmetricsreceiver

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/v3/disk"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

const (
	metricAggDiskUsed = "aggregate_disk_used"
	metricAggDiskFree = "aggregate_disk_free"

	// maxPlausibleMountBytes is the upper bound for a single mount's stats (1 PiB).
	// The largest EBS volume is 64 TiB, so this is ~16x above anything legitimate.
	// The gopsutil disk.UsageWithContext computes Total/Free as uint64(stat.Blocks) *
	// uint64(stat.Bsize). Certain filesystem states (transient loop mounts, broken
	// statvfs under FS pressure) can produce huge garbage values.
	maxPlausibleMountBytes uint64 = 1 << 50
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
		if !isPlausibleDiskUsage(du) {
			s.logger.Debug("Dropping disk sample: mount with implausible stats",
				zap.String("path", du.Path),
				zap.Uint64("total", du.Total),
				zap.Uint64("free", du.Free),
				zap.Uint64("used", du.Used))
			return nil // drop entire sample to avoid poisoning min/max rollups
		}
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

// isPlausibleDiskUsage returns true if the usage stats are physically plausible.
// Rejects mounts where statvfs returned garbage (huge values, Free > Total, etc).
func isPlausibleDiskUsage(du *disk.UsageStat) bool {
	return du.Total > 0 &&
		du.Free <= du.Total &&
		du.Total <= maxPlausibleMountBytes &&
		du.Free <= maxPlausibleMountBytes
}
