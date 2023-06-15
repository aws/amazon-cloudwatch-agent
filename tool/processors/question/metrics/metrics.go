// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metrics

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/data"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/data/config/metric/linux"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/data/config/metric/windows"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/processors"
	linuxMigration "github.com/aws/private-amazon-cloudwatch-agent-staging/tool/processors/migration/linux"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/processors/question/logs"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/runtime"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/util"
)

var Processor processors.Processor = &processor{}

type processor struct{}

func (p *processor) Process(ctx *runtime.Context, config *data.Config) {
	monitorMetrics(ctx, config)
}

func (p *processor) NextProcessor(ctx *runtime.Context, config *data.Config) interface{} {
	if ctx.OsParameter == util.OsTypeWindows {
		return logs.Processor
	} else {
		return linuxMigration.Processor
	}
}

func monitorMetrics(ctx *runtime.Context, config *data.Config) {
	if ctx.OsParameter == util.OsTypeWindows {
		monitorWindowsMetrics(ctx, config)
	} else {
		monitorLinuxMetrics(ctx, config)
	}
}

func monitorWindowsMetrics(ctx *runtime.Context, config *data.Config) {
	metrics := config.MetricsConf().Collection()
	yes := util.Yes("Do you want to monitor processor status?")
	if yes {
		metrics.WinProcessor = new(windows.Processor)
		metrics.WinProcessor.Enable()
	}
	yes = util.Yes("Do you want to monitor memory status?")
	if yes {
		metrics.WinMemory = new(windows.Memory)
		metrics.WinMemory.Enable()
	}
	yes = util.Yes("Do you want to monitor disk status?")
	if yes {
		metrics.WinLogicalDisk = new(windows.LogicalDisk)
		metrics.WinLogicalDisk.Enable()
		metrics.WinPhysicalDisk = new(windows.PhysicalDisk)
		metrics.WinPhysicalDisk.Enable()
	}
	yes = util.Yes("Do you want to monitor network status?")
	if yes {
		metrics.WinNetworkInterface = new(windows.NetworkInterface)
		metrics.WinNetworkInterface.Enable()
		metrics.WinTCPv4 = new(windows.TCPv4)
		metrics.WinTCPv4.Enable()
		metrics.WinTCPv6 = new(windows.TCPv6)
		metrics.WinTCPv6.Enable()
	}
	yes = util.Yes("Do you want to monitor paging file status?")
	if yes {
		metrics.WinPagingFile = new(windows.PagingFile)
		metrics.WinPagingFile.Enable()
	}
}

func monitorLinuxMetrics(ctx *runtime.Context, config *data.Config) {
	metrics := config.MetricsConf().Collection()
	yes := util.Yes("Do you want to monitor CPU status?")
	if yes {
		metrics.CPU = new(linux.CPU)
		metrics.CPU.Enable()
	}
	yes = util.Yes("Do you want to monitor memory status?")
	if yes {
		metrics.Memory = new(linux.Memory)
		metrics.Memory.Enable()
	}
	yes = util.Yes("Do you want to monitor disk status?")
	if yes {
		metrics.Disk = new(linux.Disk)
		metrics.Disk.Enable()
		metrics.DiskIO = new(linux.DiskIO)
		metrics.DiskIO.Enable()
	}
	yes = util.Yes("Do you want to monitor network status?")
	if yes {
		metrics.Net = new(linux.Net)
		metrics.Net.Enable()
		metrics.NetStat = new(linux.NetStat)
		metrics.NetStat.Enable()
	}
	yes = util.Yes("Do you want to monitor swap status?")
	if yes {
		metrics.Swap = new(linux.Swap)
		metrics.Swap.Enable()
	}
}
