// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package accumulator

// Default unit for telegraf metrics based on the measurement and the
// field name
var defaultUnits = map[string]map[string]string{
	"procstat": {
		"cpu_usage":                 "Percent",
		"memory_data":               "Bytes",
		"memory_locked":             "Bytes",
		"memory_rss":                "Bytes",
		"memory_stack":              "Bytes",
		"memory_swap":               "Bytes",
		"memory_vms":                "Bytes",
		"read_bytes":                "Bytes",
		"write_bytes":               "Bytes",
		"rlimit_memory_data_hard":   "Bytes",
		"rlimit_memory_data_soft":   "Bytes",
		"rlimit_memory_locked_hard": "Bytes",
		"rlimit_memory_locked_soft": "Bytes",
		"rlimit_memory_rss_hard":    "Bytes",
		"rlimit_memory_rss_soft":    "Bytes",
		"rlimit_memory_stack_hard":  "Bytes",
		"rlimit_memory_stack_soft":  "Bytes",
		"rlimit_memory_vms_hard":    "Bytes",
		"rlimit_memory_vms_soft":    "Bytes",
	},
	"cpu": {
		"usage_active":     "Percent",
		"usage_idle":       "Percent",
		"usage_nice":       "Percent",
		"usage_guest":      "Percent",
		"usage_guest_nice": "Percent",
		"usage_iowait":     "Percent",
		"usage_irq":        "Percent",
		"usage_softirq":    "Percent",
		"usage_steal":      "Percent",
		"usage_system":     "Percent",
		"usage_user":       "Percent",
	},
	"disk": {
		"free":         "Bytes",
		"total":        "Bytes",
		"used":         "Bytes",
		"inodes_free":  "Count",
		"inodes_total": "Count",
		"inodes_used":  "Count",
		"used_percent": "Percent",
	},
	"diskio": {
		"iops_in_progress": "Count",
		"io_time":          "Milliseconds",
		"reads":            "Count",
		"writes":           "Count",
		"read_bytes":       "Bytes",
		"write_bytes":      "Bytes",
		"read_time":        "Milliseconds",
		"write_time":       "Milliseconds",
	},

	"swap": {
		"used":         "Bytes",
		"total":        "Bytes",
		"used_percent": "Percent",
		"free":         "Bytes",
	},
	"mem": {
		"used":              "Bytes",
		"cached":            "Bytes",
		"total":             "Bytes",
		"available":         "Bytes",
		"free":              "Bytes",
		"buffered":          "Bytes",
		"active":            "Bytes",
		"inactive":          "Bytes",
		"available_percent": "Percent",
		"used_percent":      "Percent",
	},
	"net": {
		"bytes_sent":   "Bytes",
		"bytes_recv":   "Bytes",
		"drop_in":      "Count",
		"drop_out":     "Count",
		"err_in":       "Count",
		"err_out":      "Count",
		"packets_sent": "Count",
		"packets_recv": "Count",
	},
	"netstat": {
		"tcp_established": "Count",
		"tcp_syn_sent":    "Count",
		"tcp_syn_recv":    "Count",
		"tcp_close":       "Count",
		"tcp_close_wait":  "Count",
		"tcp_closing":     "Count",
		"tcp_fin_wait1":   "Count",
		"tcp_fin_wait2":   "Count",
		"tcp_last_ack":    "Count",
		"tcp_listen":      "Count",
		"tcp_none":        "Count",
		"tcp_time_wait":   "Count",
		"udp_socket":      "Count",
	},
	"processes": {
		"blocked":       "Count",
		"idle":          "Count",
		"paging":        "Count",
		"stopped":       "Count",
		"total":         "Count",
		"total_threads": "Count",
		"wait":          "Count",
		"zombie":        "Count",
		"running":       "Count",
		"sleeping":      "Count",
		"dead":          "Count",
	},
}

func getDefaultUnit(measurement string, fieldKey string) string {
	supportedFieldsUnit, ok := defaultUnits[measurement]
	if !ok {
		return ""
	}

	fieldUnit, ok := supportedFieldsUnit[fieldKey]
	if !ok {
		return ""
	}

	return fieldUnit
}
