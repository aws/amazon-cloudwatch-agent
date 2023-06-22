// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package linux

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/runtime"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/util"
)

type DiskIO struct {
	Instances []string

	IOTime     bool `io_time`
	WriteBytes bool `write_bytes`
	ReadBytes  bool `read_bytes`
	Writes     bool `writes`
	Reads      bool `reads`
}

func (config *DiskIO) ToMap(ctx *runtime.Context) (string, map[string]interface{}) {
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
	if config.IOTime {
		measurement = append(measurement, "io_time")
	}
	if config.WriteBytes {
		measurement = append(measurement, "write_bytes")
	}
	if config.ReadBytes {
		measurement = append(measurement, "read_bytes")
	}
	if config.Writes {
		measurement = append(measurement, "writes")
	}
	if config.Reads {
		measurement = append(measurement, "reads")
	}
	resultMap[util.MapKeyMeasurement] = measurement
	return "diskio", resultMap
}

func (config *DiskIO) Enable() {
	config.IOTime = true
	config.WriteBytes = true
	config.ReadBytes = true
	config.Writes = true
	config.Reads = true
}
