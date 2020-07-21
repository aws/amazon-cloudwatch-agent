// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package linux

import (
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

type NetStat struct {
	TCPEstablished bool `tcp_established`
	TCPTimeWait    bool `tcp_time_wait`
}

func (config *NetStat) ToMap(ctx *runtime.Context) (string, map[string]interface{}) {
	resultMap := make(map[string]interface{})
	if ctx.MetricsCollectionInterval != 0 {
		resultMap[util.MapKeyMetricsCollectionInterval] = ctx.MetricsCollectionInterval
	}
	measurement := []string{}
	if config.TCPEstablished {
		measurement = append(measurement, "tcp_established")
	}
	if config.TCPTimeWait {
		measurement = append(measurement, "tcp_time_wait")
	}
	resultMap[util.MapKeyMeasurement] = measurement
	return "netstat", resultMap
}

func (config *NetStat) Enable() {
	config.TCPEstablished = true
	config.TCPTimeWait = true
}
