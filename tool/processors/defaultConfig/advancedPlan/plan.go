// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package advancedPlan

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/data"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/data/config/metric"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/data/config/metric/linux"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/data/config/metric/windows"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/processors"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/processors/defaultConfig/standardPlan"
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
	standardPlan.ConfigureOnPremMetrics(metrics, ctx)

	if ctx.OsParameter == util.OsTypeWindows {
		metrics.WinProcessor.PercentUserTime = true

		metrics.WinTCPv4 = new(windows.TCPv4)
		metrics.WinTCPv4.ConnectionsEstablished = true

		metrics.WinTCPv6 = new(windows.TCPv6)
		metrics.WinTCPv6.ConnectionsEstablished = true
	} else {
		metrics.CPU.UsageSteal = true
		metrics.CPU.UsageGuest = true
		metrics.CPU.UsageUser = true
		metrics.CPU.UsageSystem = true

		metrics.NetStat = new(linux.NetStat)
		metrics.NetStat.TCPTimeWait = true
		metrics.NetStat.TCPEstablished = true
	}
}

func ConfigureEC2Metrics(metrics *metric.Collection, ctx *runtime.Context) {
	standardPlan.ConfigureEC2Metrics(metrics, ctx)

	if ctx.OsParameter == util.OsTypeWindows {
		metrics.WinPhysicalDisk.DiskWriteBytesPerSec = true
		metrics.WinPhysicalDisk.DiskReadBytesPerSec = true
		metrics.WinPhysicalDisk.DiskWritesPerSec = true
		metrics.WinPhysicalDisk.DiskReadsPerSec = true

		metrics.WinTCPv4 = new(windows.TCPv4)
		metrics.WinTCPv4.ConnectionsEstablished = true

		metrics.WinTCPv6 = new(windows.TCPv6)
		metrics.WinTCPv6.ConnectionsEstablished = true
	} else {
		metrics.DiskIO.WriteBytes = true
		metrics.DiskIO.ReadBytes = true
		metrics.DiskIO.Writes = true
		metrics.DiskIO.Reads = true

		metrics.NetStat = new(linux.NetStat)
		metrics.NetStat.TCPTimeWait = true
		metrics.NetStat.TCPEstablished = true
	}
}
