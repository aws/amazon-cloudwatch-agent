// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logreplayconnector

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap/zapcore"
)

func TestConsumeLogsKeepsAndDoesNotSend(t *testing.T) {
	sink := new(consumertest.LogsSink)
	conn, _ := newTestConnector(t, testConfig(), sink)
	ld := podLogs("p1", base, "request served", "app started version=1", "request served")
	require.NoError(t, conn.ConsumeLogs(t.Context(), ld))

	assert.Empty(t, sink.AllLogs(), "ConsumeLogs never sends")
	kept := conn.state.entries()
	require.Len(t, kept, 1)
	assert.Equal(t, podKey("p1"), kept[0].key)
	assert.Equal(t, []kv{{"k8s.namespace.name", "ns"}, {"k8s.pod.name", "p1"}, {"k8s.container.name", "app"}}, kept[0].entry.Identity)
	assert.Equal(t, podFile("p1"), kept[0].entry.SourceFile)
	assert.Equal(t, base.UnixNano(), kept[0].entry.Time)

	got, err := decodeRecord(kept[0].entry.Record)
	require.NoError(t, err)
	// The kept record is the original with its resource and scope.
	want := copyOf(ld, 0, 1, base)
	want.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).SetObservedTimestamp(pcommon.NewTimestampFromTime(base.Add(time.Second)))
	requireLogsEqual(t, want, got)
}

func TestConsumeLogsDoesNotMutateInput(t *testing.T) {
	conn, _ := newTestConnector(t, testConfig(), consumertest.NewNop())
	ld := podLogs("p1", base, "app started version=1")
	before := plog.NewLogs()
	ld.CopyTo(before)
	require.NoError(t, conn.ConsumeLogs(t.Context(), ld))
	requireLogsEqual(t, before, ld)
	assert.False(t, conn.Capabilities().MutatesData)
}

func TestConsumeLogsEntities(t *testing.T) {
	conn, _ := newTestConnector(t, testConfig(), consumertest.NewNop())
	ld := podLogs("p1", base, "app started version=1")
	// A second container of the same pod is another entity.
	other := podResource("p1")
	other["k8s.container.name"] = "sidecar"
	addRecord(addResource(ld, other), base, "app started version=9", "/var/log/containers/p1_ns_sidecar-1.log")
	addRecord(addResource(ld, podResource("p2")), base, "app started version=2", podFile("p2"))
	require.NoError(t, conn.ConsumeLogs(t.Context(), ld))

	sidecarKey := encodeIdentity([]kv{{"k8s.namespace.name", "ns"}, {"k8s.pod.name", "p1"}, {"k8s.container.name", "sidecar"}})
	assert.ElementsMatch(t, []string{podKey("p1"), podKey("p2"), sidecarKey}, keys(conn.state, 0))
}

func TestConsumeLogsReplacesWithNewer(t *testing.T) {
	conn, _ := newTestConnector(t, testConfig(), consumertest.NewNop())
	ctx := t.Context()
	changes := 0
	conn.changed = func() { changes++ }

	require.NoError(t, conn.ConsumeLogs(ctx, podLogs("p1", base, "app started version=1")))
	assert.Equal(t, 1, changes)

	// The same record again changes nothing.
	require.NoError(t, conn.ConsumeLogs(ctx, podLogs("p1", base, "app started version=1")))
	assert.Equal(t, 1, changes)

	// An older record does not replace the entry.
	require.NoError(t, conn.ConsumeLogs(ctx, podLogs("p1", base.Add(-time.Minute), "app started version=0")))
	assert.Equal(t, 1, changes)
	assert.Contains(t, string(conn.state.entries()[0].entry.Record), "version=1")

	// A newer record replaces it, for example after a container restart.
	require.NoError(t, conn.ConsumeLogs(ctx, podLogs("p1", base.Add(time.Hour), "app started version=2")))
	assert.Equal(t, 2, changes)
	kept := conn.state.entries()
	require.Len(t, kept, 1)
	assert.Contains(t, string(kept[0].entry.Record), "version=2")
	assert.Equal(t, base.Add(time.Hour).UnixNano(), kept[0].entry.Time)

	// Within one batch, the last record of the same time wins.
	require.NoError(t, conn.ConsumeLogs(ctx, podLogs("p1", base.Add(2*time.Hour), "app started version=3", "app started version=4")))
	assert.Contains(t, string(conn.state.entries()[0].entry.Record), "version=4")
}

