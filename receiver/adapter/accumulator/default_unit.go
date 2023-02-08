// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package accumulator

// default units
// follow the format "Prefix_Metric"
var defaultUnits = map[string]string{
	"procstat_cpu_usage": "Percent",

	"procstat_memory_data":   "Bytes",
	"procstat_memory_locked": "Bytes",
	"procstat_memory_rss":    "Bytes",
	"procstat_memory_stack":  "Bytes",
	"procstat_memory_swap":   "Bytes",
	"procstat_memory_vms":    "Bytes",

	"procstat_read_bytes":  "Bytes",
	"procstat_write_bytes": "Bytes",

	"procstat_rlimit_memory_data_hard":   "Bytes",
	"procstat_rlimit_memory_data_soft":   "Bytes",
	"procstat_rlimit_memory_locked_hard": "Bytes",
	"procstat_rlimit_memory_locked_soft": "Bytes",
	"procstat_rlimit_memory_rss_hard":    "Bytes",
	"procstat_rlimit_memory_rss_soft":    "Bytes",
	"procstat_rlimit_memory_stack_hard":  "Bytes",
	"procstat_rlimit_memory_stack_soft":  "Bytes",
	"procstat_rlimit_memory_vms_hard":    "Bytes",
	"procstat_rlimit_memory_vms_soft":    "Bytes",

	"cpu_usage_active":     "Percent",
	"cpu_usage_idle":       "Percent",
	"cpu_usage_nice":       "Percent",
	"cpu_usage_guest":      "Percent",
	"cpu_usage_guest_nice": "Percent",
	"cpu_usage_iowait":     "Percent",
	"cpu_usage_irq":        "Percent",
	"cpu_usage_softirq":    "Percent",
	"cpu_usage_steal":      "Percent",
	"cpu_usage_system":     "Percent",
	"cpu_usage_user":       "Percent",

	"disk_free":         "Bytes",
	"disk_total":        "Bytes",
	"disk_used":         "Bytes",
	"disk_inodes_free":  "Count",
	"disk_inodes_total": "Count",
	"disk_inodes_used":  "Count",
	"disk_used_percent": "Percent",

	"diskio_iops_in_progress": "Count",
	"diskio_io_time":          "Milliseconds",
	"diskio_reads":            "Count",
	"diskio_writes":           "Count",
	"diskio_read_bytes":       "Bytes",
	"diskio_write_bytes":      "Bytes",
	"diskio_read_time":        "Milliseconds",
	"diskio_write_time":       "Milliseconds",

	"swap_used":         "Bytes",
	"swap_total":        "Bytes",
	"swap_used_percent": "Percent",
	"swap_free":         "Bytes",

	"mem_used":              "Bytes",
	"mem_cached":            "Bytes",
	"mem_total":             "Bytes",
	"mem_available":         "Bytes",
	"mem_free":              "Bytes",
	"mem_buffered":          "Bytes",
	"mem_active":            "Bytes",
	"mem_inactive":          "Bytes",
	"mem_available_percent": "Percent",
	"mem_used_percent":      "Percent",

	"net_bytes_sent":   "Bytes",
	"net_bytes_recv":   "Bytes",
	"net_drop_in":      "Count",
	"net_drop_out":     "Count",
	"net_err_in":       "Count",
	"net_err_out":      "Count",
	"net_packets_sent": "Count",
	"net_packets_recv": "Count",

	"netstat_tcp_established": "Count",
	"netstat_tcp_syn_sent":    "Count",
	"netstat_tcp_syn_recv":    "Count",
	"netstat_tcp_close":       "Count",
	"netstat_tcp_close_wait":  "Count",
	"netstat_tcp_closing":     "Count",
	"netstat_tcp_fin_wait1":   "Count",
	"netstat_tcp_fin_wait2":   "Count",
	"netstat_tcp_last_ack":    "Count",
	"netstat_tcp_listen":      "Count",
	"netstat_tcp_none":        "Count",
	"netstat_tcp_time_wait":   "Count",
	"netstat_udp_socket":      "Count",

	"processes_blocked":       "Count",
	"processes_idle":          "Count",
	"processes_paging":        "Count",
	"processes_stopped":       "Count",
	"processes_total":         "Count",
	"processes_total_threads": "Count",
	"processes_wait":          "Count",
	"processes_zombie":        "Count",
	"processes_running":       "Count",
	"processes_sleeping":      "Count",
	"processes_dead":          "Count",
}

func getDefaultUnit(metricName string) string {
	if unit, ok := defaultUnits[metricName]; ok {
		return unit
	}
	return ""
}
