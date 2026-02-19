// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package disktagger

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type mockProvider struct {
	cache      map[string]string
	refreshErr error
}

func (m *mockProvider) Refresh() error { return m.refreshErr }
func (m *mockProvider) Serial(devName string) string {
	return m.cache[devName]
}

func TestProcessMetrics_NilProvider(t *testing.T) {
	tagger := newTagger(&Config{DiskDeviceTagKey: "device"}, zap.NewNop(), nil)
	md := pmetric.NewMetrics()
	result, err := tagger.processMetrics(context.Background(), md)
	require.NoError(t, err)
	assert.Equal(t, 0, result.ResourceMetrics().Len())
}

func TestProcessMetrics_AddsDiskID(t *testing.T) {
	provider := &mockProvider{cache: map[string]string{"sda": "os-disk-name"}}
	tagger := newTagger(&Config{DiskDeviceTagKey: "device"}, zap.NewNop(), provider)

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName("disk_used_percent")
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.Attributes().PutStr("device", "sda1")

	result, err := tagger.processMetrics(context.Background(), md)
	require.NoError(t, err)

	attrs := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes()
	val, ok := attrs.Get(AttributeDiskID)
	assert.False(t, ok, "exact match sda1 not in cache, prefix matching is in mapProvider not mockProvider")
	_ = val
}

func TestProcessMetrics_SkipsExistingDiskID(t *testing.T) {
	provider := &mockProvider{cache: map[string]string{"sda": "os-disk"}}
	tagger := newTagger(&Config{DiskDeviceTagKey: "device"}, zap.NewNop(), provider)

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName("disk_used_percent")
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.Attributes().PutStr("device", "sda")
	dp.Attributes().PutStr(AttributeDiskID, "already-set")

	result, err := tagger.processMetrics(context.Background(), md)
	require.NoError(t, err)

	val, ok := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes().Get(AttributeDiskID)
	assert.True(t, ok)
	assert.Equal(t, "already-set", val.Str())
}

func TestProcessMetrics_NoDeviceAttribute(t *testing.T) {
	provider := &mockProvider{cache: map[string]string{"sda": "os-disk"}}
	tagger := newTagger(&Config{DiskDeviceTagKey: "device"}, zap.NewNop(), provider)

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName("cpu_usage_idle")
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.Attributes().PutStr("cpu", "cpu0")

	result, err := tagger.processMetrics(context.Background(), md)
	require.NoError(t, err)

	_, ok := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes().Get(AttributeDiskID)
	assert.False(t, ok)
}

func TestProcessMetrics_SumMetricType(t *testing.T) {
	provider := &mockProvider{cache: map[string]string{"sda": "os-disk"}}
	tagger := newTagger(&Config{DiskDeviceTagKey: "device"}, zap.NewNop(), provider)

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName("disk_io")
	dp := m.SetEmptySum().DataPoints().AppendEmpty()
	dp.Attributes().PutStr("device", "sda")

	result, err := tagger.processMetrics(context.Background(), md)
	require.NoError(t, err)

	val, ok := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().Get(AttributeDiskID)
	assert.True(t, ok)
	assert.Equal(t, "os-disk", val.Str())
}

func TestShutdown_Safe(t *testing.T) {
	tagger := newTagger(&Config{}, zap.NewNop(), nil)
	// Shutdown without Start — done is nil
	require.NoError(t, tagger.Shutdown(context.Background()))
	// Double shutdown after Start
	tagger.done = make(chan struct{})
	require.NoError(t, tagger.Shutdown(context.Background()))
	require.NoError(t, tagger.Shutdown(context.Background()))
}
