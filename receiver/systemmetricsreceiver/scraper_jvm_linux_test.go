//go:build linux

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package systemmetricsreceiver

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func TestJVMScraperName(t *testing.T) {
	s := newJVMScraper(zap.NewNop())
	assert.Equal(t, "jvm", s.Name())
}

func TestJVMScraperNoSocketsIsNoop(t *testing.T) {
	s := newJVMScraper(zap.NewNop())
	metrics := pmetric.NewMetrics()
	err := s.Scrape(context.Background(), metrics)
	require.NoError(t, err)
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())
}

func TestParseHeap(t *testing.T) {
	tests := map[string]struct {
		input         []byte
		pid           string
		wantNil       bool
		wantMax       float64
		wantCommitted float64
		wantUsed      float64
	}{
		"full prometheus output with HELP/TYPE": {
			input: []byte(`# HELP jvm_heap_max_bytes Maximum heap size (Xmx)
# TYPE jvm_heap_max_bytes gauge
jvm_heap_max_bytes 536870912
# HELP jvm_heap_committed_bytes Committed heap size
# TYPE jvm_heap_committed_bytes gauge
jvm_heap_committed_bytes 402653184
# HELP jvm_heap_after_gc_bytes Heap memory used after GC
# TYPE jvm_heap_after_gc_bytes gauge
jvm_heap_after_gc_bytes 157286400
# HELP jvm_gc_count_total Garbage collection count
# TYPE jvm_gc_count_total counter
jvm_gc_count_total 42
# HELP jvm_allocated_bytes Total allocated bytes
# TYPE jvm_allocated_bytes counter
jvm_allocated_bytes 8589934592
`),
			pid: "1234", wantMax: 536870912, wantCommitted: 402653184, wantUsed: 157286400,
		},
		"partial metrics (committed only)": {
			input: []byte("# TYPE jvm_heap_committed_bytes gauge\njvm_heap_committed_bytes 400000000\n"),
			pid:   "1", wantMax: -1, wantCommitted: 400000000, wantUsed: -1,
		},
		"bare lines without TYPE/HELP": {
			input: []byte("jvm_heap_max_bytes 536870912\njvm_heap_committed_bytes 402653184\njvm_heap_after_gc_bytes 157286400\n"),
			pid:   "99", wantMax: 536870912, wantCommitted: 402653184, wantUsed: 157286400,
		},
		"empty input": {
			input: []byte(""), pid: "1", wantNil: true,
		},
		"malformed input (no valid metrics)": {
			input: []byte("not valid {{{"), pid: "1", wantNil: true,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			s := newJVMScraper(zap.NewNop())
			hd, err := s.parseHeap(tc.input, tc.pid)
			require.NoError(t, err)
			if tc.wantNil {
				assert.Nil(t, hd)
			} else {
				require.NotNil(t, hd)
				assert.Equal(t, tc.pid, hd.pid)
				assert.Equal(t, tc.wantMax, hd.maxBytes)
				assert.Equal(t, tc.wantCommitted, hd.committedBytes)
				assert.Equal(t, tc.wantUsed, hd.usedBytes)
			}
		})
	}
}

func TestParseMetricsTextSkipsNaNInf(t *testing.T) {
	metricsText := []byte(`jvm_heap_max_bytes 536870912
jvm_heap_committed_bytes NaN
jvm_heap_after_gc_bytes +Inf
jvm_gc_count_total -Inf
jvm_allocated_bytes 1024
`)
	m := parseMetricsText(metricsText)
	assert.Equal(t, 536870912.0, m["jvm_heap_max_bytes"])
	assert.Equal(t, 1024.0, m["jvm_allocated_bytes"])
	assert.NotContains(t, m, "jvm_heap_committed_bytes")
	assert.NotContains(t, m, "jvm_heap_after_gc_bytes")
	assert.NotContains(t, m, "jvm_gc_count_total")
}

