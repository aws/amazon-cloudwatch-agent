package windows

import (
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

type PhysicalDisk struct {
	Instances []string

	PercentDiskTime      bool `% Disk Time`
	DiskWriteBytesPerSec bool `Disk Write Bytes/sec`
	DiskReadBytesPerSec  bool `Disk Read Bytes/sec`
	DiskWritesPerSec     bool `Disk Writes/sec`
	DiskReadsPerSec      bool `Disk Reads/sec`
}

func (config *PhysicalDisk) ToMap(ctx *runtime.Context) (string, map[string]interface{}) {
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
	if config.PercentDiskTime {
		measurement = append(measurement, "% Disk Time")
	}
	if config.DiskWriteBytesPerSec {
		measurement = append(measurement, "Disk Write Bytes/sec")
	}
	if config.DiskReadBytesPerSec {
		measurement = append(measurement, "Disk Read Bytes/sec")
	}
	if config.DiskWritesPerSec {
		measurement = append(measurement, "Disk Writes/sec")
	}
	if config.DiskReadsPerSec {
		measurement = append(measurement, "Disk Reads/sec")
	}
	resultMap[util.MapKeyMeasurement] = measurement

	return "PhysicalDisk", resultMap
}

func (config *PhysicalDisk) Enable() {
	config.PercentDiskTime = true
	config.DiskWriteBytesPerSec = true
	config.DiskReadBytesPerSec = true
	config.DiskWritesPerSec = true
	config.DiskReadsPerSec = true
}
