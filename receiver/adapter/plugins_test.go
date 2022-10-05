// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build !windows
// +build !windows

package adapter

import (
	"fmt"
	"testing"
	"time"

	"github.com/influxdata/telegraf/agent"
	"github.com/influxdata/telegraf/config"
	"github.com/influxdata/telegraf/plugins/inputs/disk"
	"github.com/influxdata/telegraf/plugins/inputs/mem"
	"github.com/influxdata/telegraf/plugins/inputs/net"
	_ "github.com/influxdata/telegraf/plugins/inputs/swap"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap/zaptest"
)

var testCfg = "./testdata/all_plugins.toml"

func Test_CPUPlugin(t *testing.T) {
	t.Helper()
	as := assert.New(t)
	cpu := "cpu"

	c := config.NewConfig()
	c.InputFilters = []string{cpu}
	err := c.LoadConfig(testCfg)
	as.NoError(err)

	a, _ := agent.NewAgent(c)
	as.Len(a.Config.Inputs, 1)

	receiver := newAdaptedReceiver(a.Config.Inputs[0], zaptest.NewLogger(t))
	err = receiver.start(nil, nil)
	as.NoError(err)

	// Scrape twice but with a slight delay so that delta is detected and usage metrics are captured
	// https://github.com/influxdata/telegraf/blob/8c49ddccc3cb8f8fe020dc4e1f38b93a0f2ad467/plugins/inputs/cpu/cpu.go#L109-L111
	otelMetrics, err := receiver.scrape(nil)
	as.NoError(err)
	time.Sleep(1 * time.Second)
	otelMetrics, err = receiver.scrape(nil)
	as.NoError(err)

	err = receiver.shutdown(nil)
	as.NoError(err)

	as.Equal(2, otelMetrics.ResourceMetrics().Len())

	// Validate CPU Time metrics
	// https://github.com/influxdata/telegraf/blob/8c49ddccc3cb8f8fe020dc4e1f38b93a0f2ad467/plugins/inputs/cpu/cpu.go#L72-L86
	metrics := otelMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	as.Equal(11, metrics.Len())
	expectedCPUTimeMetrics := []string{"time_active", "time_user", "time_system", "time_idle", "time_nice", "time_iowait", "time_irq", "time_softirq", "time_steal", "time_guest", "time_guest_nice"}
	validateMetricName(as, cpu, expectedCPUTimeMetrics, metrics)

	// Validate CPU Usage metrics
	// https://github.com/influxdata/telegraf/blob/8c49ddccc3cb8f8fe020dc4e1f38b93a0f2ad467/plugins/inputs/cpu/cpu.go#L113-L123
	metrics = otelMetrics.ResourceMetrics().At(1).ScopeMetrics().At(0).Metrics()
	as.Equal(11, metrics.Len())
	expectedCPUUsageMetrics := []string{"usage_active", "usage_user", "usage_system", "usage_idle", "usage_nice", "usage_iowait", "usage_irq", "usage_softirq", "usage_steal", "usage_guest", "usage_guest_nice"}
	validateMetricName(as, cpu, expectedCPUUsageMetrics, metrics)
}

func Test_MemPlugin(t *testing.T) {
	t.Helper()
	as := assert.New(t)
	memory := "mem"

	memStats := mem.MemStats{}
	err := memStats.Init()
	as.NoError(err)

	c := config.NewConfig()
	c.InputFilters = []string{memory}

	err = c.LoadConfig(testCfg)
	as.NoError(err)

	a, _ := agent.NewAgent(c)
	as.Len(a.Config.Inputs, 1)

	receiver := newAdaptedReceiver(a.Config.Inputs[0], zaptest.NewLogger(t))
	err = receiver.start(nil, nil)
	as.NoError(err)

	otelMetrics, err := receiver.scrape(nil)
	as.NoError(err)

	err = receiver.shutdown(nil)
	as.NoError(err)

	// Validate Mem metrics
	// https://github.com/influxdata/telegraf/blob/8c49ddccc3cb8f8fe020dc4e1f38b93a0f2ad467/plugins/inputs/mem/mem.go#L40-L44
	metrics := otelMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	as.Equal(5, metrics.Len())
	expectedMemoryUsageMetrics := []string{"total", "available", "used", "used_percent", "available_percent"}
	validateMetricName(as, memory, expectedMemoryUsageMetrics, metrics)
}

