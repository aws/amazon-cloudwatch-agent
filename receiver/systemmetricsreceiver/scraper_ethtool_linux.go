//go:build linux

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package systemmetricsreceiver

import (
	"context"
	"net"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

const (
	// ENA driver stat names (as returned by ethtool -S).
	enaBwInAllowanceExceeded  = "bw_in_allowance_exceeded"
	enaBwOutAllowanceExceeded = "bw_out_allowance_exceeded"
	enaPpsAllowanceExceeded   = "pps_allowance_exceeded"

	// Published aggregate metric names.
	metricAggBwIn  = "aggregate_bw_in_allowance_exceeded"
	metricAggBwOut = "aggregate_bw_out_allowance_exceeded"
	metricAggPps   = "aggregate_pps_allowance_exceeded"
)

// enaMetricNames maps ENA driver stat names to published aggregate metric names.
var enaMetricNames = map[string]string{
	enaBwInAllowanceExceeded:  metricAggBwIn,
	enaBwOutAllowanceExceeded: metricAggBwOut,
	enaPpsAllowanceExceeded:   metricAggPps,
}

type ethtoolScraper struct {
	logger     *zap.Logger
	ps         PS
	listIfaces func() ([]net.Interface, error)
	prevStats  map[string]map[string]uint64
}

func newEthtoolScraper(logger *zap.Logger, ps PS) *ethtoolScraper {
	return &ethtoolScraper{logger: logger, ps: ps, listIfaces: net.Interfaces}
}

func (s *ethtoolScraper) Name() string { return "ethtool" }

func (s *ethtoolScraper) Scrape(ctx context.Context, metrics pmetric.Metrics) error {
	ifaces, err := s.listIfaces()
	if err != nil {
		s.logger.Debug("Failed to list network interfaces", zap.Error(err))
		return nil
	}

	now := pcommon.NewTimestampFromTime(time.Now())
	curStats := make(map[string]map[string]uint64)
	aggDeltas := make(map[string]uint64)

	for _, iface := range ifaces {
		if skipInterface(iface) {
			continue
		}
		stats, err := s.ps.EthtoolStats(ctx, iface.Name)
		if err != nil {
			s.logger.Debug("Failed to read ethtool stats", zap.String("interface", iface.Name), zap.Error(err))
			continue
		}

		// Save current allowance counters for next delta computation.
		cur := make(map[string]uint64)
		for enaStat := range enaMetricNames {
			if val, ok := stats[enaStat]; ok {
				cur[enaStat] = val
			}
		}
		if len(cur) > 0 {
			curStats[iface.Name] = cur
		}

		// First scrape seeds baseline — no deltas to emit yet.
		prev, hasPrev := s.prevStats[iface.Name]
		if !hasPrev {
			continue
		}

		for enaStat := range enaMetricNames {
			curVal, okCur := cur[enaStat]
			prevVal, okPrev := prev[enaStat]
			if !okCur || !okPrev {
				continue
			}
			if curVal < prevVal {
				continue // counter reset — drop this interface's contribution
			}
			aggDeltas[enaStat] += curVal - prevVal
		}
	}

	s.prevStats = curStats

	if len(aggDeltas) == 0 {
		return nil
	}

	rm := metrics.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	for enaStat, metricName := range enaMetricNames {
		if delta, ok := aggDeltas[enaStat]; ok {
			addGaugeDP(sm.Metrics().AppendEmpty(), metricName, "None", float64(delta), now)
		}
	}
	return nil
}

// skipInterface returns true for loopback and veth* interfaces.
func skipInterface(iface net.Interface) bool {
	if iface.Flags&net.FlagLoopback != 0 {
		return true
	}
	return strings.HasPrefix(iface.Name, "veth")
}
