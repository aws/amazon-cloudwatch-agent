// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatch

import "github.com/aws/aws-sdk-go/service/cloudwatch"

type MetricUnit map[string]string

// CloudWatch supports:
// https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_MetricDatum.html
var standardUnitValues = cloudwatch.StandardUnit_Values()

var metricDefaultUnit = MetricUnit{
	"procstat_read_bytes":                   cloudwatch.StandardUnitBytes,
	"procstat_read_count":                   cloudwatch.StandardUnitCount,
	"procstat_write_bytes":                  cloudwatch.StandardUnitBytes,
	"procstat_write_count":                  cloudwatch.StandardUnitCount,
	"procstat_memory_data":                  cloudwatch.StandardUnitBytes,
	"procstat_memory_locked":                cloudwatch.StandardUnitBytes,
	"procstat_memory_rss":                   cloudwatch.StandardUnitBytes,
	"procstat_memory_stack":                 cloudwatch.StandardUnitBytes,
	"procstat_memory_swap":                  cloudwatch.StandardUnitBytes,
	"procstat_memory_vms":                   cloudwatch.StandardUnitBytes,
	"procstat_cpu_usage":                    cloudwatch.StandardUnitPercent,
	"procstat_cpu_time":                     cloudwatch.StandardUnitCount,
	"procstat_cpu_time_user":                cloudwatch.StandardUnitCount,
	"procstat_cpu_time_guest":               cloudwatch.StandardUnitCount,
	"procstat_cpu_time_guest_nice":          cloudwatch.StandardUnitCount,
	"procstat_cpu_time_idle":                cloudwatch.StandardUnitCount,
	"procstat_cpu_time_iowait":              cloudwatch.StandardUnitCount,
	"procstat_cpu_time_irq":                 cloudwatch.StandardUnitCount,
	"procstat_cpu_time_soft_irq":            cloudwatch.StandardUnitCount,
	"procstat_cpu_time_nice":                cloudwatch.StandardUnitCount,
	"procstat_cpu_time_steal":               cloudwatch.StandardUnitCount,
	"procstat_cpu_time_stolen":              cloudwatch.StandardUnitCount,
	"procstat_rlimit_cpu_time_hard":         cloudwatch.StandardUnitCount,
	"procstat_rlimit_cpu_time_soft":         cloudwatch.StandardUnitCount,
	"procstat_rlimit_file_locks_hard":       cloudwatch.StandardUnitCount,
	"procstat_rlimit_file_locks_soft":       cloudwatch.StandardUnitCount,
	"procstat_rlimit_memory_data_hard":      cloudwatch.StandardUnitBytes,
	"procstat_rlimit_memory_data_soft":      cloudwatch.StandardUnitBytes,
	"procstat_rlimit_memory_locked_hard":    cloudwatch.StandardUnitBytes,
	"procstat_rlimit_memory_locked_soft":    cloudwatch.StandardUnitBytes,
	"procstat_rlimit_memory_rss_hard":       cloudwatch.StandardUnitBytes,
	"procstat_rlimit_memory_rss_soft":       cloudwatch.StandardUnitBytes,
	"procstat_rlimit_memory_stack_hard":     cloudwatch.StandardUnitBytes,
	"procstat_rlimit_memory_stack_soft":     cloudwatch.StandardUnitBytes,
	"procstat_rlimit_memory_vms_hard":       cloudwatch.StandardUnitBytes,
	"procstat_rlimit_memory_vms_soft":       cloudwatch.StandardUnitBytes,
	"procstat_pid":                          cloudwatch.StandardUnitCount,
	"procstat_pid_count":                    cloudwatch.StandardUnitCount,
	"procstat_nice_priority":                cloudwatch.StandardUnitCount,
	"procstat_realtime_priority":            cloudwatch.StandardUnitCount,
	"procstat_signals_pending":              cloudwatch.StandardUnitCount,
	"procstat_voluntary_context_switches":   cloudwatch.StandardUnitCount,
	"procstat_involuntary_context_switches": cloudwatch.StandardUnitCount,

	"cpu_usage_active":     cloudwatch.StandardUnitPercent,
	"cpu_usage_idle":       cloudwatch.StandardUnitPercent,
	"cpu_usage_nice":       cloudwatch.StandardUnitPercent,
	"cpu_usage_guest":      cloudwatch.StandardUnitPercent,
	"cpu_usage_guest_nice": cloudwatch.StandardUnitPercent,
	"cpu_usage_iowait":     cloudwatch.StandardUnitPercent,
	"cpu_usage_irq":        cloudwatch.StandardUnitPercent,
	"cpu_usage_softirq":    cloudwatch.StandardUnitPercent,
	"cpu_usage_steal":      cloudwatch.StandardUnitPercent,
	"cpu_usage_system":     cloudwatch.StandardUnitPercent,
	"cpu_usage_user":       cloudwatch.StandardUnitPercent,

	"disk_free":         cloudwatch.StandardUnitBytes,
	"disk_total":        cloudwatch.StandardUnitBytes,
	"disk_used":         cloudwatch.StandardUnitBytes,
	"disk_inodes_free":  cloudwatch.StandardUnitCount,
	"disk_inodes_total": cloudwatch.StandardUnitCount,
	"disk_inodes_used":  cloudwatch.StandardUnitCount,
	"disk_used_percent": cloudwatch.StandardUnitPercent,

	"diskio_iops_in_progress": cloudwatch.StandardUnitCount,
	"diskio_io_time":          cloudwatch.StandardUnitMilliseconds,
	"diskio_reads":            cloudwatch.StandardUnitCount,
	"diskio_writes":           cloudwatch.StandardUnitCount,
	"diskio_read_bytes":       cloudwatch.StandardUnitBytes,
	"diskio_write_bytes":      cloudwatch.StandardUnitBytes,
	"diskio_read_time":        cloudwatch.StandardUnitMilliseconds,
	"diskio_write_time":       cloudwatch.StandardUnitMilliseconds,

	"swap_used":         cloudwatch.StandardUnitBytes,
	"swap_total":        cloudwatch.StandardUnitBytes,
	"swap_used_percent": cloudwatch.StandardUnitPercent,
	"swap_free":         cloudwatch.StandardUnitBytes,

	"mem_used":              cloudwatch.StandardUnitBytes,
	"mem_cached":            cloudwatch.StandardUnitBytes,
	"mem_total":             cloudwatch.StandardUnitBytes,
	"mem_available":         cloudwatch.StandardUnitBytes,
	"mem_free":              cloudwatch.StandardUnitBytes,
	"mem_buffered":          cloudwatch.StandardUnitBytes,
	"mem_active":            cloudwatch.StandardUnitBytes,
	"mem_inactive":          cloudwatch.StandardUnitBytes,
	"mem_available_percent": cloudwatch.StandardUnitPercent,
	"mem_used_percent":      cloudwatch.StandardUnitPercent,

	"net_bytes_sent":   cloudwatch.StandardUnitBytes,
	"net_bytes_recv":   cloudwatch.StandardUnitBytes,
	"net_drop_in":      cloudwatch.StandardUnitCount,
	"net_drop_out":     cloudwatch.StandardUnitCount,
	"net_err_in":       cloudwatch.StandardUnitCount,
	"net_err_out":      cloudwatch.StandardUnitCount,
	"net_packets_sent": cloudwatch.StandardUnitCount,
	"net_packets_recv": cloudwatch.StandardUnitCount,

	"netstat_tcp_established": cloudwatch.StandardUnitCount,
	"netstat_tcp_syn_sent":    cloudwatch.StandardUnitCount,
	"netstat_tcp_syn_recv":    cloudwatch.StandardUnitCount,
	"netstat_tcp_close":       cloudwatch.StandardUnitCount,
	"netstat_tcp_close_wait":  cloudwatch.StandardUnitCount,
	"netstat_tcp_closing":     cloudwatch.StandardUnitCount,
	"netstat_tcp_fin_wait1":   cloudwatch.StandardUnitCount,
	"netstat_tcp_fin_wait2":   cloudwatch.StandardUnitCount,
	"netstat_tcp_last_ack":    cloudwatch.StandardUnitCount,
	"netstat_tcp_listen":      cloudwatch.StandardUnitCount,
	"netstat_tcp_none":        cloudwatch.StandardUnitCount,
	"netstat_tcp_time_wait":   cloudwatch.StandardUnitCount,
	"netstat_udp_socket":      cloudwatch.StandardUnitCount,

	"processes_blocked":       cloudwatch.StandardUnitCount,
	"processes_idle":          cloudwatch.StandardUnitCount,
	"processes_paging":        cloudwatch.StandardUnitCount,
	"processes_stopped":       cloudwatch.StandardUnitCount,
	"processes_total":         cloudwatch.StandardUnitCount,
	"processes_total_threads": cloudwatch.StandardUnitCount,
	"processes_wait":          cloudwatch.StandardUnitCount,
	"processes_zombie":        cloudwatch.StandardUnitCount,
	"processes_running":       cloudwatch.StandardUnitCount,
	"processes_sleeping":      cloudwatch.StandardUnitCount,
	"processes_dead":          cloudwatch.StandardUnitCount,
}

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
