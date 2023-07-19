// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metric

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

func TestCollection_ToMap(t *testing.T) {
	expectedKey := "metrics_collected"
	expectedValue := map[string]interface{}{
		"diskio":   map[string]interface{}{"resources": []string{"*"}, "measurement": []string{"io_time", "write_bytes", "read_bytes", "writes", "reads"}},
		"mem":      map[string]interface{}{"measurement": []string{"mem_used_percent"}},
		"net":      map[string]interface{}{"resources": []string{"*"}, "measurement": []string{"bytes_sent", "bytes_recv", "packets_sent", "packets_recv"}},
		"netstat":  map[string]interface{}{"measurement": []string{"tcp_established", "tcp_time_wait"}},
		"swap":     map[string]interface{}{"measurement": []string{"swap_used_percent"}},
		"cpu":      map[string]interface{}{"measurement": []string{"cpu_usage_idle", "cpu_usage_iowait", "cpu_usage_steal", "cpu_usage_guest", "cpu_usage_user", "cpu_usage_system"}, "resources": []string{"*"}, "totalcpu": true},
		"disk":     map[string]interface{}{"resources": []string{"*"}, "measurement": []string{"used_percent", "inodes_free"}},
		"statsd":   map[string]interface{}{"service_address": ":8125", "metrics_collection_interval": 10, "metrics_aggregation_interval": 60},
		"collectd": map[string]interface{}{"metrics_aggregation_interval": 60},
	}
	ctx := &runtime.Context{
		WantPerInstanceMetrics: true,
		IsOnPrem:               true,
		OsParameter:            util.OsTypeLinux,
		WantEC2TagDimensions:   true,
	}
	conf := new(Collection)
	conf.EnableAll(ctx)
	key, value := conf.ToMap(ctx)
	assert.Equal(t, expectedKey, key)
	assert.Equal(t, expectedValue, value)

	// NOTE: mac is same as linux
	ctx = &runtime.Context{
		WantPerInstanceMetrics: true,
		IsOnPrem:               true,
		OsParameter:            util.OsTypeDarwin,
		WantEC2TagDimensions:   true,
	}
	conf = new(Collection)
	conf.EnableAll(ctx)
	key, value = conf.ToMap(ctx)
	assert.Equal(t, expectedKey, key)
	assert.Equal(t, expectedValue, value)

	expectedValue = map[string]interface{}{
		"Network Interface": map[string]interface{}{"resources": []string{"*"}, "measurement": []string{"Bytes Sent/sec", "Bytes Received/sec", "Packets Sent/sec", "Packets Received/sec"}},
		"Processor":         map[string]interface{}{"resources": []string{"*"}, "measurement": []string{"% Processor Time", "% User Time", "% Idle Time", "% Interrupt Time"}},
		"LogicalDisk":       map[string]interface{}{"resources": []string{"*"}, "measurement": []string{"% Free Space"}},
		"Memory":            map[string]interface{}{"measurement": []string{"% Committed Bytes In Use"}},
		"PhysicalDisk":      map[string]interface{}{"resources": []string{"*"}, "measurement": []string{"% Disk Time", "Disk Write Bytes/sec", "Disk Read Bytes/sec", "Disk Writes/sec", "Disk Reads/sec"}},
		"TCPv4":             map[string]interface{}{"measurement": []string{"Connections Established"}},
		"TCPv6":             map[string]interface{}{"measurement": []string{"Connections Established"}},
		"Paging File":       map[string]interface{}{"resources": []string{"*"}, "measurement": []string{"% Usage"}},
		"statsd":            map[string]interface{}{"service_address": ":8125", "metrics_collection_interval": 10, "metrics_aggregation_interval": 60},
	}
	ctx.OsParameter = util.OsTypeWindows
	conf.EnableAll(ctx)
	key, value = conf.ToMap(ctx)
	assert.Equal(t, expectedKey, key)
	assert.Equal(t, expectedValue, value)
}
