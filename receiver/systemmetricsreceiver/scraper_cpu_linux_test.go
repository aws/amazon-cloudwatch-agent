//go:build linux

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package systemmetricsreceiver

import (
	"context"
	"testing"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func TestCPUScraperName(t *testing.T) {
	s := newCPUScraper(zap.NewNop(), &MockPS{})
	assert.Equal(t, "cpu", s.Name())
}

func TestCPUScraperFirstScrapeSkips(t *testing.T) {
	ps := &MockPS{CPUTimesData: []cpu.TimesStat{
		{CPU: "cpu-total", User: 100, System: 50, Idle: 800, Iowait: 10, Nice: 5, Irq: 3, Softirq: 2, Steal: 1},
	}}
	s := newCPUScraper(zap.NewNop(), ps)

	metrics := pmetric.NewMetrics()
	require.NoError(t, s.Scrape(context.Background(), metrics))
	assert.Equal(t, 0, metrics.ResourceMetrics().Len(), "first scrape should emit nothing")
}

func TestCPUScraperIOWaitTime(t *testing.T) {
	ps := &MockPS{}
	s := newCPUScraper(zap.NewNop(), ps)

	// First scrape: seed previous stats
	ps.CPUTimesData = []cpu.TimesStat{
		{CPU: "cpu-total", User: 100, System: 50, Idle: 800, Iowait: 10, Nice: 5, Irq: 3, Softirq: 2, Steal: 1},
	}
	metrics := pmetric.NewMetrics()
	require.NoError(t, s.Scrape(context.Background(), metrics))

	// Second scrape: total delta = 100, iowait delta = 5 → 5%
	ps.CPUTimesData = []cpu.TimesStat{
		{CPU: "cpu-total", User: 120, System: 60, Idle: 860, Iowait: 15, Nice: 7, Irq: 5, Softirq: 3, Steal: 1},
	}
	metrics = pmetric.NewMetrics()
	require.NoError(t, s.Scrape(context.Background(), metrics))

	require.Equal(t, 1, metrics.ResourceMetrics().Len())
	m := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	assert.Equal(t, "cpu_time_iowait", m.Name())
	assert.Equal(t, "Percent", m.Unit())
	assert.InDelta(t, 5.0, m.Gauge().DataPoints().At(0).DoubleValue(), 0.01)
}

func TestCPUScraperZeroDeltaSkips(t *testing.T) {
	stat := cpu.TimesStat{CPU: "cpu-total", User: 100, System: 50, Idle: 800, Iowait: 10, Nice: 5, Irq: 3, Softirq: 2, Steal: 1}
	ps := &MockPS{CPUTimesData: []cpu.TimesStat{stat}}
	s := newCPUScraper(zap.NewNop(), ps)

	// Seed
	metrics := pmetric.NewMetrics()
	require.NoError(t, s.Scrape(context.Background(), metrics))

	// Same values → zero delta → skip
	metrics = pmetric.NewMetrics()
	require.NoError(t, s.Scrape(context.Background(), metrics))
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())
}

func TestCPUScraperNegativeDeltaSkips(t *testing.T) {
	ps := &MockPS{}
	s := newCPUScraper(zap.NewNop(), ps)

	// Seed with higher values
	ps.CPUTimesData = []cpu.TimesStat{
		{CPU: "cpu-total", User: 200, System: 100, Idle: 800, Iowait: 20, Nice: 10, Irq: 5, Softirq: 3, Steal: 2},
	}
	metrics := pmetric.NewMetrics()
	require.NoError(t, s.Scrape(context.Background(), metrics))

	// Lower values (suspend/resume) → negative delta → skip
	ps.CPUTimesData = []cpu.TimesStat{
		{CPU: "cpu-total", User: 50, System: 20, Idle: 100, Iowait: 5, Nice: 2, Irq: 1, Softirq: 1, Steal: 0},
	}
	metrics = pmetric.NewMetrics()
	require.NoError(t, s.Scrape(context.Background(), metrics))
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())
}

func TestCPUScraperNegativeIowaitDeltaSkips(t *testing.T) {
	ps := &MockPS{}
	s := newCPUScraper(zap.NewNop(), ps)

	// Seed
	ps.CPUTimesData = []cpu.TimesStat{
		{CPU: "cpu-total", User: 100, System: 50, Idle: 800, Iowait: 20, Nice: 5, Irq: 3, Softirq: 2, Steal: 1},
	}
	metrics := pmetric.NewMetrics()
	require.NoError(t, s.Scrape(context.Background(), metrics))

	// iowait decreases but total increases (counter reset on iowait only)
	ps.CPUTimesData = []cpu.TimesStat{
		{CPU: "cpu-total", User: 120, System: 60, Idle: 870, Iowait: 5, Nice: 7, Irq: 5, Softirq: 3, Steal: 1},
	}
	metrics = pmetric.NewMetrics()
	require.NoError(t, s.Scrape(context.Background(), metrics))
	assert.Equal(t, 0, metrics.ResourceMetrics().Len(), "negative iowait delta should be skipped")
}

func TestCPUScraperEmptyTimesSkips(t *testing.T) {
	ps := &MockPS{CPUTimesData: []cpu.TimesStat{}}
	s := newCPUScraper(zap.NewNop(), ps)

	metrics := pmetric.NewMetrics()
	require.NoError(t, s.Scrape(context.Background(), metrics))
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())
}
