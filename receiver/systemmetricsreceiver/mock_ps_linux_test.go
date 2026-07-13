//go:build linux

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package systemmetricsreceiver

import (
	"context"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

// MockPS implements PS for testing.
type MockPS struct {
	CPUTimesData     []cpu.TimesStat
	CPUTimesErr      error
	VMStatData       *mem.VirtualMemoryStat
	VMStatErr        error
	DiskUsageData    []*disk.UsageStat
	DiskUsageErr     error
	EthtoolStatsData map[string]uint64
	EthtoolStatsErr  error
}

func (m *MockPS) CPUTimes(_ context.Context) ([]cpu.TimesStat, error) {
	return m.CPUTimesData, m.CPUTimesErr
}

func (m *MockPS) VMStat(_ context.Context) (*mem.VirtualMemoryStat, error) {
	return m.VMStatData, m.VMStatErr
}

func (m *MockPS) DiskUsage(_ context.Context) ([]*disk.UsageStat, error) {
	return m.DiskUsageData, m.DiskUsageErr
}

func (m *MockPS) EthtoolStats(_ context.Context, _ string) (map[string]uint64, error) {
	return m.EthtoolStatsData, m.EthtoolStatsErr
}
