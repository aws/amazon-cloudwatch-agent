//go:build linux

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package systemmetricsreceiver

import (
	"context"
	"errors"
	"testing"

	"github.com/shirou/gopsutil/v3/disk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func TestDiskScraperName(t *testing.T) {
	s := newDiskScraper(zap.NewNop(), &MockPS{})
	assert.Equal(t, "disk", s.Name())
}

func TestDiskScraperMetrics(t *testing.T) {
	ps := &MockPS{DiskUsageData: []*disk.UsageStat{
		{Path: "/", Total: 128 * 1048576, Used: 50 * 1048576, Free: 78 * 1048576},
		{Path: "/home", Total: 300 * 1048576, Used: 100 * 1048576, Free: 200 * 1048576},
	}}
	s := newDiskScraper(zap.NewNop(), ps)

	metrics := pmetric.NewMetrics()
	require.NoError(t, s.Scrape(context.Background(), metrics))

	require.Equal(t, 1, metrics.ResourceMetrics().Len())
	sm := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0)
	require.Equal(t, 2, sm.Metrics().Len())

	assert.Equal(t, "aggregate_disk_used", sm.Metrics().At(0).Name())
	assert.Equal(t, "Bytes", sm.Metrics().At(0).Unit())
	assert.InDelta(t, 150*1048576.0, sm.Metrics().At(0).Gauge().DataPoints().At(0).DoubleValue(), 0.01)

	assert.Equal(t, "aggregate_disk_free", sm.Metrics().At(1).Name())
	assert.Equal(t, "Bytes", sm.Metrics().At(1).Unit())
	assert.InDelta(t, 278*1048576.0, sm.Metrics().At(1).Gauge().DataPoints().At(0).DoubleValue(), 0.01)
}

func TestDiskScraperNoPartitionsSkips(t *testing.T) {
	ps := &MockPS{DiskUsageData: []*disk.UsageStat{}}
	s := newDiskScraper(zap.NewNop(), ps)

	metrics := pmetric.NewMetrics()
	require.NoError(t, s.Scrape(context.Background(), metrics))
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())
}

func TestDiskScraperErrorSkips(t *testing.T) {
	ps := &MockPS{DiskUsageErr: errors.New("permission denied")}
	s := newDiskScraper(zap.NewNop(), ps)

	metrics := pmetric.NewMetrics()
	require.NoError(t, s.Scrape(context.Background(), metrics))
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())
}

// ---- plausibility guard tests ----

func TestDiskScraperDropsSampleWhenFreeExceedsTotal(t *testing.T) {
	// A transient mount reports tiny total but huge free from a
	// statvfs signed-multiply overflow.
	ps := &MockPS{DiskUsageData: []*disk.UsageStat{
		{Path: "/", Total: 1_000_000_000, Used: 500_000_000, Free: 500_000_000},
		{Path: "/bad", Total: 4096, Used: 0, Free: 9_223_372_036_854_755_328},
	}}
	s := newDiskScraper(zap.NewNop(), ps)

	metrics := pmetric.NewMetrics()
	require.NoError(t, s.Scrape(context.Background(), metrics))
	assert.Equal(t, 0, metrics.ResourceMetrics().Len(), "sample must be dropped when any mount has Free > Total")
}

func TestDiskScraperDropsSampleWhenTotalExceedsCap(t *testing.T) {
	ps := &MockPS{DiskUsageData: []*disk.UsageStat{
		{Path: "/", Total: 1_000_000_000, Used: 500_000_000, Free: 500_000_000},
		{Path: "/huge", Total: 1 << 51, Used: 1 << 50, Free: 1 << 50},
	}}
	s := newDiskScraper(zap.NewNop(), ps)

	metrics := pmetric.NewMetrics()
	require.NoError(t, s.Scrape(context.Background(), metrics))
	assert.Equal(t, 0, metrics.ResourceMetrics().Len(), "sample must be dropped when mount exceeds 1 PiB cap")
}

func TestDiskScraperDropsSampleWhenTotalIsZero(t *testing.T) {
	ps := &MockPS{DiskUsageData: []*disk.UsageStat{
		{Path: "/", Total: 1_000_000_000, Used: 500_000_000, Free: 500_000_000},
		{Path: "/broken", Total: 0, Used: 0, Free: 0},
	}}
	s := newDiskScraper(zap.NewNop(), ps)

	metrics := pmetric.NewMetrics()
	require.NoError(t, s.Scrape(context.Background(), metrics))
	assert.Equal(t, 0, metrics.ResourceMetrics().Len(), "sample must be dropped when mount has zero total")
}

func TestIsPlausibleDiskUsage(t *testing.T) {
	tests := []struct {
		name   string
		usage  *disk.UsageStat
		expect bool
	}{
		{"normal", &disk.UsageStat{Total: 1e9, Free: 5e8}, true},
		{"full disk", &disk.UsageStat{Total: 1e9, Free: 0}, true},
		{"at 1 PiB cap", &disk.UsageStat{Total: 1 << 50, Free: 1 << 50}, true},
		{"zero total", &disk.UsageStat{Total: 0, Free: 0}, false},
		{"free > total", &disk.UsageStat{Total: 4096, Free: 9_223_372_036_854_755_328}, false},
		{"above cap", &disk.UsageStat{Total: 1<<50 + 1, Free: 0}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, isPlausibleDiskUsage(tt.usage))
		})
	}
}
