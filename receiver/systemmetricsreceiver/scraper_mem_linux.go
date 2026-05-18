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
	metricMemTotal     = "mem_total"
	metricMemAvailable = "mem_available"
	metricMemCached    = "mem_cached"
	metricMemActive    = "mem_active"
)

type memScraper struct {
	logger *zap.Logger
	ps     PS
}

func newMemScraper(logger *zap.Logger, ps PS) *memScraper {
	return &memScraper{logger: logger, ps: ps}
}

func (s *memScraper) Name() string { return "mem" }

func (s *memScraper) Scrape(ctx context.Context, metrics pmetric.Metrics) error {
	vm, err := s.ps.VMStat(ctx)
	if err != nil {
		s.logger.Debug("Failed to read memory stats", zap.Error(err))
		return nil
	}

	now := pcommon.NewTimestampFromTime(time.Now())
	rm := metrics.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	addGaugeDP(sm.Metrics().AppendEmpty(), metricMemTotal, "Bytes", float64(vm.Total), now)
	addGaugeDP(sm.Metrics().AppendEmpty(), metricMemAvailable, "Bytes", float64(vm.Available), now)
	addGaugeDP(sm.Metrics().AppendEmpty(), metricMemCached, "Bytes", float64(vm.Cached), now)
	addGaugeDP(sm.Metrics().AppendEmpty(), metricMemActive, "Bytes", float64(vm.Active), now)
	return nil
}
