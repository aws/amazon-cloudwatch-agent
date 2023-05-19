// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package standardPlan

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/data"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/data/config/metric"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/data/config/metric/linux"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/data/config/metric/windows"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/processors"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/processors/defaultConfig/basicPlan"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/runtime"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/util"
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
	basicPlan.ConfigureOnPremMetrics(metrics, ctx)

	if ctx.OsParameter == util.OsTypeWindows {
		metrics.WinProcessor.PercentIdleTime = true
		metrics.WinProcessor.PercentInterruptTime = true

		metrics.WinPhysicalDisk.PercentDiskTime = true
	} else {
		metrics.CPU.UsageIdle = true
		metrics.CPU.UsageIOWait = true

		metrics.DiskIO.IOTime = true
	}
}

func ConfigureEC2Metrics(metrics *metric.Collection, ctx *runtime.Context) {
	basicPlan.ConfigureEC2Metrics(metrics, ctx)

	if ctx.OsParameter == util.OsTypeWindows {
		metrics.WinProcessor = new(windows.Processor)
		metrics.WinProcessor.PercentIdleTime = true
		metrics.WinProcessor.PercentInterruptTime = true
		metrics.WinProcessor.PercentUserTime = true

		metrics.WinPhysicalDisk = new(windows.PhysicalDisk)
		metrics.WinPhysicalDisk.PercentDiskTime = true

		metrics.WinPagingFile = new(windows.PagingFile)
		metrics.WinPagingFile.PercentUsage = true
	} else {
		metrics.CPU = new(linux.CPU)
		metrics.CPU.TotalCPU = false
		metrics.CPU.UsageIdle = true
		metrics.CPU.UsageIOWait = true
		metrics.CPU.UsageUser = true
		metrics.CPU.UsageSystem = true

		metrics.DiskIO = new(linux.DiskIO)
		metrics.DiskIO.IOTime = true

		//UsedPercent is configured in basic one already.
		metrics.Disk.InodesFree = true

		metrics.Swap = new(linux.Swap)
		metrics.Swap.UsedPercent = true
	}
}
