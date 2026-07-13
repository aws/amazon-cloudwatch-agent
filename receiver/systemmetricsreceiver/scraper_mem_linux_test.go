//go:build linux

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package systemmetricsreceiver

import (
	"context"
	"errors"
	"testing"

	"github.com/shirou/gopsutil/v3/mem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func TestMemScraperName(t *testing.T) {
	s := newMemScraper(zap.NewNop(), &MockPS{})
	assert.Equal(t, "mem", s.Name())
}

func TestMemScraperMetrics(t *testing.T) {
	ps := &MockPS{VMStatData: &mem.VirtualMemoryStat{
		Total:     8 * 1048576 * 1024, // 8 GB
		Available: 6 * 1048576 * 1024, // 6 GB
		Cached:    2 * 1048576 * 1024, // 2 GB
		Active:    3 * 1048576 * 1024, // 3 GB
	}}
	s := newMemScraper(zap.NewNop(), ps)

	metrics := pmetric.NewMetrics()
	require.NoError(t, s.Scrape(context.Background(), metrics))

	require.Equal(t, 1, metrics.ResourceMetrics().Len())
	sm := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0)
	require.Equal(t, 4, sm.Metrics().Len())

	assert.Equal(t, "mem_total", sm.Metrics().At(0).Name())
	assert.Equal(t, "Bytes", sm.Metrics().At(0).Unit())
	assert.InDelta(t, 8*1048576*1024.0, sm.Metrics().At(0).Gauge().DataPoints().At(0).DoubleValue(), 0.01)

	assert.Equal(t, "mem_available", sm.Metrics().At(1).Name())
	assert.Equal(t, "Bytes", sm.Metrics().At(1).Unit())
	assert.InDelta(t, 6*1048576*1024.0, sm.Metrics().At(1).Gauge().DataPoints().At(0).DoubleValue(), 0.01)

	assert.Equal(t, "mem_cached", sm.Metrics().At(2).Name())
	assert.Equal(t, "Bytes", sm.Metrics().At(2).Unit())
	assert.InDelta(t, 2*1048576*1024.0, sm.Metrics().At(2).Gauge().DataPoints().At(0).DoubleValue(), 0.01)

	assert.Equal(t, "mem_active", sm.Metrics().At(3).Name())
	assert.Equal(t, "Bytes", sm.Metrics().At(3).Unit())
	assert.InDelta(t, 3*1048576*1024.0, sm.Metrics().At(3).Gauge().DataPoints().At(0).DoubleValue(), 0.01)
}

func TestMemScraperErrorSkips(t *testing.T) {
	ps := &MockPS{VMStatErr: errors.New("permission denied")}
	s := newMemScraper(zap.NewNop(), ps)

	metrics := pmetric.NewMetrics()
	require.NoError(t, s.Scrape(context.Background(), metrics))
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())
}
