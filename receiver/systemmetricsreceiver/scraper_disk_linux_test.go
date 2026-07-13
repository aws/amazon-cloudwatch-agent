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
		{Path: "/", Used: 50 * 1048576, Free: 78 * 1048576},       // 50 MB used, 78 MB free
		{Path: "/home", Used: 100 * 1048576, Free: 200 * 1048576}, // 100 MB used, 200 MB free
	}}
	s := newDiskScraper(zap.NewNop(), ps)

	metrics := pmetric.NewMetrics()
	require.NoError(t, s.Scrape(context.Background(), metrics))

	require.Equal(t, 1, metrics.ResourceMetrics().Len())
	sm := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0)
	require.Equal(t, 2, sm.Metrics().Len())

	// DiskUsed = (50 + 100) * 1048576 bytes
	assert.Equal(t, "aggregate_disk_used", sm.Metrics().At(0).Name())
	assert.Equal(t, "Bytes", sm.Metrics().At(0).Unit())
	assert.InDelta(t, 150*1048576.0, sm.Metrics().At(0).Gauge().DataPoints().At(0).DoubleValue(), 0.01)

	// DiskFree = (78 + 200) * 1048576 bytes
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