func TestConsumeLogsRecordTime(t *testing.T) {
	conn, _ := newTestConnector(t, testConfig(), consumertest.NewNop())
	ctx := t.Context()

	// No timestamp: the observed timestamp.
	ld := plog.NewLogs()
	lr := addRecord(addResource(ld, podResource("p1")), time.Time{}, "app started", podFile("p1"))
	lr.SetObservedTimestamp(pcommon.NewTimestampFromTime(base))
	// No time at all: now.
	addRecord(addResource(ld, podResource("p2")), time.Time{}, "app started", podFile("p2"))
	// A time ahead of now: now.
	addRecord(addResource(ld, podResource("p3")), testNow.Add(time.Hour), "app started", podFile("p3"))
	require.NoError(t, conn.ConsumeLogs(ctx, ld))

	times := map[string]int64{}
	for _, k := range conn.state.entries() {
		times[k.entry.Identity[1].Value] = k.entry.Time
	}
	assert.Equal(t, map[string]int64{"p1": base.UnixNano(), "p2": testNow.UnixNano(), "p3": testNow.UnixNano()}, times)
}

func TestConsumeLogsNotUsed(t *testing.T) {
	cfg := testConfig()
	cfg.MaxRecordBytes = minMaxRecordBytes
	conn, logs := newTestConnector(t, cfg, consumertest.NewNop())
	ld := plog.NewLogs()
	// No container name.
	noContainer := podResource("p1")
	delete(noContainer, "k8s.container.name")
	addRecord(addResource(ld, noContainer), base, "app started", podFile("p1"))
	// An empty pod name.
	emptyPod := podResource("p2")
	emptyPod["k8s.pod.name"] = ""
	addRecord(addResource(ld, emptyPod), base, "app started", podFile("p2"))
	// No source file.
	addRecord(addResource(ld, podResource("p3")), base, "app started", "")
	// Larger than max_record_bytes.
	addRecord(addResource(ld, podResource("p4")), base, "app started "+strings.Repeat("x", minMaxRecordBytes), podFile("p4"))
	// A body that is not a string.
	lr := addRecord(addResource(ld, podResource("p5")), base, "", podFile("p5"))
	lr.Body().SetEmptyMap().PutStr("msg", "app started")
	require.NoError(t, conn.ConsumeLogs(t.Context(), ld))

	assert.Empty(t, conn.state.entries())
	assert.Len(t, logs.FilterMessage("Matching log record not kept: an identity attribute is missing or empty").All(), 2)
	assert.Len(t, logs.FilterMessage("Matching log record not kept: the source file attribute is missing or empty").All(), 1)
	tooLarge := logs.FilterMessage("Matching log record not kept: it is larger than max_record_bytes").All()
	require.Len(t, tooLarge, 1)
	assert.Equal(t, zapcore.WarnLevel, tooLarge[0].Level)
}

func TestConsumeLogsConditions(t *testing.T) {
	cfg := testConfig()
	cfg.Rules[0].Conditions = []string{`attributes["log.iostream"] == "stderr"`}
	conn, _ := newTestConnector(t, cfg, consumertest.NewNop())
	ld := podLogs("p1", base, "app started")
	sl := addResource(ld, podResource("p2"))
	addRecord(sl, base, "app started", podFile("p2")).Attributes().PutStr("log.iostream", "stderr")
	// The conditions alone, with no body_contains.
	require.NoError(t, conn.ConsumeLogs(t.Context(), ld))
	assert.Equal(t, []string{podKey("p2")}, keys(conn.state, 0))

	cfg = testConfig()
	cfg.Rules[0].BodyContains = nil
	cfg.Rules[0].Conditions = []string{`IsMatch(body, "^app started")`}
	conn, _ = newTestConnector(t, cfg, consumertest.NewNop())
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("p3", base, "request", "app started")))
	assert.Equal(t, []string{podKey("p3")}, keys(conn.state, 0))
}

func TestConsumeLogsErrorModes(t *testing.T) {
	// The condition fails to evaluate on a record without the attribute.
	condition := `ExtractPatterns(log.body, "level=(?P<v>\\S+)")["v"] == "info"`
	for _, mode := range []ottl.ErrorMode{ottl.IgnoreError, ottl.SilentError, ottl.PropagateError} {
		t.Run(string(mode), func(t *testing.T) {
			cfg := testConfig()
			cfg.ErrorMode = mode
			cfg.Rules[0].Conditions = []string{condition}
			conn, _ := newTestConnector(t, cfg, consumertest.NewNop())
			ld := podLogs("p1", base, "app started")
			err := conn.ConsumeLogs(t.Context(), ld)
			if mode == ottl.PropagateError {
				require.ErrorContains(t, err, `rule "app_start"`)
			} else {
				require.NoError(t, err)
			}
			assert.Empty(t, conn.state.entries())
		})
	}
}

