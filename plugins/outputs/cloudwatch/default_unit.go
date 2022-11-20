// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatch

import "github.com/aws/aws-sdk-go/service/cloudwatch"

type pluginsSupportedMetricDefaultUnit map[string]struct {
	supportedMetrics   []string
	defaultMetricsUnit []string
}

// CloudWatch supports:
// https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_MetricDatum.html
var standardUnitValues = cloudwatch.StandardUnit_Values()

// Supported Metrics https://github.com/aws/amazon-cloudwatch-agent/blob/6451e8b913bcf9892f2cead08e335c913c690e6d/translator/translate/metrics/config/registered_metrics.go
var metricDefaultUnit = pluginsSupportedMetricDefaultUnit{
	"cpu": {
		supportedMetrics: []string{"usage_active", "usage_idle", "usage_nice", "usage_guest", "usage_guest_nice",
			"usage_iowait", "usage_irq", "usage_softirq", "usage_steal", "usage_system", "usage_user"},
		defaultMetricsUnit: []string{cloudwatch.StandardUnitPercent, cloudwatch.StandardUnitPercent, cloudwatch.StandardUnitPercent, cloudwatch.StandardUnitPercent, cloudwatch.StandardUnitPercent,
			cloudwatch.StandardUnitPercent, cloudwatch.StandardUnitPercent, cloudwatch.StandardUnitPercent, cloudwatch.StandardUnitPercent, cloudwatch.StandardUnitPercent, cloudwatch.StandardUnitPercent},
	},

	"disk": {
		supportedMetrics: []string{"free", "total", "used", "inodes_free", "inodes_total",
			"inodes_used", "used_percent"},
		defaultMetricsUnit: []string{cloudwatch.StandardUnitBytes, cloudwatch.StandardUnitBytes, cloudwatch.StandardUnitBytes, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount,
			cloudwatch.StandardUnitCount, cloudwatch.StandardUnitPercent},
	},

	"diskio": {
		supportedMetrics: []string{"iops_in_progress", "io_time", "reads", "writes", "read_bytes",
			"write_bytes", "read_time", "write_time"},
		defaultMetricsUnit: []string{cloudwatch.StandardUnitCount, cloudwatch.StandardUnitMilliseconds, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitBytes,
			cloudwatch.StandardUnitBytes, cloudwatch.StandardUnitMilliseconds, cloudwatch.StandardUnitMilliseconds},
	},

	"swap": {
		supportedMetrics:   []string{"used", "total", "used_percent", "free"},
		defaultMetricsUnit: []string{cloudwatch.StandardUnitBytes, cloudwatch.StandardUnitBytes, cloudwatch.StandardUnitPercent, cloudwatch.StandardUnitBytes},
	},

	"mem": {
		supportedMetrics: []string{"used", "cached", "total", "available", "free",
			"buffered", "active", "inactive", "available_percent", "used_percent"},
		defaultMetricsUnit: []string{cloudwatch.StandardUnitBytes, cloudwatch.StandardUnitBytes, cloudwatch.StandardUnitBytes, cloudwatch.StandardUnitBytes, cloudwatch.StandardUnitBytes,
			cloudwatch.StandardUnitBytes, cloudwatch.StandardUnitBytes, cloudwatch.StandardUnitBytes, cloudwatch.StandardUnitPercent, cloudwatch.StandardUnitPercent},
	},

	"net": {
		supportedMetrics: []string{"bytes_sent", "bytes_recv", "drop_in", "drop_out", "err_in",
			"err_out", "packets_sent", "packets_recv"},
		defaultMetricsUnit: []string{cloudwatch.StandardUnitBytes, cloudwatch.StandardUnitBytes, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount,
			cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount},
	},

	"netstat": {
		supportedMetrics: []string{"tcp_established", "tcp_syn_sent", "tcp_syn_recv", "tcp_close", "tcp_close_wait",
			"tcp_closing", "tcp_fin_wait1", "tcp_fin_wait2", "tcp_last_ack", "tcp_listen", "tcp_none",
			"tcp_time_wait", "udp_socket"},
		defaultMetricsUnit: []string{cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount,
			cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount,
			cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount},
	},

	"processes": {
		supportedMetrics: []string{"blocked", "idle", "paging", "stopped", "total",
			"total_threads", "wait", "zombie", "running", "sleeping", "dead"},
		defaultMetricsUnit: []string{cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount,
			cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount},
	},
	"procstat_lookup": {
		supportedMetrics:   []string{"pid_count"},
		defaultMetricsUnit: []string{cloudwatch.StandardUnitCount},
	},
	"procstat": {
		supportedMetrics: []string{"read_bytes", "read_count", "write_bytes", "write_count", "memory_data",
			"memory_locked", "memory_rss", "memory_stack", "memory_swap", "memory_vms", "cpu_usage",
			"cpu_time", "cpu_time_user", "cpu_time_guest", "cpu_time_guest_nice", "cpu_time_idle", "cpu_time_iowait",
			"cpu_time_irq", "cpu_time_soft_irq", "cpu_time_nice", "cpu_time_steal", "cpu_time_stolen", "rlimit_cpu_time_hard",
			"rlimit_cpu_time_soft", "rlimit_file_locks_hard", "rlimit_file_locks_soft", "rlimit_memory_data_hard", "rlimit_memory_data_soft", "rlimit_memory_locked_hard",
			"rlimit_memory_locked_soft", "rlimit_memory_rss_hard", "rlimit_memory_rss_soft", "rlimit_memory_stack_hard", "rlimit_memory_stack_soft", "rlimit_memory_vms_hard",
			"rlimit_memory_vms_soft", "pid", "nice_priority", "realtime_priority", "signals_pending", "voluntary_context_switches",
			"involuntary_context_switches"},
		defaultMetricsUnit: []string{cloudwatch.StandardUnitBytes, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitBytes, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitBytes,
			cloudwatch.StandardUnitBytes, cloudwatch.StandardUnitBytes, cloudwatch.StandardUnitBytes, cloudwatch.StandardUnitBytes, cloudwatch.StandardUnitBytes, cloudwatch.StandardUnitPercent,
			cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount,
			cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount,
			cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitBytes, cloudwatch.StandardUnitBytes, cloudwatch.StandardUnitBytes,
			cloudwatch.StandardUnitBytes, cloudwatch.StandardUnitBytes, cloudwatch.StandardUnitBytes, cloudwatch.StandardUnitBytes, cloudwatch.StandardUnitBytes, cloudwatch.StandardUnitBytes,
			cloudwatch.StandardUnitBytes, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount, cloudwatch.StandardUnitCount,
			cloudwatch.StandardUnitCount},
	},
}

// isUnitInvalid checks whether the given unit is supported with CloudWatchBackend or not
// https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_MetricDatum.html
func isUnitInvalid(unit string) bool {
	if unit == "" {
		return false
	}
	for _, v := range standardUnitValues {
		if v == unit {
			return false
		}
	}
	return true
}
