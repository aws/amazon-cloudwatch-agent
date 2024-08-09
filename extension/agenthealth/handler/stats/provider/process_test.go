// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package provider

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v3/process"
	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
)

type mockProcessMetrics struct {
	mu  sync.RWMutex
	err error
}

var _ processMetrics = (*mockProcessMetrics)(nil)

func (m *mockProcessMetrics) CPUPercent() (float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.err != nil {
		return -1, m.err
	}
	return 1, nil
}

func (m *mockProcessMetrics) MemoryInfo() (*process.MemoryInfoStat, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.err != nil {
		return nil, m.err
	}
	return &process.MemoryInfoStat{RSS: uint64(2)}, nil
}

func (m *mockProcessMetrics) NumFDs() (int32, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.err != nil {
		return -1, m.err
	}
	return 3, nil
}

func (m *mockProcessMetrics) NumThreads() (int32, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.err != nil {
		return -1, m.err
	}
	return 4, nil
}

func TestProcessStats(t *testing.T) {
	t.Skip("stat provider tests are flaky. disable until fix is available")
	testErr := errors.New("test error")
	mock := &mockProcessMetrics{}
	provider := newProcessStats(mock, time.Millisecond)
	got := provider.getStats()
	assert.NotNil(t, got.CpuPercent)
	assert.NotNil(t, got.MemoryBytes)
	assert.NotNil(t, got.FileDescriptorCount)
	assert.NotNil(t, got.ThreadCount)
	assert.EqualValues(t, 1, *got.CpuPercent)
	assert.EqualValues(t, 2, *got.MemoryBytes)
	assert.EqualValues(t, 3, *got.FileDescriptorCount)
	assert.EqualValues(t, 4, *got.ThreadCount)
	mock.mu.Lock()
	mock.err = testErr
	mock.mu.Unlock()
	provider.refresh()
	assert.Eventually(t, func() bool {
		return provider.getStats() == agent.Stats{}
	}, 5*time.Millisecond, time.Millisecond)
}
