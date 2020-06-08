package metric

import (
	"github.com/aws/amazon-cloudwatch-agent/tool/data/config/metric/collectd"
	"github.com/aws/amazon-cloudwatch-agent/tool/data/config/metric/linux"
	"github.com/aws/amazon-cloudwatch-agent/tool/data/config/metric/statsd"
	"github.com/aws/amazon-cloudwatch-agent/tool/data/config/metric/windows"
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

type Collection struct {
	//linux
	CPU     *linux.CPU
	Disk    *linux.Disk
	DiskIO  *linux.DiskIO
	Memory  *linux.Memory
	Net     *linux.Net
	NetStat *linux.NetStat
	Swap    *linux.Swap

	//windows
	WinLogicalDisk      *windows.LogicalDisk
	WinPhysicalDisk     *windows.PhysicalDisk
	WinMemory           *windows.Memory
	WinNetworkInterface *windows.NetworkInterface
	WinProcessor        *windows.Processor
	WinTCPv4            *windows.TCPv4
	WinTCPv6            *windows.TCPv6
	WinPagingFile       *windows.PagingFile

	//statsd
	StatsD *statsd.StatsD

	//collectd linux only
	CollectD *collectd.CollectD
}

func (config *Collection) ToMap(ctx *runtime.Context) (string, map[string]interface{}) {
	resultMap := make(map[string]interface{})
	if ctx.OsParameter == util.OsTypeWindows {
		if config.WinLogicalDisk != nil {
			util.AddToMap(ctx, resultMap, config.WinLogicalDisk)
		}
		if config.WinPhysicalDisk != nil {
			util.AddToMap(ctx, resultMap, config.WinPhysicalDisk)
		}
		if config.WinMemory != nil {
			util.AddToMap(ctx, resultMap, config.WinMemory)
		}
		if config.WinNetworkInterface != nil {
			util.AddToMap(ctx, resultMap, config.WinNetworkInterface)
		}
		if config.WinProcessor != nil {
			util.AddToMap(ctx, resultMap, config.WinProcessor)
		}
		if config.WinTCPv4 != nil {
			util.AddToMap(ctx, resultMap, config.WinTCPv4)
		}
		if config.WinTCPv6 != nil {
			util.AddToMap(ctx, resultMap, config.WinTCPv6)
		}
		if config.WinPagingFile != nil {
			util.AddToMap(ctx, resultMap, config.WinPagingFile)
		}
	} else {
		//Difficult to check an interface is nil or not. https://github.com/golang/go/issues/17346
		if config.CPU != nil {
			util.AddToMap(ctx, resultMap, config.CPU)
		}
		if config.Disk != nil {
			util.AddToMap(ctx, resultMap, config.Disk)
		}
		if config.DiskIO != nil {
			util.AddToMap(ctx, resultMap, config.DiskIO)
		}
		if config.Memory != nil {
			util.AddToMap(ctx, resultMap, config.Memory)
		}
		if config.Net != nil {
			util.AddToMap(ctx, resultMap, config.Net)
		}
		if config.NetStat != nil {
			util.AddToMap(ctx, resultMap, config.NetStat)
		}
		if config.Swap != nil {
			util.AddToMap(ctx, resultMap, config.Swap)
		}
		if config.CollectD != nil {
			util.AddToMap(ctx, resultMap, config.CollectD)
		}
	}
	if config.StatsD != nil {
		util.AddToMap(ctx, resultMap, config.StatsD)
	}
	return "metrics_collected", resultMap
}

func (config *Collection) EnableAll(ctx *runtime.Context) {
	if ctx.OsParameter == util.OsTypeWindows {
		config.WinLogicalDisk = new(windows.LogicalDisk)
		config.WinLogicalDisk.Enable()
		config.WinPhysicalDisk = new(windows.PhysicalDisk)
		config.WinPhysicalDisk.Enable()
		config.WinMemory = new(windows.Memory)
		config.WinMemory.Enable()
		config.WinNetworkInterface = new(windows.NetworkInterface)
		config.WinNetworkInterface.Enable()
		config.WinProcessor = new(windows.Processor)
		config.WinProcessor.Enable()
		config.WinTCPv4 = new(windows.TCPv4)
		config.WinTCPv4.Enable()
		config.WinTCPv6 = new(windows.TCPv6)
		config.WinTCPv6.Enable()
		config.WinPagingFile = new(windows.PagingFile)
		config.WinPagingFile.Enable()
	} else {
		config.CPU = new(linux.CPU)
		config.CPU.Enable()
		config.Disk = new(linux.Disk)
		config.Disk.Enable()
		config.DiskIO = new(linux.DiskIO)
		config.DiskIO.Enable()
		config.Memory = new(linux.Memory)
		config.Memory.Enable()
		config.Net = new(linux.Net)
		config.Net.Enable()
		config.NetStat = new(linux.NetStat)
		config.NetStat.Enable()
		config.Swap = new(linux.Swap)
		config.Swap.Enable()
		config.CollectD = new(collectd.CollectD)
		config.CollectD.Enable()
	}
	config.StatsD = new(statsd.StatsD)
	config.StatsD.Enable()
}