func TestEmitPerJVM(t *testing.T) {
	s := newJVMScraper(zap.NewNop())
	metrics := pmetric.NewMetrics()
	now := pcommon.Timestamp(0)

	hd := heapData{pid: "42", maxBytes: 536870912, committedBytes: 402653184, usedBytes: 157286400}
	s.emitPerJVM(metrics, hd, now)

	require.Equal(t, 1, metrics.ResourceMetrics().Len())
	rm := metrics.ResourceMetrics().At(0)

	sm := rm.ScopeMetrics().At(0)
	assert.Equal(t, 4, sm.Metrics().Len())

	// heap_max_bytes = 536870912 (raw bytes, from jvm_heap_max_bytes)
	m0 := sm.Metrics().At(0)
	assert.Equal(t, "heap_max_bytes", m0.Name())
	assert.Equal(t, "Bytes", m0.Unit())
	assert.InDelta(t, 536870912.0, m0.Gauge().DataPoints().At(0).DoubleValue(), 0.01)

	// heap_committed_bytes = 402653184 (raw bytes, from jvm_heap_committed_bytes)
	m1 := sm.Metrics().At(1)
	assert.Equal(t, "heap_committed_bytes", m1.Name())
	assert.Equal(t, "Bytes", m1.Unit())
	assert.InDelta(t, 402653184.0, m1.Gauge().DataPoints().At(0).DoubleValue(), 0.01)

	// heap_after_gc_bytes = 157286400 (raw bytes)
	m2 := sm.Metrics().At(2)
	assert.Equal(t, "heap_after_gc_bytes", m2.Name())
	assert.Equal(t, "Bytes", m2.Unit())
	assert.InDelta(t, 157286400.0, m2.Gauge().DataPoints().At(0).DoubleValue(), 0.01)

	// heap_free_after_gc_bytes = 536870912 - 157286400 = 379584512 (max - afterGC)
	m3 := sm.Metrics().At(3)
	assert.Equal(t, "heap_free_after_gc_bytes", m3.Name())
	assert.Equal(t, "Bytes", m3.Unit())
	assert.InDelta(t, 379584512.0, m3.Gauge().DataPoints().At(0).DoubleValue(), 0.01)
}

func TestEmitAggregate(t *testing.T) {
	s := newJVMScraper(zap.NewNop())
	metrics := pmetric.NewMetrics()
	now := pcommon.Timestamp(0)

	allHeap := []heapData{
		{pid: "1", maxBytes: 1048576000, usedBytes: 524288000}, // 50% utilized
		{pid: "2", maxBytes: 1048576000, usedBytes: 786432000}, // 75% utilized
	}
	s.emitAggregate(metrics, allHeap, now)

	require.Equal(t, 1, metrics.ResourceMetrics().Len())
	sm := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0)
	require.Equal(t, 4, sm.Metrics().Len())

	// aggregate_jvm_count = 2
	m0 := sm.Metrics().At(0)
	assert.Equal(t, "aggregate_jvm_count", m0.Name())
	assert.Equal(t, "Count", m0.Unit())
	assert.InDelta(t, 2.0, m0.Gauge().DataPoints().At(0).DoubleValue(), 0.01)

	// aggregate_heap_max_bytes = 1048576000 + 1048576000 = 2097152000
	m1 := sm.Metrics().At(1)
	assert.Equal(t, "aggregate_heap_max_bytes", m1.Name())
	assert.Equal(t, "Bytes", m1.Unit())
	assert.InDelta(t, 2097152000.0, m1.Gauge().DataPoints().At(0).DoubleValue(), 0.01)

	// aggregate_heap_free_after_gc_bytes = (1048576000-524288000) + (1048576000-786432000) = 786432000
	m2 := sm.Metrics().At(2)
	assert.Equal(t, "aggregate_heap_free_after_gc_bytes", m2.Name())
	assert.Equal(t, "Bytes", m2.Unit())
	assert.InDelta(t, 786432000.0, m2.Gauge().DataPoints().At(0).DoubleValue(), 0.01)

	// aggregate_heap_after_gc_utilized = (524288000 + 786432000) / (1048576000 + 1048576000) * 100 = 62.5%
	m3 := sm.Metrics().At(3)
	assert.Equal(t, "aggregate_heap_after_gc_utilized", m3.Name())
	assert.Equal(t, "Percent", m3.Unit())
	assert.InDelta(t, 62.5, m3.Gauge().DataPoints().At(0).DoubleValue(), 0.01)
}

