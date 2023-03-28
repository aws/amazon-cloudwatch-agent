// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux
// +build linux

package adapter

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/agent"
	"github.com/influxdata/telegraf/config"
	_ "github.com/influxdata/telegraf/plugins/inputs/disk"
	_ "github.com/influxdata/telegraf/plugins/inputs/diskio"
	_ "github.com/influxdata/telegraf/plugins/inputs/mem"
	_ "github.com/influxdata/telegraf/plugins/inputs/net"
	_ "github.com/influxdata/telegraf/plugins/inputs/processes"
	_ "github.com/influxdata/telegraf/plugins/inputs/procstat"
	_ "github.com/influxdata/telegraf/plugins/inputs/socket_listener"
	_ "github.com/influxdata/telegraf/plugins/inputs/swap"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	_ "github.com/aws/private-amazon-cloudwatch-agent-staging/plugins/inputs/statsd"
)

const testCfg = "./testdata/all_plugins.toml"

// Service Input differs from a regular plugin in that it operates a background service while Telegraf/CWAgent is running
// https://github.com/influxdata/telegraf/blob/d67f75e55765d364ad0aabe99382656cb5b51014/docs/INPUTS.md#service-input-plugins
type regularInputConfig struct {
	scrapeCount int
}

type serviceInputConfig struct {
	protocol      string
	listeningPort string
	metricSending []byte
}

/*
sanityTestConfig struct
@plugin               Telegraf input plugins
@regularInputConfig   Telegraf Regular Input's Configuration including number of time scraping metrics
@serviceInputConfig   Telegraf Service Input's Configuration including the port, protocol, metric's format sending
*/

type sanityTestConfig struct {
	plugin               string
	regularInputConfig   regularInputConfig
	serviceInputConfig   serviceInputConfig
	expectedMetrics      [][]string
	numMetricsComparator assert.ComparisonAssertionFunc
}

func Test_CPUPlugin(t *testing.T) {
	scrapeAndValidateMetrics(t, &sanityTestConfig{
		plugin: "cpu",
		// Scrape twice so that delta is detected and usage metrics are captured
		regularInputConfig: regularInputConfig{scrapeCount: 2},
		serviceInputConfig: serviceInputConfig{},
		// https://github.com/influxdata/telegraf/blob/8c49ddccc3cb8f8fe020dc4e1f38b93a0f2ad467/plugins/inputs/cpu/cpu.go#L109-L111
		expectedMetrics: [][]string{
			// https://github.com/influxdata/telegraf/blob/8c49ddccc3cb8f8fe020dc4e1f38b93a0f2ad467/plugins/inputs/cpu/cpu.go#L72-L86
			{"time_active", "time_user", "time_system", "time_idle", "time_nice", "time_iowait", "time_irq", "time_softirq", "time_steal", "time_guest", "time_guest_nice"},
			// https://github.com/influxdata/telegraf/blob/8c49ddccc3cb8f8fe020dc4e1f38b93a0f2ad467/plugins/inputs/cpu/cpu.go#L113-L123
			{"usage_active", "usage_user", "usage_system", "usage_idle", "usage_nice", "usage_iowait", "usage_irq", "usage_softirq", "usage_steal", "usage_guest", "usage_guest_nice"},
		},
		numMetricsComparator: assert.Equal,
	})
}

func Test_MemPlugin(t *testing.T) {
	scrapeAndValidateMetrics(t, &sanityTestConfig{
		plugin:             "mem",
		regularInputConfig: regularInputConfig{scrapeCount: 1},
		serviceInputConfig: serviceInputConfig{},
		// https://github.com/influxdata/telegraf/blob/8c49ddccc3cb8f8fe020dc4e1f38b93a0f2ad467/plugins/inputs/mem/mem.go#L40-L44
		expectedMetrics:      [][]string{{"total", "available", "used", "used_percent", "available_percent"}},
		numMetricsComparator: assert.Equal,
	})
}

func Test_SwapPlugin(t *testing.T) {
	scrapeAndValidateMetrics(t, &sanityTestConfig{
		plugin:             "swap",
		regularInputConfig: regularInputConfig{scrapeCount: 1},
		serviceInputConfig: serviceInputConfig{},
		// https://github.com/influxdata/telegraf/blob/8c49ddccc3cb8f8fe020dc4e1f38b93a0f2ad467/plugins/inputs/swap/swap.go#L32-L37
		expectedMetrics:      [][]string{{"total", "free", "used", "used_percent"}},
		numMetricsComparator: assert.Equal,
	})
}

