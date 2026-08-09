// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package selftelemetry

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	otelmetric "go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/zap"
)

// newTestBridge wires the extension to a real prometheus-backed MeterProvider, so the assertions run
// against what a scraper would actually see rather than against the instruments.
func newTestBridge(t *testing.T, sources func() []source) (*selfTelemetry, *prometheus.Registry) {
	t.Helper()
	out := prometheus.NewRegistry()
	exporter, err := otelprom.New(
		otelprom.WithRegisterer(out),
		otelprom.WithoutCounterSuffixes(),
		otelprom.WithoutUnits(),
		otelprom.WithoutScopeInfo(),
		otelprom.WithoutTargetInfo(),
	)
	require.NoError(t, err)
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter))
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(t.Context())) })

	return &selfTelemetry{
		logger:      zap.NewNop(),
		cfg:         &Config{},
		meter:       provider.Meter(scopeName),
		sources:     sources,
		instruments: map[string]otelmetric.Float64Observable{},
	}, out
}

// served flattens the exposition into name -> label set for readable assertions.
func served(t *testing.T, reg *prometheus.Registry) map[string][]map[string]string {
	t.Helper()
	families, err := reg.Gather()
	require.NoError(t, err)
	result := map[string][]map[string]string{}
	for _, mf := range families {
		for _, m := range mf.GetMetric() {
			labels := map[string]string{}
			for _, l := range m.GetLabel() {
				labels[l.GetName()] = l.GetValue()
			}
			result[mf.GetName()] = append(result[mf.GetName()], labels)
		}
	}
	return result
}

func receiverRegistry(t *testing.T, receiverID string) *prometheus.Registry {
	t.Helper()
	reg := prometheus.NewRegistry()
	wrapped := prometheus.WrapRegistererWith(prometheus.Labels{receiverLabel: receiverID}, reg)
	targets := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "prometheus_target_scrape_pool_targets",
		Help: "Current number of targets in this scrape pool.",
	}, []string{"scrape_job"})
	wrapped.MustRegister(targets)
	targets.WithLabelValues("ebs_csi_node").Set(3)
	targets.WithLabelValues("dcgm").Set(0)
	return reg
}

// TestPublishesReceiverAndTelegrafRegistries covers the reason the bridge exists: both runtimes'
// registries have to land on the one endpoint, each series naming the receiver it came from.
func TestPublishesReceiverAndTelegrafRegistries(t *testing.T) {
	telegraf := prometheus.NewRegistry()
	telegrafTargets := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "prometheus_sd_discovered_targets",
		Help: "Current number of discovered targets.",
	}, []string{"config"})
	telegraf.MustRegister(telegrafTargets)
	telegrafTargets.WithLabelValues("kubernetes-pods").Set(12)

	bridge, out := newTestBridge(t, func() []source {
		return []source{
			{gatherer: receiverRegistry(t, "prometheus/alpha")},
			{gatherer: telegraf, fallback: telegrafReceiver},
		}
	})
	bridge.sync()

	got := served(t, out)
	require.Len(t, got["prometheus_target_scrape_pool_targets"], 2)
	for _, labels := range got["prometheus_target_scrape_pool_targets"] {
		assert.Equal(t, "prometheus/alpha", labels[receiverLabel])
	}
	require.Len(t, got["prometheus_sd_discovered_targets"], 1)
	assert.Equal(t, telegrafReceiver, got["prometheus_sd_discovered_targets"][0][receiverLabel],
		"telegraf series must be labelled so the endpoint says where each series came from")
}

// TestZeroTargetPoolIsPublished is the signal the whole feature is for: a pool that discovered
// nothing has to be visible, since that failure is otherwise silent.
func TestZeroTargetPoolIsPublished(t *testing.T) {
	bridge, out := newTestBridge(t, func() []source {
		return []source{{gatherer: receiverRegistry(t, "prometheus/alpha")}}
	})
	bridge.sync()

	var zeroTargetJobs []string
	families, err := out.Gather()
	require.NoError(t, err)
	for _, mf := range families {
		if mf.GetName() != "prometheus_target_scrape_pool_targets" {
			continue
		}
		for _, m := range mf.GetMetric() {
			if m.GetGauge().GetValue() != 0 {
				continue
			}
			for _, l := range m.GetLabel() {
				if l.GetName() == "scrape_job" {
					zeroTargetJobs = append(zeroTargetJobs, l.GetValue())
				}
			}
		}
	}
	assert.Equal(t, []string{"dcgm"}, zeroTargetJobs)
}

// TestSkipsUnrelatedAndUnsupportedFamilies keeps the endpoint to the scrape and discovery signals,
// and proves a summary does not fail the whole pass.
func TestSkipsUnrelatedAndUnsupportedFamilies(t *testing.T) {
	reg := prometheus.NewRegistry()
	unrelated := prometheus.NewGauge(prometheus.GaugeOpts{Name: "go_goroutines", Help: "unrelated"})
	summary := prometheus.NewSummary(prometheus.SummaryOpts{
		Name: "prometheus_target_interval_length_seconds",
		Help: "Actual intervals between scrapes.",
	})
	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "prometheus_target_scrapes_sample_out_of_order_total",
		Help: "Total number of samples rejected.",
	})
	reg.MustRegister(unrelated, summary, counter)
	unrelated.Set(42)
	summary.Observe(15.2)
	counter.Add(7)

	bridge, out := newTestBridge(t, func() []source {
		return []source{{gatherer: reg, fallback: telegrafReceiver}}
	})
	bridge.sync()

	got := served(t, out)
	assert.NotContains(t, got, "go_goroutines", "only the scrape and discovery families belong here")
	assert.NotContains(t, got, "prometheus_target_interval_length_seconds", "summaries have no observable form")
	assert.Contains(t, got, "prometheus_target_scrapes_sample_out_of_order_total")
}

// TestSyncPicksUpLateFamilies matters because extensions start before receivers, so the first pass
// sees an empty registry and later passes have to add what appeared since.
func TestSyncPicksUpLateFamilies(t *testing.T) {
	reg := prometheus.NewRegistry()
	bridge, out := newTestBridge(t, func() []source {
		return []source{{gatherer: reg, fallback: telegrafReceiver}}
	})

	bridge.sync()
	assert.Empty(t, served(t, out), "nothing is registered yet")

	targets := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "prometheus_target_scrape_pool_targets",
		Help: "Current number of targets in this scrape pool.",
	}, []string{"scrape_job"})
	reg.MustRegister(targets)
	targets.WithLabelValues("apiserver").Set(1)

	bridge.sync()
	assert.Contains(t, served(t, out), "prometheus_target_scrape_pool_targets")
}