func TestEmitAggregateMaxOnly(t *testing.T) {
	s := newJVMScraper(zap.NewNop())
	metrics := pmetric.NewMetrics()
	now := pcommon.Timestamp(0)

	// JVM with max but no after-GC data — should still contribute to count and aggregate max
	allHeap := []heapData{
		{pid: "1", maxBytes: 1048576000, usedBytes: -1},
	}
	s.emitAggregate(metrics, allHeap, now)

	require.Equal(t, 1, metrics.ResourceMetrics().Len())
	sm := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0)
	require.Equal(t, 2, sm.Metrics().Len()) // count + max only, no free/utilized

	assert.Equal(t, "aggregate_jvm_count", sm.Metrics().At(0).Name())
	assert.InDelta(t, 1.0, sm.Metrics().At(0).Gauge().DataPoints().At(0).DoubleValue(), 0.01)

	assert.Equal(t, "aggregate_heap_max_bytes", sm.Metrics().At(1).Name())
	assert.InDelta(t, 1048576000.0, sm.Metrics().At(1).Gauge().DataPoints().At(0).DoubleValue(), 0.01)
}

func TestEmitAggregateEmpty(t *testing.T) {
	s := newJVMScraper(zap.NewNop())
	metrics := pmetric.NewMetrics()
	now := pcommon.Timestamp(0)

	s.emitAggregate(metrics, nil, now)
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())
}

func TestDiscoverSockets(t *testing.T) {
	header := "Num       RefCount Protocol Flags    Type St Inode Path\n"
	tests := map[string]struct {
		content  string
		usePath  string // if set, use this path instead of a temp file
		wantPIDs []string
	}{
		"two matching sockets": {
			content:  header + "0000000000000000: 00000002 00000000 00010000 0002 01 12345 @aws-jvm-metrics-1234\n" + "0000000000000000: 00000002 00000000 00010000 0002 01 12346 @aws-jvm-metrics-5678\n",
			wantPIDs: []string{"1234", "5678"},
		},
		"skips non-DGRAM (SOCK_STREAM 0001)": {
			content:  header + "0000000000000000: 00000002 00000000 00010000 0001 01 12345 @aws-jvm-metrics-1234\n",
			wantPIDs: nil,
		},
		"skips non-JVM prefix": {
			content:  header + "0000000000000000: 00000002 00000000 00010000 0002 01 12345 @some-other-socket\n",
			wantPIDs: nil,
		},
		"skips invalid PID (non-digits)": {
			content:  header + "0000000000000000: 00000002 00000000 00010000 0002 01 12345 @aws-jvm-metrics-abc\n" + "0000000000000000: 00000002 00000000 00010000 0002 01 12346 @aws-jvm-metrics-12-34\n",
			wantPIDs: nil,
		},
		"header only (no sockets)": {
			content:  header,
			wantPIDs: nil,
		},
		"missing file returns nil": {
			usePath:  "/nonexistent/proc/net/unix",
			wantPIDs: nil,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			s := newJVMScraper(zap.NewNop())
			if tc.usePath != "" {
				s.procNetUnixPath = tc.usePath
			} else {
				s.procNetUnixPath = writeTempFile(t, tc.content)
			}
			sockets := s.discoverSockets()
			if tc.wantPIDs == nil {
				assert.Empty(t, sockets)
			} else {
				require.Len(t, sockets, len(tc.wantPIDs))
				for i, pid := range tc.wantPIDs {
					assert.Equal(t, pid, sockets[i].pid)
				}
			}
		})
	}
}

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "proc_net_unix")
	require.NoError(t, err)
	_, err = f.WriteString(content)
	require.NoError(t, err)
	f.Close()
	return f.Name()
}
