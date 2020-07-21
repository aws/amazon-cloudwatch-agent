// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package basicPlan

import (
	"github.com/aws/amazon-cloudwatch-agent/tool/data"
	"github.com/aws/amazon-cloudwatch-agent/tool/data/config/metric"
	"github.com/aws/amazon-cloudwatch-agent/tool/data/config/metric/linux"
	"github.com/aws/amazon-cloudwatch-agent/tool/data/config/metric/windows"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors"
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

var Processor processors.Processor = &processor{}

type processor struct{}

func (p *processor) Process(ctx *runtime.Context, conf *data.Config) {
	metricsConfig := conf.MetricsConf()
	metricsCollection := metricsConfig.Collection()
	if ctx.IsOnPrem {
		ConfigureOnPremMetrics(metricsCollection, ctx)
	} else {
		ConfigureEC2Metrics(metricsCollection, ctx)
	}
}

func (p *processor) NextProcessor(ctx *runtime.Context, config *data.Config) interface{} {
	return nil
}

func ConfigureOnPremMetrics(metrics *metric.Collection, ctx *runtime.Context) {
	if ctx.OsParameter == util.OsTypeWindows {
		metrics.WinProcessor = new(windows.Processor)
		metrics.WinProcessor.PercentProcessorTime = true

		metrics.WinLogicalDisk = new(windows.LogicalDisk)
		metrics.WinLogicalDisk.PercentFreeSpace = true

		metrics.WinPhysicalDisk = new(windows.PhysicalDisk)
		metrics.WinPhysicalDisk.DiskWriteBytesPerSec = true
		metrics.WinPhysicalDisk.DiskReadBytesPerSec = true
		metrics.WinPhysicalDisk.DiskWritesPerSec = true
		metrics.WinPhysicalDisk.DiskReadsPerSec = true

		metrics.WinMemory = new(windows.Memory)
		metrics.WinMemory.PercentCommittedBytesInUse = true

		metrics.WinNetworkInterface = new(windows.NetworkInterface)
		metrics.WinNetworkInterface.BytesSentPerSec = true
		metrics.WinNetworkInterface.BytesReceivedPerSec = true
		metrics.WinNetworkInterface.PacketsSentPerSec = true
		metrics.WinNetworkInterface.PacketsReceivedPerSec = true

		metrics.WinPagingFile = new(windows.PagingFile)
		metrics.WinPagingFile.PercentUsage = true
	} else {
		metrics.CPU = new(linux.CPU)
		metrics.CPU.TotalCPU = true
		metrics.CPU.UsageIdle = true

		metrics.DiskIO = new(linux.DiskIO)
		metrics.DiskIO.WriteBytes = true
		metrics.DiskIO.ReadBytes = true
		metrics.DiskIO.Writes = true
		metrics.DiskIO.Reads = true

		metrics.Disk = new(linux.Disk)
		metrics.Disk.UsedPercent = true

		metrics.Memory = new(linux.Memory)
		metrics.Memory.MemUsedPercent = true

		metrics.Net = new(linux.Net)
		metrics.Net.BytesSent = true
		metrics.Net.BytesReceived = true
		metrics.Net.PacketsSent = true
		metrics.Net.PacketsReceived = true

		metrics.Swap = new(linux.Swap)
		metrics.Swap.UsedPercent = true
	}
}

func ConfigureEC2Metrics(metrics *metric.Collection, ctx *runtime.Context) {
	if ctx.OsParameter == util.OsTypeWindows {
		metrics.WinMemory = new(windows.Memory)
		metrics.WinMemory.PercentCommittedBytesInUse = true

		metrics.WinLogicalDisk = new(windows.LogicalDisk)
		metrics.WinLogicalDisk.PercentFreeSpace = true
	} else {
		metrics.Memory = new(linux.Memory)
		metrics.Memory.MemUsedPercent = true

		metrics.Disk = new(linux.Disk)
		metrics.Disk.UsedPercent = true
	}
}
