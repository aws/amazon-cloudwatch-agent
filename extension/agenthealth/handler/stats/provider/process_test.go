// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package provider

import (
	"errors"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v3/process"
	"github.com/stretchr/testify/assert"
)

type mockProcessMetrics struct {
	err error
}

var _ processMetrics = (*mockProcessMetrics)(nil)

func (m mockProcessMetrics) CPUPercent() (float64, error) {
	if m.err != nil {
		return -1, m.err
	}
	return 1, nil
}

func (m mockProcessMetrics) MemoryInfo() (*process.MemoryInfoStat, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &process.MemoryInfoStat{RSS: uint64(2)}, nil
}

func (m mockProcessMetrics) NumFDs() (int32, error) {
	if m.err != nil {
		return -1, m.err
	}
	return 3, nil
}

func (m mockProcessMetrics) NumThreads() (int32, error) {
	if m.err != nil {
		return -1, m.err
	}
	return 4, nil
}

func TestProcessStats(t *testing.T) {
	testErr := errors.New("test error")
	mock := &mockProcessMetrics{}
	provider := newProcessStats(mock, time.Millisecond)
	got := provider.stats
	assert.NotNil(t, got.CpuPercent)
	assert.NotNil(t, got.MemoryBytes)
	assert.NotNil(t, got.FileDescriptorCount)
	assert.NotNil(t, got.ThreadCount)
	assert.EqualValues(t, 1, *got.CpuPercent)
	assert.EqualValues(t, 2, *got.MemoryBytes)
	assert.EqualValues(t, 3, *got.FileDescriptorCount)
	assert.EqualValues(t, 4, *got.ThreadCount)
	mock.err = testErr
	time.Sleep(2 * time.Millisecond)
	got = provider.stats
	assert.Nil(t, got.CpuPercent)
	assert.Nil(t, got.MemoryBytes)
	assert.Nil(t, got.FileDescriptorCount)
	assert.Nil(t, got.ThreadCount)
}