func TestConsumeLogsPropagateKeepsEarlierRecords(t *testing.T) {
	cfg := testConfig()
	cfg.ErrorMode = ottl.PropagateError
	cfg.Rules[0].Conditions = []string{`ExtractPatterns(log.body, "level=(?P<v>\\S+)")["v"] == "info"`}
	conn, _ := newTestConnector(t, cfg, consumertest.NewNop())
	ld := plog.NewLogs()
	addRecord(addResource(ld, podResource("p1")), base, "app started level=info", podFile("p1"))
	addRecord(addResource(ld, podResource("p2")), base, "app started", podFile("p2"))
	addRecord(addResource(ld, podResource("p3")), base, "app started level=info", podFile("p3"))
	require.Error(t, conn.ConsumeLogs(t.Context(), ld))
	assert.Equal(t, []string{podKey("p1")}, keys(conn.state, 0), "the record before the error is kept, the ones after it are not scanned")
}

func TestConsumeLogsTwoRules(t *testing.T) {
	cfg := testConfig()
	cfg.Rules = append(cfg.Rules, RuleConfig{Name: "app_ready", BodyContains: []string{"app ready"}, Identity: []string{"k8s.pod.uid"}})
	conn, _ := newTestConnector(t, cfg, consumertest.NewNop())
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("p1", base, "app started", "app ready", "app started and app ready")))
	assert.Equal(t, []string{podKey("p1")}, keys(conn.state, 0))
	assert.Equal(t, []string{encodeIdentity([]kv{{"k8s.pod.uid", "uid-p1"}})}, keys(conn.state, 1))
	for _, k := range conn.state.entries() {
		assert.Contains(t, string(k.entry.Record), "app started and app ready", "the last matching record of each rule is kept")
	}
}

func TestConsumeLogsConcurrent(t *testing.T) {
	conn, _ := newTestConnector(t, testConfig(), consumertest.NewNop())
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			for j := range 50 {
				pod := fmt.Sprintf("p%d", i)
				_ = conn.ConsumeLogs(context.Background(), podLogs(pod, base.Add(time.Duration(j)*time.Second), "app started"))
			}
		})
	}
	wg.Wait()
	assert.Len(t, conn.state.entries(), 8)
	for _, k := range conn.state.entries() {
		assert.Equal(t, base.Add(49*time.Second).UnixNano(), k.entry.Time)
	}
}

func TestConsumeLogsNoMatchDoesNotAllocate(t *testing.T) {
	if raceEnabled {
		t.Skip("the race detector allocates")
	}
	conn, _ := newTestConnector(t, testConfig(), consumertest.NewNop())
	ctx := context.Background()
	ld := podLogs("p1", base, "GET /health 200", "POST /v1/completions 200", "INFO engine step")
	allocs := testing.AllocsPerRun(100, func() {
		_ = conn.ConsumeLogs(ctx, ld)
	})
	assert.Zero(t, allocs)
}

func BenchmarkConsumeLogsNoMatch(b *testing.B) {
	conn, _ := newTestConnector(b, testConfig(), consumertest.NewNop())
	ctx := context.Background()
	ld := podLogs("p1", base, `(APIServer pid=1) INFO:     10.0.3.7:51234 - "POST /v1/chat/completions HTTP/1.1" 200 OK`)
	b.ReportAllocs()
	for b.Loop() {
		_ = conn.ConsumeLogs(ctx, ld)
	}
}

func BenchmarkConsumeLogsMatch(b *testing.B) {
	conn, _ := newTestConnector(b, testConfig(), consumertest.NewNop())
	ctx := context.Background()
	ld := podLogs("p1", base, "app started "+strings.Repeat("k=v, ", 400))
	b.ReportAllocs()
	for b.Loop() {
		_ = conn.ConsumeLogs(ctx, ld)
	}
}

// TestConsumeLogsTooLargeRemovesStale checks that a newer record that is
// too large to keep removes the kept record of its entity, which would show
// a configuration that is gone.
func TestConsumeLogsTooLargeRemovesStale(t *testing.T) {
	cfg := testConfig()
	cfg.MaxRecordBytes = minMaxRecordBytes
	conn, _ := newTestConnector(t, cfg, consumertest.NewNop())
	ctx := t.Context()
	require.NoError(t, conn.ConsumeLogs(ctx, podLogs("p1", base, "app started version=1")))
	huge := "app started version=2 " + strings.Repeat("x", minMaxRecordBytes)
	require.NoError(t, conn.ConsumeLogs(ctx, podLogs("p1", base.Add(-time.Minute), huge)))
	assert.Len(t, conn.state.entries(), 1, "an older record that is too large changes nothing")
	require.NoError(t, conn.ConsumeLogs(ctx, podLogs("p1", base.Add(time.Minute), huge)))
	assert.Empty(t, conn.state.entries())
}
