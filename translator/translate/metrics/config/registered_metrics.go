// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package config

// This served as the allowlisted metric name, which is registered under the plugin name
// Note: the registered metric name don't have plugin name as prefix
var Registered_Metrics_Linux = map[string][]string{
	"cpu": {"time_active", "time_guest", "time_guest_nice", "time_idle", "time_iowait", "time_irq", "time_nice", "time_softirq", "time_steal", "time_system", "time_user",
		"usage_active", "usage_guest", "usage_guest_nice", "usage_idle", "usage_iowait", "usage_irq", "usage_nice", "usage_softirq", "usage_steal", "usage_system", "usage_user"},
	"disk":      {"free", "inodes_free", "inodes_total", "inodes_used", "total", "used", "used_percent"},
	"diskio":    {"iops_in_progress", "io_time", "reads", "read_bytes", "read_time", "writes", "write_bytes", "write_time"},
	"swap":      {"free", "used", "used_percent"},
	"mem":       {"active", "available", "available_percent", "buffered", "cached", "free", "inactive", "total", "used", "used_percent"},
	"net":       {"bytes_sent", "bytes_recv", "drop_in", "drop_out", "err_in", "err_out", "packets_sent", "packets_recv"},
	"netstat":   {"tcp_close", "tcp_close_wait", "tcp_closing", "tcp_established", "tcp_fin_wait1", "tcp_fin_wait2", "tcp_last_ack", "tcp_listen", "tcp_none", "tcp_syn_sent", "tcp_syn_recv", "tcp_time_wait", "udp_socket"},
	"processes": {"blocked", "dead", "idle", "paging", "running", "sleeping", "stopped", "total", "total_threads", "wait", "zombies"},
	"procstat": {"cpu_time", "cpu_time_guest", "cpu_time_guest_nice", "cpu_time_idle", "cpu_time_iowait", "cpu_time_irq", "cpu_time_nice", "cpu_time_soft_irq", "cpu_time_steal", "cpu_time_stolen", "cpu_time_system", "cpu_time_user", "cpu_usage", "involuntary_context_switches",
		"memory_data", "memory_locked", "memory_rss", "memory_stack", "memory_swap", "memory_vms", "nice_priority", "num_fds", "num_threads", "pid",
		"read_bytes", "read_count", "realtime_priority", "rlimit_cpu_time_hard", "rlimit_cpu_time_soft", "rlimit_file_locks_hard", "rlimit_file_locks_soft", "rlimit_memory_data_hard", "rlimit_memory_data_soft", "rlimit_memory_locked_hard", "rlimit_memory_locked_soft",
		"rlimit_memory_rss_hard", "rlimit_memory_rss_soft", "rlimit_memory_stack_hard", "rlimit_memory_stack_soft", "rlimit_memory_vms_hard", "rlimit_memory_vms_soft", "rlimit_nice_priority_hard", "rlimit_nice_priority_soft", "rlimit_num_fds_hard", "rlimit_num_fds_soft",
		"rlimit_realtime_priority_hard", "rlimit_realtime_priority_soft", "rlimit_signals_pending_hard", "rlimit_signals_pending_soft", "signals_pending", "voluntary_context_switches", "write_bytes", "write_count", "pid_count"},
	"nvidia_smi": {"utilization_gpu", "temperature_gpu", "power_draw", "utilization_memory", "fan_speed", "memory_total", "memory_used", "memory_free", "temperature_gpu", "pcie_link_gen_current", "pcie_link_width_current",
		"encoder_stats_session_count", "encoder_stats_average_fps", "encoder_stats_average_latency", "clocks_current_graphics", "clocks_current_sm", "clocks_current_memory", "clocks_current_video"},
}

// This served as the allowlisted metric name, which is registered under the plugin name
// Note: the registered metric name don't have plugin name as prefix
var Registered_Metrics_Darwin = map[string][]string{
	"cpu": {"time_active", "time_guest", "time_guest_nice", "time_idle", "time_iowait", "time_irq", "time_nice", "time_softirq", "time_steal", "time_system", "time_user",
		"usage_active", "usage_guest", "usage_guest_nice", "usage_idle", "usage_iowait", "usage_irq", "usage_nice", "usage_softirq", "usage_steal", "usage_system", "usage_user"},
	"disk":      {"free", "inodes_free", "inodes_total", "inodes_used", "total", "used", "used_percent"},
	"diskio":    {"iops_in_progress", "io_time", "reads", "read_bytes", "read_time", "writes", "write_bytes", "write_time"},
	"swap":      {"free", "used", "used_percent"},
	"mem":       {"active", "available", "available_percent", "buffered", "cached", "free", "inactive", "total", "used", "used_percent"},
	"net":       {"bytes_sent", "bytes_recv", "drop_in", "drop_out", "err_in", "err_out", "packets_sent", "packets_recv"},
	"netstat":   {"tcp_close", "tcp_close_wait", "tcp_closing", "tcp_established", "tcp_fin_wait1", "tcp_fin_wait2", "tcp_last_ack", "tcp_listen", "tcp_none", "tcp_syn_sent", "tcp_syn_recv", "tcp_time_wait", "udp_socket"},
	"processes": {"blocked", "idle", "running", "sleeping", "stopped", "total", "zombies"},
	"procstat": {"cpu_time_system", "cpu_time_user", "cpu_usage",
		"memory_data", "memory_locked", "memory_rss", "memory_stack", "memory_swap", "memory_vms", "pid",
		"pid_count"},
	"nvidia_smi": {"utilization_gpu", "temperature_gpu", "power_draw", "utilization_memory", "utilization_encoder", "utilization_decoder", "fan_speed", "memory_total", "memory_used", "memory_free", "temperature_gpu", "pcie_link_gen_current", "pcie_link_width_current",
		"encoder_stats_session_count", "encoder_stats_average_fps", "encoder_stats_average_latency", "clocks_current_graphics", "clocks_current_sm", "clocks_current_memory", "clocks_current_video"},
}

var Registered_Metrics_Windows = map[string][]string{
	"Processor":         {"% Idle Time", "% Interrupt Time", "% User Time", "% Processor Time"},
	"LogicalDisk":       {"% Idle Time", "% Disk Read Time", "% Disk Write Time", "% User Time"},
	"Memory":            {"Available Bytes", "Cache Faults/sec", "Page Faults/sec", "Pages/sec"},
	"Network Interface": {"Bytes Received/sec", "Bytes Sent/sec", "Packets Received/sec", "Packets Sent/sec"},
	"System":            {"Context Switches/sec", "System Calls/sec", "Processor Queue Length"},
}

var DisableWinPerfCounters = map[string]bool{
	"statsd":     true,
	"procstat":   true,
	"nvidia_smi": true,
}
