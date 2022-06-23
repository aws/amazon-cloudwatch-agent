// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package linux

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/runtime"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/util"
)

type Net struct {
	Instances []string

	BytesSent       bool `bytes_sent`
	BytesReceived   bool `bytes_recv`
	PacketsSent     bool `packets_sent`
	PacketsReceived bool `packets_recv`
}

func (config *Net) ToMap(ctx *runtime.Context) (string, map[string]interface{}) {
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
	if config.BytesSent {
		measurement = append(measurement, "bytes_sent")
	}
	if config.BytesReceived {
		measurement = append(measurement, "bytes_recv")
	}
	if config.PacketsSent {
		measurement = append(measurement, "packets_sent")
	}
	if config.BytesReceived {
		measurement = append(measurement, "packets_recv")
	}
	resultMap[util.MapKeyMeasurement] = measurement
	return "net", resultMap
}

func (config *Net) Enable() {
	config.BytesSent = true
	config.BytesReceived = true
	config.PacketsSent = true
	config.PacketsReceived = true
}
