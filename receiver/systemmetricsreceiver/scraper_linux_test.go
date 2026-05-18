//go:build linux

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package systemmetricsreceiver

import (
	"context"
	"testing"

	"github.com/shirou/gopsutil/v3/mem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestSystemScraperOrchestration(t *testing.T) {
	s := newScraper(zap.NewNop())
	assert.NotNil(t, s.scrapers)
	assert.Equal(t, 5, len(s.scrapers), "should have cpu, mem, disk, ethtool, jvm scrapers")
}

func TestSystemScraperAlwaysEmitsSystemMetrics(t *testing.T) {
	s := &hostScraper{
		logger: zap.NewNop(),
		scrapers: []MetricScraper{newMemScraper(zap.NewNop(), &MockPS{
			VMStatData: &mem.VirtualMemoryStat{Total: 1000, Available: 500, Cached: 200, Active: 300},
		})},
		ps: &SystemPS{},
	}

	metrics, err := s.scrape(context.Background())
	require.NoError(t, err)
	assert.Greater(t, metrics.ResourceMetrics().Len(), 0, "system metrics should always be emitted")
}
