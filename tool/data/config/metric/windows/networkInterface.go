// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package windows

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/runtime"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/util"
)

type NetworkInterface struct {
	Instances []string

	BytesSentPerSec       bool `Bytes Sent/sec`
	BytesReceivedPerSec   bool `Bytes Received/sec`
	PacketsSentPerSec     bool `Packets Sent/sec`
	PacketsReceivedPerSec bool `Packets Received/sec`
}

func (config *NetworkInterface) ToMap(ctx *runtime.Context) (string, map[string]interface{}) {
	resultMap := make(map[string]interface{})
	if config.Instances != nil && len(config.Instances) > 0 {
		resultMap[util.MapKeyInstances] = config.Instances
	} else {
		resultMap[util.MapKeyInstances] = []string{"*"}
	}
	if ctx.MetricsCollectionInterval != 0 {
		resultMap[util.MapKeyMetricsCollectionInterval] = ctx.MetricsCollectionInterval
	}
	measurement := []string{}
	if config.BytesSentPerSec {
		measurement = append(measurement, "Bytes Sent/sec")
	}
	if config.BytesReceivedPerSec {
		measurement = append(measurement, "Bytes Received/sec")
	}
	if config.PacketsSentPerSec {
		measurement = append(measurement, "Packets Sent/sec")
	}
	if config.BytesReceivedPerSec {
		measurement = append(measurement, "Packets Received/sec")
	}
	resultMap[util.MapKeyMeasurement] = measurement
	return "Network Interface", resultMap
}

func (config *NetworkInterface) Enable() {
	config.BytesSentPerSec = true
	config.BytesReceivedPerSec = true
	config.PacketsSentPerSec = true
	config.PacketsReceivedPerSec = true
}
