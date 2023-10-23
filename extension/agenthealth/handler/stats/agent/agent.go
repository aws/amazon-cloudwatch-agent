// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agent

type Stats struct {
	CpuPercent           *float64 `json:"cpu,omitempty"`
	MemoryBytes          *uint64  `json:"mem,omitempty"`
	FileDescriptorCount  *int32   `json:"fd,omitempty"`
	ThreadCount          *int32   `json:"th,omitempty"`
	LatencyMillis        *int64   `json:"lat,omitempty"`
	PayloadBytes         *int     `json:"load,omitempty"`
	StatusCode           *int     `json:"code,omitempty"`
	SharedConfigFallback *int     `json:"scfb,omitempty"`
	ImdsFallbackSucceed  *int     `json:"ifs,omitempty"`
}

func (s *Stats) Merge(other Stats) {
	if other.CpuPercent != nil {
		s.CpuPercent = other.CpuPercent
	}
	if other.MemoryBytes != nil {
		s.MemoryBytes = other.MemoryBytes
	}
	if other.FileDescriptorCount != nil {
		s.FileDescriptorCount = other.FileDescriptorCount
	}
	if other.ThreadCount != nil {
		s.ThreadCount = other.ThreadCount
	}
	if other.LatencyMillis != nil {
		s.LatencyMillis = other.LatencyMillis
	}
	if other.PayloadBytes != nil {
		s.PayloadBytes = other.PayloadBytes
	}
	if other.StatusCode != nil {
		s.StatusCode = other.StatusCode
	}
	if other.SharedConfigFallback != nil {
		s.SharedConfigFallback = other.SharedConfigFallback
	}
	if other.ImdsFallbackSucceed != nil {
		s.ImdsFallbackSucceed = other.ImdsFallbackSucceed
	}
}

type StatsProvider interface {
	Stats(operation string) Stats
}
