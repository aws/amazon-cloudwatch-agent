//go:build linux

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package systemmetricsreceiver

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

const metricCPUIOWaitTime = "cpu_time_iowait"

type cpuScraper struct {
	logger   *zap.Logger
	ps       PS
	prevStat *cpu.TimesStat
}

func newCPUScraper(logger *zap.Logger, ps PS) *cpuScraper {
	return &cpuScraper{
		logger: logger,
		ps:     ps,
	}
}

func (s *cpuScraper) Name() string { return "cpu" }

func (s *cpuScraper) Scrape(ctx context.Context, metrics pmetric.Metrics) error {
	times, err := s.ps.CPUTimes(ctx)
	if err != nil {
		s.logger.Debug("Failed to read CPU times", zap.Error(err))
		return nil
	}
	if len(times) == 0 {
		return nil
	}

	cur := times[0]

	if s.prevStat == nil {
		s.prevStat = &cur
		return nil
	}

	prev := s.prevStat
	s.prevStat = &cur

	curTotal := cpuTotal(cur)
	prevTotal := cpuTotal(*prev)
	deltaTotal := curTotal - prevTotal
	if deltaTotal <= 0 {
		return nil
	}

	iowaitDelta := cur.Iowait - prev.Iowait
	if iowaitDelta < 0 {
		return nil
	}
	iowaitPct := 100 * iowaitDelta / deltaTotal

	now := pcommon.NewTimestampFromTime(time.Now())
	rm := metrics.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	addGaugeDP(sm.Metrics().AppendEmpty(), metricCPUIOWaitTime, "Percent", iowaitPct, now)
	return nil
}

// cpuTotal sums the 8 base CPU states (excludes Guest/GuestNice to avoid double-counting).
func cpuTotal(t cpu.TimesStat) float64 {
	return t.User + t.System + t.Nice + t.Iowait + t.Irq + t.Softirq + t.Steal + t.Idle
}