func Test_SwapPlugin(t *testing.T) {
	t.Helper()
	as := assert.New(t)
	swaps := "swap"

	memStats := mem.MemStats{}
	err := memStats.Init()
	as.NoError(err)

	c := config.NewConfig()
	c.InputFilters = []string{swaps}

	err = c.LoadConfig(testCfg)
	as.NoError(err)

	a, _ := agent.NewAgent(c)
	as.Len(a.Config.Inputs, 1)

	receiver := newAdaptedReceiver(a.Config.Inputs[0], zaptest.NewLogger(t))
	err = receiver.start(nil, nil)
	as.NoError(err)

	otelMetrics, err := receiver.scrape(nil)
	as.NoError(err)

	err = receiver.shutdown(nil)
	as.NoError(err)

	// Validate Swap metrics
	// https://github.com/influxdata/telegraf/blob/8c49ddccc3cb8f8fe020dc4e1f38b93a0f2ad467/plugins/inputs/swap/swap.go#L32-L37
	metrics := otelMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	as.Equal(4, metrics.Len())
	expectedSwapMetrics := []string{"total", "free", "used", "used_percent"}
	validateMetricName(as, swaps, expectedSwapMetrics, metrics)
}

func Test_NetPlugin(t *testing.T) {
	t.Helper()
	as := assert.New(t)
	network := "net"

	netStats := net.NetIOStats{}
	netStats.IgnoreProtocolStats = true

	c := config.NewConfig()
	c.InputFilters = []string{network}

	err := c.LoadConfig(testCfg)
	as.NoError(err)

	a, _ := agent.NewAgent(c)
	as.Len(a.Config.Inputs, 1)

	receiver := newAdaptedReceiver(a.Config.Inputs[0], zaptest.NewLogger(t))
	err = receiver.start(nil, nil)
	as.NoError(err)

	otelMetrics, err := receiver.scrape(nil)
	as.NoError(err)

	err = receiver.shutdown(nil)
	as.NoError(err)

	// Validate Net metrics
	// https://github.com/influxdata/telegraf/blob/8c49ddccc3cb8f8fe020dc4e1f38b93a0f2ad467/plugins/inputs/net/net.go#L86-L93
	metrics := otelMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	as.Equal(8, metrics.Len())
	expectedNetMetrics := []string{"bytes_sent", "bytes_recv", "packets_sent", "packets_recv", "err_in", "err_out", "drop_in", "drop_out"}
	validateMetricName(as, network, expectedNetMetrics, metrics)
}

func Test_DiskPlugin(t *testing.T) {
	t.Helper()
	as := assert.New(t)
	diskP := "disk"

	diskStats := disk.DiskStats{}
	err := diskStats.Init()
	as.NoError(err)

	c := config.NewConfig()
	c.InputFilters = []string{diskP}

	err = c.LoadConfig(testCfg)
	as.NoError(err)

	a, _ := agent.NewAgent(c)
	as.Len(a.Config.Inputs, 1)

	err = a.Config.Inputs[0].Init()
	as.NoError(err)

	receiver := newAdaptedReceiver(a.Config.Inputs[0], zaptest.NewLogger(t))
	err = receiver.start(nil, nil)
	as.NoError(err)

	otelMetrics, err := receiver.scrape(nil)
	as.NoError(err)

	err = receiver.shutdown(nil)
	as.NoError(err)

	// Validate Disk metrics
	// https://github.com/influxdata/telegraf/blob/8c49ddccc3cb8f8fe020dc4e1f38b93a0f2ad467/plugins/inputs/disk/disk.go#L72-L78
	metrics := otelMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	as.Equal(7, metrics.Len())
	expectedDiskMetrics := []string{"total", "free", "used", "used_percent", "inodes_total", "inodes_free", "inodes_used"}
	validateMetricName(as, diskP, expectedDiskMetrics, metrics)
}

func validateMetricName(as *assert.Assertions, plugin string, expectedResourceMetricsName []string, actualOtelSlMetrics pmetric.MetricSlice) {
	as.Equal(len(expectedResourceMetricsName), actualOtelSlMetrics.Len(), "Number of metrics did not match!")

	matchMetrics := actualOtelSlMetrics.Len()
	for _, expectedMetric := range expectedResourceMetricsName {
		for metricIndex := 0; metricIndex < actualOtelSlMetrics.Len(); metricIndex++ {
			metric := actualOtelSlMetrics.At(metricIndex)
			// Check name to decrease the match metrics since metric name is the only unique attribute
			// And ignore the rest checking
			if fmt.Sprintf("%s_%s", plugin, expectedMetric) != metric.Name() {
				continue
			}
			matchMetrics--
		}
	}

	as.Equal(0, matchMetrics, "Metrics did not match!")
}
