// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package provider

import (
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/shirou/gopsutil/v3/process"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
)

const (
	processGetInterval = time.Minute
)

var (
	processSingleton *processStats
	processOnce      sync.Once
)

type processMetrics interface {
	CPUPercent() (float64, error)
	MemoryInfo() (*process.MemoryInfoStat, error)
	NumFDs() (int32, error)
	NumThreads() (int32, error)
}

type processStats struct {
	*intervalStats

	proc processMetrics
}

var _ agent.StatsProvider = (*processStats)(nil)

func (p *processStats) cpuPercent() *float64 {
	if cpuPercent, err := p.proc.CPUPercent(); err == nil {
		return aws.Float64(float64(int64(cpuPercent*10)) / 10) // truncate to 10th decimal place
	}
	return nil
}

func (p *processStats) memoryBytes() *uint64 {
	if memInfo, err := p.proc.MemoryInfo(); err == nil {
		return aws.Uint64(memInfo.RSS)
	}
	return nil
}

func (p *processStats) fileDescriptorCount() *int32 {
	if fdCount, err := p.proc.NumFDs(); err == nil {
		return aws.Int32(fdCount)
	}
	return nil
}

func (p *processStats) threadCount() *int32 {
	if thCount, err := p.proc.NumThreads(); err == nil {
		return aws.Int32(thCount)
	}
	return nil
}

func (p *processStats) updateLoop() {
	ticker := time.NewTicker(p.interval)
	for range ticker.C {
		p.refresh()
	}
}

func (p *processStats) refresh() {
	p.stats.Store(agent.Stats{
		CpuPercent:          p.cpuPercent(),
		MemoryBytes:         p.memoryBytes(),
		FileDescriptorCount: p.fileDescriptorCount(),
		ThreadCount:         p.threadCount(),
	})
}

func newProcessStats(proc processMetrics, interval time.Duration) *processStats {
	ps := &processStats{
		intervalStats: newIntervalStats(interval),
		proc:          proc,
	}
	ps.refresh()
	go ps.updateLoop()
	return ps
}

func GetProcessStats() agent.StatsProvider {
	processOnce.Do(func() {
		proc, _ := process.NewProcess(int32(os.Getpid()))
		processSingleton = newProcessStats(proc, processGetInterval)
	})
	return processSingleton
}
