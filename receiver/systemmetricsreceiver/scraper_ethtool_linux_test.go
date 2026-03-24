//go:build linux

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package systemmetricsreceiver

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// fakeIfaces returns a listIfaces func that returns the given interfaces.
func fakeIfaces(ifaces ...net.Interface) func() ([]net.Interface, error) {
	return func() ([]net.Interface, error) { return ifaces, nil }
}

func fakeIfacesErr(err error) func() ([]net.Interface, error) {
	return func() ([]net.Interface, error) { return nil, err }
}

func iface(name string, flags net.Flags) net.Interface {
	return net.Interface{Name: name, Flags: flags}
}

func TestEthtoolScraperName(t *testing.T) {
	s := newEthtoolScraper(zap.NewNop(), &MockPS{})
	assert.Equal(t, "ethtool", s.Name())
}

func TestEthtoolScraperFirstScrapeSeeds(t *testing.T) {
	ps := &MockPS{EthtoolStatsData: map[string]uint64{
		"bw_in_allowance_exceeded":  42,
		"bw_out_allowance_exceeded": 7,
		"pps_allowance_exceeded":    3,
	}}
	s := newEthtoolScraper(zap.NewNop(), ps)
	s.listIfaces = fakeIfaces(iface("eth0", net.FlagUp))

	metrics := pmetric.NewMetrics()
	require.NoError(t, s.Scrape(context.Background(), metrics))

	// First scrape seeds baseline — no metrics emitted.
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())
	// But prevStats should be populated.
	assert.NotNil(t, s.prevStats)
	assert.Contains(t, s.prevStats, "eth0")
}

func TestEthtoolScraperDeltaValues(t *testing.T) {
	ps := &MockPS{}
	s := newEthtoolScraper(zap.NewNop(), ps)
	s.listIfaces = fakeIfaces(iface("eth0", net.FlagUp))

	// Seed baseline.
	ps.EthtoolStatsData = map[string]uint64{
		"bw_in_allowance_exceeded":  10,
		"bw_out_allowance_exceeded": 20,
		"pps_allowance_exceeded":    30,
	}
	require.NoError(t, s.Scrape(context.Background(), pmetric.NewMetrics()))

	// Second scrape — should emit deltas.
	ps.EthtoolStatsData = map[string]uint64{
		"bw_in_allowance_exceeded":  15,
		"bw_out_allowance_exceeded": 25,
		"pps_allowance_exceeded":    33,
	}
	metrics := pmetric.NewMetrics()
	require.NoError(t, s.Scrape(context.Background(), metrics))

	require.Equal(t, 1, metrics.ResourceMetrics().Len())
	sm := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0)
	require.Equal(t, 3, sm.Metrics().Len())

	emitted := make(map[string]float64)
	for i := 0; i < sm.Metrics().Len(); i++ {
		m := sm.Metrics().At(i)
		assert.Equal(t, "None", m.Unit())
		emitted[m.Name()] = m.Gauge().DataPoints().At(0).DoubleValue()
	}

	assert.Equal(t, 5.0, emitted["aggregate_bw_in_allowance_exceeded"])
	assert.Equal(t, 5.0, emitted["aggregate_bw_out_allowance_exceeded"])
	assert.Equal(t, 3.0, emitted["aggregate_pps_allowance_exceeded"])
}

func TestEthtoolScraperPerInterfaceDeltas(t *testing.T) {
	ps := &MockPS{}
	s := newEthtoolScraper(zap.NewNop(), ps)
	s.listIfaces = fakeIfaces(
		iface("eth0", net.FlagUp),
		iface("eth1", net.FlagUp),
	)

	// Seed baseline for both interfaces.
	ps.EthtoolStatsData = map[string]uint64{
		"bw_in_allowance_exceeded": 100,
	}
	require.NoError(t, s.Scrape(context.Background(), pmetric.NewMetrics()))

	// Second scrape — deltas summed across interfaces.
	ps.EthtoolStatsData = map[string]uint64{
		"bw_in_allowance_exceeded": 110,
	}
	metrics := pmetric.NewMetrics()
	require.NoError(t, s.Scrape(context.Background(), metrics))

	// 1 ResourceMetrics with summed delta: (110-100) + (110-100) = 20
	require.Equal(t, 1, metrics.ResourceMetrics().Len())
	sm := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0)
	assert.Equal(t, 1, sm.Metrics().Len())
	assert.Equal(t, "aggregate_bw_in_allowance_exceeded", sm.Metrics().At(0).Name())
	assert.Equal(t, 20.0, sm.Metrics().At(0).Gauge().DataPoints().At(0).DoubleValue())
}

