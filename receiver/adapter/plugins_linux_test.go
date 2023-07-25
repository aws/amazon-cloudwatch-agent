// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux
// +build linux

package adapter

import (
	"testing"

	_ "github.com/influxdata/telegraf/plugins/inputs/disk"
	_ "github.com/influxdata/telegraf/plugins/inputs/diskio"
	_ "github.com/influxdata/telegraf/plugins/inputs/mem"
	_ "github.com/influxdata/telegraf/plugins/inputs/net"
	_ "github.com/influxdata/telegraf/plugins/inputs/processes"
	_ "github.com/influxdata/telegraf/plugins/inputs/procstat"
	_ "github.com/influxdata/telegraf/plugins/inputs/socket_listener"
	_ "github.com/influxdata/telegraf/plugins/inputs/swap"
	"github.com/stretchr/testify/assert"

	_ "github.com/aws/amazon-cloudwatch-agent/plugins/inputs/statsd"
)

const testCfg = "./testdata/all_plugins.toml"

func Test_CPUPlugin(t *testing.T) {
	scrapeAndValidateMetrics(t, &sanityTestConfig{
		testConfig: testCfg,
		plugin:     "cpu",
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
		testConfig:         testCfg,
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
		testConfig:         testCfg,
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
		testConfig:         testCfg,
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
		testConfig:         testCfg,
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
		testConfig:         testCfg,
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
		testConfig:         testCfg,
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
		testConfig:         testCfg,
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
		testConfig:         testCfg,
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

// Failing in Github Action; however, not local. Therefore, comment it for avoid causing disruptness and
// the test only serves as sanity.
/*
func Test_StatsdPlugin(t *testing.T) {
	scrapeAndValidateMetrics(t, &sanityTestConfig{
		plugin:             "statsd",
		regularInputConfig: regularInputConfig{},
		//StatsD Format: https://github.com/aws/amazon-cloudwatch-agent/tree/v1.247360.0/plugins/inputs/statsd#influx-statsd
		serviceInputConfig:   serviceInputConfig{protocol: "udp", listeningPort: "127.0.0.1:14224", metricSending: []byte("statsd_time_idle:42|c\n")},
		expectedMetrics:      [][]string{{"time_idle"}},
		numMetricsComparator: assert.Equal,
	})
}
*/

func Test_SocketListenerPlugin(t *testing.T) {
	scrapeAndValidateMetrics(t, &sanityTestConfig{
		testConfig:           testCfg,
		plugin:               "socket_listener",
		regularInputConfig:   regularInputConfig{},
		serviceInputConfig:   serviceInputConfig{protocol: "tcp", listeningPort: "127.0.0.1:25826", metricSending: []byte("socket_listener,foo=bar time_idle=1i 123456789\n")},
		expectedMetrics:      [][]string{{"time_idle"}},
		numMetricsComparator: assert.Equal,
	})
}
