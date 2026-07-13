//go:build linux

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package systemmetricsreceiver

import (
	"context"
	"strings"
	"sync"

	"github.com/safchain/ethtool"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

// PS abstracts OS-level data sources for testing.
type PS interface {
	CPUTimes(ctx context.Context) ([]cpu.TimesStat, error)
	VMStat(ctx context.Context) (*mem.VirtualMemoryStat, error)
	DiskUsage(ctx context.Context) ([]*disk.UsageStat, error)
	EthtoolStats(ctx context.Context, iface string) (map[string]uint64, error)
}

type SystemPS struct {
	ethtool     *ethtool.Ethtool
	ethtoolOnce sync.Once
	ethtoolErr  error
}

func (s *SystemPS) CPUTimes(ctx context.Context) ([]cpu.TimesStat, error) {
	return cpu.TimesWithContext(ctx, false)
}

func (s *SystemPS) VMStat(ctx context.Context) (*mem.VirtualMemoryStat, error) {
	return mem.VirtualMemoryWithContext(ctx)
}

func (s *SystemPS) DiskUsage(ctx context.Context) ([]*disk.UsageStat, error) {
	partitions, err := disk.PartitionsWithContext(ctx, true)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{})
	var stats []*disk.UsageStat
	for _, p := range partitions {
		if !strings.HasPrefix(p.Device, "/dev/") {
			continue
		}
		if _, ok := seen[p.Mountpoint]; ok {
			continue
		}
		seen[p.Mountpoint] = struct{}{}
		usage, err := disk.UsageWithContext(ctx, p.Mountpoint)
		if err != nil || usage.Total == 0 {
			continue
		}
		stats = append(stats, usage)
	}
	return stats, nil
}

func (s *SystemPS) EthtoolStats(_ context.Context, iface string) (map[string]uint64, error) {
	s.ethtoolOnce.Do(func() {
		s.ethtool, s.ethtoolErr = ethtool.NewEthtool()
	})
	if s.ethtoolErr != nil {
		return nil, s.ethtoolErr
	}
	return s.ethtool.Stats(iface)
}

func (s *SystemPS) Close() {
	if s.ethtool != nil {
		s.ethtool.Close()
		s.ethtool = nil
	}
}