func TestEthtoolScraperCounterResetDropsDelta(t *testing.T) {
	ps := &MockPS{}
	s := newEthtoolScraper(zap.NewNop(), ps)
	s.listIfaces = fakeIfaces(iface("eth0", net.FlagUp))

	// Seed baseline.
	ps.EthtoolStatsData = map[string]uint64{
		"bw_in_allowance_exceeded": 100,
	}
	require.NoError(t, s.Scrape(context.Background(), pmetric.NewMetrics()))

	// Counter reset — current < previous.
	ps.EthtoolStatsData = map[string]uint64{
		"bw_in_allowance_exceeded": 5,
	}
	metrics := pmetric.NewMetrics()
	require.NoError(t, s.Scrape(context.Background(), metrics))

	// Negative delta dropped — no metrics emitted.
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())
}

func TestEthtoolScraperSkipsLoopbackAndVeth(t *testing.T) {
	ps := &MockPS{}
	s := newEthtoolScraper(zap.NewNop(), ps)
	s.listIfaces = fakeIfaces(
		iface("lo", net.FlagLoopback|net.FlagUp),
		iface("veth1234", net.FlagUp),
		iface("eth0", net.FlagUp),
	)

	// Seed.
	ps.EthtoolStatsData = map[string]uint64{"bw_in_allowance_exceeded": 10}
	require.NoError(t, s.Scrape(context.Background(), pmetric.NewMetrics()))

	// Second scrape.
	ps.EthtoolStatsData = map[string]uint64{"bw_in_allowance_exceeded": 20}
	metrics := pmetric.NewMetrics()
	require.NoError(t, s.Scrape(context.Background(), metrics))

	// Only eth0 should produce metrics.
	require.Equal(t, 1, metrics.ResourceMetrics().Len())
}

func TestEthtoolScraperFiltersNonAllowanceStats(t *testing.T) {
	ps := &MockPS{}
	s := newEthtoolScraper(zap.NewNop(), ps)
	s.listIfaces = fakeIfaces(iface("eth0", net.FlagUp))

	// Seed with non-allowance stats only.
	ps.EthtoolStatsData = map[string]uint64{"tx_bytes": 999999, "rx_packets": 200}
	require.NoError(t, s.Scrape(context.Background(), pmetric.NewMetrics()))

	// Second scrape — still no allowance stats.
	metrics := pmetric.NewMetrics()
	require.NoError(t, s.Scrape(context.Background(), metrics))
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())
}

func TestEthtoolScraperErrorSkips(t *testing.T) {
	ps := &MockPS{EthtoolStatsErr: errors.New("no ENA driver")}
	s := newEthtoolScraper(zap.NewNop(), ps)
	s.listIfaces = fakeIfaces(iface("eth0", net.FlagUp))

	metrics := pmetric.NewMetrics()
	require.NoError(t, s.Scrape(context.Background(), metrics))
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())
}

func TestEthtoolScraperListIfacesError(t *testing.T) {
	s := newEthtoolScraper(zap.NewNop(), &MockPS{})
	s.listIfaces = fakeIfacesErr(errors.New("permission denied"))

	metrics := pmetric.NewMetrics()
	require.NoError(t, s.Scrape(context.Background(), metrics))
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())
}

func TestEthtoolScraperNoInterfaces(t *testing.T) {
	s := newEthtoolScraper(zap.NewNop(), &MockPS{})
	s.listIfaces = fakeIfaces()

	metrics := pmetric.NewMetrics()
	require.NoError(t, s.Scrape(context.Background(), metrics))
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())
}

func TestSkipInterface(t *testing.T) {
	assert.True(t, skipInterface(iface("lo", net.FlagLoopback|net.FlagUp)))
	assert.True(t, skipInterface(iface("veth1234", net.FlagUp)))
	assert.True(t, skipInterface(iface("vethABC", net.FlagUp)))
	assert.False(t, skipInterface(iface("eth0", net.FlagUp)))
	assert.False(t, skipInterface(iface("eth1", net.FlagUp)))
	assert.False(t, skipInterface(iface("ens5", net.FlagUp)))
	assert.False(t, skipInterface(iface("docker0", net.FlagUp)))
}