func Test_NetPlugin(t *testing.T) {
	scrapeAndValidateMetrics(t, &sanityTestConfig{
		plugin:             "net",
		regularInputConfig: regularInputConfig{scrapeCount: 1},
		serviceInputConfig: serviceInputConfig{},
		// https://github.com/influxdata/telegraf/blob/8c49ddccc3cb8f8fe020dc4e1f38b93a0f2ad467/plugins/inputs/net/net.go#L86-L93
		expectedMetrics: [][]string{{"bytes_sent", "bytes_recv", "packets_sent", "packets_recv", "err_in", "err_out", "drop_in", "drop_out"}},
		// The net plugin stands-out here because we don't specify an interface filter in our config (to be agnostic of where this test runs)
		// which means the plugin reports metrics for each network interface it picks up. Hence, we only check at least 1 metric is reported
		// (expectedNumberOfMetrics i.e. 1 <= actualResourceMetricsCount)
		numMetricsComparator: assert.LessOrEqual,
	})
}

func Test_DiskPlugin(t *testing.T) {
	scrapeAndValidateMetrics(t, &sanityTestConfig{
		plugin:             "disk",
		regularInputConfig: regularInputConfig{scrapeCount: 1},
		// https://github.com/influxdata/telegraf/blob/8c49ddccc3cb8f8fe020dc4e1f38b93a0f2ad467/plugins/inputs/disk/disk.go#L72-L78
		expectedMetrics: [][]string{{"total", "free", "used", "used_percent", "inodes_total", "inodes_free", "inodes_used"}},
		// The disk plugin shares the same reason with net, being reported by multiple devices. Therefore, we only check at least 1 metric
		// is reported
		numMetricsComparator: assert.LessOrEqual,
	})
}

func Test_ProcessesPlugin(t *testing.T) {
	scrapeAndValidateMetrics(t, &sanityTestConfig{
		plugin:             "processes",
		regularInputConfig: regularInputConfig{scrapeCount: 1},
		serviceInputConfig: serviceInputConfig{},
		// https://github.com/influxdata/telegraf/blob/8c49ddccc3cb8f8fe020dc4e1f38b93a0f2ad467/plugins/inputs/processes/processes_notwindows.go#L65-L71
		expectedMetrics: [][]string{{"blocked", "zombies", "stopped", "running", "sleeping", "total", "unknown"}},

		numMetricsComparator: assert.Equal,
	})
}

func Test_ProcStatPlugin(t *testing.T) {
	scrapeAndValidateMetrics(t, &sanityTestConfig{
		plugin:             "procstat",
		regularInputConfig: regularInputConfig{scrapeCount: 2},
		serviceInputConfig: serviceInputConfig{},
		// https://github.com/influxdata/telegraf/blob/8c49ddccc3cb8f8fe020dc4e1f38b93a0f2ad467/plugins/inputs/procstat/procstat.go#L69-L300
		expectedMetrics: [][]string{{"cpu_time_system", "cpu_time_user", "cpu_usage", "memory_data", "memory_locked", "memory_rss", "memory_stack", "memory_swap", "memory_vms"}},
		// The procstat finds the process/PID/User/etc and find corresponding process/PID/User/etc usage from management subsystem.
		// However, its only able to use pgrep or PID file to find the target
		// https://github.com/influxdata/telegraf/blob/8c49ddccc3cb8f8fe020dc4e1f38b93a0f2ad467/plugins/inputs/procstat/procstat.go#L71-L79
		// Therefore, the metrics are different based on number of processes/PID find by pgrep or PID File and not stable.
		numMetricsComparator: assert.LessOrEqual,
	})
}

func Test_NetStatPlugin(t *testing.T) {
	scrapeAndValidateMetrics(t, &sanityTestConfig{
		plugin:             "netstat",
		regularInputConfig: regularInputConfig{scrapeCount: 1},
		serviceInputConfig: serviceInputConfig{},
		// https://github.com/aws/telegraf/blob/066eb60aa48d74bf63dcd4e10b8f13db12b43c3b/plugins/inputs/net/netstat.go#L48-L63
		expectedMetrics: [][]string{{"tcp_close_wait", "tcp_closing", "tcp_fin_wait1", "tcp_fin_wait2", "tcp_last_ack", "tcp_listen", "tcp_none", "tcp_syn_recv", "tcp_time_wait", "udp_socket", "tcp_established", "tcp_syn_sent", "tcp_close"}},

		// The netstat plugin stands-out here because we don't specify an interface filter in our config (to be agnostic of where this test runs)
		// which means the plugin reports metrics for each network interface it picks up. Hence, we only check at least 1 metric is reported
		// (expectedNumberOfMetrics i.e. 1 <= actualResourceMetricsCount)
		numMetricsComparator: assert.LessOrEqual,
	})
}

