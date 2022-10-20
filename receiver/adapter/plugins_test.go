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
	_ "github.com/influxdata/telegraf/plugins/inputs/disk"
	_ "github.com/influxdata/telegraf/plugins/inputs/mem"
	_ "github.com/influxdata/telegraf/plugins/inputs/net"
	_ "github.com/influxdata/telegraf/plugins/inputs/processes"
	_ "github.com/influxdata/telegraf/plugins/inputs/swap"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap/zaptest"
)

var testCfg = "./testdata/all_plugins.toml"

type SanityTestConfig struct {
	plugin                               string
	scrapeCount                          int
	expectedMetrics                      [][]string
	expectedResourceMetricsLen           int
	expectedResourceMetricsLenComparator assert.ComparisonAssertionFunc
}

func Test_CPUPlugin(t *testing.T) {
	scrapeAndValidateMetrics(t, &SanityTestConfig{
		plugin: "cpu",
		// Scrape twice so that delta is detected and usage metrics are captured
		scrapeCount: 2,
		// https://github.com/influxdata/telegraf/blob/8c49ddccc3cb8f8fe020dc4e1f38b93a0f2ad467/plugins/inputs/cpu/cpu.go#L109-L111
		expectedMetrics: [][]string{
			// https://github.com/influxdata/telegraf/blob/8c49ddccc3cb8f8fe020dc4e1f38b93a0f2ad467/plugins/inputs/cpu/cpu.go#L72-L86
			{"time_active", "time_user", "time_system", "time_idle", "time_nice", "time_iowait", "time_irq", "time_softirq", "time_steal", "time_guest", "time_guest_nice"},
			// https://github.com/influxdata/telegraf/blob/8c49ddccc3cb8f8fe020dc4e1f38b93a0f2ad467/plugins/inputs/cpu/cpu.go#L113-L123
			{"usage_active", "usage_user", "usage_system", "usage_idle", "usage_nice", "usage_iowait", "usage_irq", "usage_softirq", "usage_steal", "usage_guest", "usage_guest_nice"},
		},
		expectedResourceMetricsLen:           2,
		expectedResourceMetricsLenComparator: assert.Equal,
	})
}

func Test_MemPlugin(t *testing.T) {
	scrapeAndValidateMetrics(t, &SanityTestConfig{
		plugin:      "mem",
		scrapeCount: 1,
		// https://github.com/influxdata/telegraf/blob/8c49ddccc3cb8f8fe020dc4e1f38b93a0f2ad467/plugins/inputs/mem/mem.go#L40-L44
		expectedMetrics:                      [][]string{{"total", "available", "used", "used_percent", "available_percent"}},
		expectedResourceMetricsLen:           1,
		expectedResourceMetricsLenComparator: assert.Equal,
	})
}

func Test_SwapPlugin(t *testing.T) {
	scrapeAndValidateMetrics(t, &SanityTestConfig{
		plugin:      "swap",
		scrapeCount: 1,
		// https://github.com/influxdata/telegraf/blob/8c49ddccc3cb8f8fe020dc4e1f38b93a0f2ad467/plugins/inputs/swap/swap.go#L32-L37
		expectedMetrics:                      [][]string{{"total", "free", "used", "used_percent"}},
		expectedResourceMetricsLen:           1,
		expectedResourceMetricsLenComparator: assert.Equal,
	})
}

func Test_NetPlugin(t *testing.T) {
	scrapeAndValidateMetrics(t, &SanityTestConfig{
		plugin:      "net",
		scrapeCount: 1,
		// https://github.com/influxdata/telegraf/blob/8c49ddccc3cb8f8fe020dc4e1f38b93a0f2ad467/plugins/inputs/net/net.go#L86-L93
		expectedMetrics:            [][]string{{"bytes_sent", "bytes_recv", "packets_sent", "packets_recv", "err_in", "err_out", "drop_in", "drop_out"}},
		expectedResourceMetricsLen: 1,
		// The net plugin stands-out here because we don't specify an interface filter in our config (to be agnostic of where this test runs)
		// which means the plugin reports metrics for each network interface it picks up. Hence, we only check atleast 1 metric is reported
		// (expectedResourceMetricsLen i.e. 1 <= actualResourceMetricsCount)
		expectedResourceMetricsLenComparator: assert.LessOrEqual,
	})
}

func Test_DiskPlugin(t *testing.T) {
	scrapeAndValidateMetrics(t, &SanityTestConfig{
		plugin:      "disk",
		scrapeCount: 1,
		// https://github.com/influxdata/telegraf/blob/8c49ddccc3cb8f8fe020dc4e1f38b93a0f2ad467/plugins/inputs/disk/disk.go#L72-L78
		expectedMetrics:                      [][]string{{"total", "free", "used", "used_percent", "inodes_total", "inodes_free", "inodes_used"}},
		expectedResourceMetricsLen:           1,
		expectedResourceMetricsLenComparator: assert.Equal,
	})
}

func Test_ProcessesPlugin(t *testing.T) {
	scrapeAndValidateMetrics(t, &SanityTestConfig{
		plugin:      "processes",
		scrapeCount: 1,
		// https://github.com/influxdata/telegraf/blob/8c49ddccc3cb8f8fe020dc4e1f38b93a0f2ad467/plugins/inputs/processes/processes_notwindows.go#L65-L71
		expectedMetrics:                      [][]string{{"blocked", "zombies", "stopped", "running", "sleeping", "total", "unknown"}},
		expectedResourceMetricsLen:           1,
		expectedResourceMetricsLenComparator: assert.Equal,
	})
}

func scrapeAndValidateMetrics(t *testing.T, cfg *SanityTestConfig) {
	as := assert.New(t)
	receiver := getInitializedReceiver(t, cfg.plugin)

	err := receiver.start(nil, nil)
	as.NoError(err)

	var otelMetrics pmetric.Metrics
	for i := 0; i < cfg.scrapeCount; i++ {
		if i != 0 {
			time.Sleep(1 * time.Second)
		}
		otelMetrics, err = receiver.scrape(nil)
		as.NoError(err)
	}

	err = receiver.shutdown(nil)
	as.NoError(err)

	cfg.expectedResourceMetricsLenComparator(t, cfg.expectedResourceMetricsLen, otelMetrics.ResourceMetrics().Len())

	var metrics pmetric.MetricSlice
	for i := 0; i < len(cfg.expectedMetrics); i++ {
		metrics = otelMetrics.ResourceMetrics().At(i).ScopeMetrics().At(0).Metrics()
		validateMetricName(as, cfg.plugin, cfg.expectedMetrics[i], metrics)
	}
}

func getInitializedReceiver(t *testing.T, plugin string) *AdaptedReceiver {
	as := assert.New(t)
	c := config.NewConfig()
	c.InputFilters = []string{plugin}
	err := c.LoadConfig(testCfg)
	as.NoError(err)

	a, _ := agent.NewAgent(c)
	as.Len(a.Config.Inputs, 1)

	err = a.Config.Inputs[0].Init()
	as.NoError(err)

	return newAdaptedReceiver(a.Config.Inputs[0], zaptest.NewLogger(t))
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
