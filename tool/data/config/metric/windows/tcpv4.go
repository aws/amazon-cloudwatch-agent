// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package windows

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/runtime"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/util"
)

type TCPv4 struct {
	ConnectionsEstablished bool `Connections Established`
}

func (config *TCPv4) ToMap(ctx *runtime.Context) (string, map[string]interface{}) {
	resultMap := make(map[string]interface{})
	if ctx.MetricsCollectionInterval != 0 {
		resultMap[util.MapKeyMetricsCollectionInterval] = ctx.MetricsCollectionInterval
	}
	measurement := []string{}
	if config.ConnectionsEstablished {
		measurement = append(measurement, "Connections Established")
	}
	resultMap[util.MapKeyMeasurement] = measurement
	return "TCPv4", resultMap
}

func (config *TCPv4) Enable() {
	config.ConnectionsEstablished = true
}