func Test_DiskIOPlugin(t *testing.T) {
	scrapeAndValidateMetrics(t, &sanityTestConfig{
		plugin:             "diskio",
		regularInputConfig: regularInputConfig{scrapeCount: 1},
		serviceInputConfig: serviceInputConfig{},
		// https://github.com/influxdata/telegraf/blob/8c49ddccc3cb8f8fe020dc4e1f38b93a0f2ad467/plugins/inputs/diskio/diskio.go#L106-L118
		expectedMetrics: [][]string{{"iops_in_progress", "io_time", "reads", "read_bytes", "read_time", "writes", "write_bytes", "write_time"}},

		// The diskio plugin shares the same reason with netstat, being reported by multiple devices. Therefore, we only check at least 1 metric
		// is reported
		numMetricsComparator: assert.LessOrEqual,
	})
}

func Test_StatsdPlugin(t *testing.T) {
	scrapeAndValidateMetrics(t, &sanityTestConfig{
		plugin:             "statsd",
		regularInputConfig: regularInputConfig{},
		//StatsD Format: https://github.com/aws/private-amazon-cloudwatch-agent-staging/tree/a136be274bc35947fe7c92f9f4fd1f338de62aee/plugins/inputs/statsd#influx-statsd
		serviceInputConfig:   serviceInputConfig{protocol: "udp", listeningPort: "127.0.0.1:14224", metricSending: []byte("statsd_time_idle:42|c\n")},
		expectedMetrics:      [][]string{{"time_idle"}},
		numMetricsComparator: assert.Equal,
	})
}

func Test_SocketListenerPlugin(t *testing.T) {
	scrapeAndValidateMetrics(t, &sanityTestConfig{
		plugin:               "socket_listener",
		regularInputConfig:   regularInputConfig{},
		serviceInputConfig:   serviceInputConfig{protocol: "tcp", listeningPort: "127.0.0.1:25826", metricSending: []byte("socket_listener,foo=bar time_idle=1i 123456789\n")},
		expectedMetrics:      [][]string{{"time_idle"}},
		numMetricsComparator: assert.Equal,
	})
}

func scrapeAndValidateMetrics(t *testing.T, cfg *sanityTestConfig) {
	as := assert.New(t)
	receiver := getInitializedReceiver(as, cfg.plugin)

	ctx := context.TODO()
	err := receiver.start(ctx, nil)
	as.NoError(err)

	otelMetrics := scrapeMetrics(as, ctx, receiver, cfg)

	err = receiver.shutdown(ctx)
	as.NoError(err)

	cfg.numMetricsComparator(t, len(cfg.expectedMetrics), otelMetrics.ResourceMetrics().Len())

	var metrics pmetric.MetricSlice
	for i := 0; i < len(cfg.expectedMetrics); i++ {
		metrics = otelMetrics.ResourceMetrics().At(i).ScopeMetrics().At(0).Metrics()
		validateMetricName(as, cfg.plugin, cfg.expectedMetrics[i], metrics)
	}
}

func getInitializedReceiver(as *assert.Assertions, plugin string) *AdaptedReceiver {
	c := config.NewConfig()
	c.InputFilters = []string{plugin}
	err := c.LoadConfig(testCfg)
	as.NoError(err)

	a, _ := agent.NewAgent(c)
	as.Len(a.Config.Inputs, 1)

	err = a.Config.Inputs[0].Init()
	as.NoError(err)

	return newAdaptedReceiver(a.Config.Inputs[0], zap.NewNop())
}

func scrapeMetrics(as *assert.Assertions, ctx context.Context, receiver *AdaptedReceiver, cfg *sanityTestConfig) pmetric.Metrics {

	var err error
	var otelMetrics pmetric.Metrics

	if _, ok := receiver.input.Input.(telegraf.ServiceInput); ok {
		conn, err := net.Dial(cfg.serviceInputConfig.protocol, cfg.serviceInputConfig.listeningPort)
		as.NoError(err)
		_, err = conn.Write(cfg.serviceInputConfig.metricSending)
		as.NoError(err)
		as.NoError(conn.Close())

		for {
			otelMetrics, err = receiver.scrape(ctx)
			as.NoError(err)

			time.Sleep(1 * time.Second)
			if otelMetrics.ResourceMetrics().Len() > 0 {
				break
			}
		}
	} else {
		for i := 0; i < cfg.regularInputConfig.scrapeCount; i++ {
			if i != 0 {
				time.Sleep(1 * time.Second)
			}
			otelMetrics, err = receiver.scrape(ctx)
			as.NoError(err)
		}
	}

	return otelMetrics
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
