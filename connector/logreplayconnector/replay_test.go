// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logreplayconnector

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap/zapcore"
)

// TestReplayTimer checks that Start does not send, that the first tick
// comes after the startup delay plus the jitter, and that each later tick
// comes one interval after the one before, each with a copy that carries
// the time of its tick.
func TestReplayTimer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := context.Background()
		sink := new(consumertest.LogsSink)
		conn, _ := newTestConnector(t, testConfig(), sink)
		conn.now = time.Now
		var jitterArg time.Duration
		conn.jitter = func(n time.Duration) time.Duration {
			jitterArg = n
			return 3 * time.Second
		}
		start := time.Now()
		require.NoError(t, conn.Start(ctx, componenttest.NewNopHost()))
		assert.Equal(t, maxStartupJitter, jitterArg, "the jitter is at most 10 s")
		ld := podLogs("p1", base, "app started version=1")
		require.NoError(t, conn.ConsumeLogs(ctx, ld))

		first := start.Add(startupReplayDelay + 3*time.Second)
		time.Sleep(time.Until(first) - time.Nanosecond)
		synctest.Wait()
		assert.Empty(t, sink.AllLogs(), "nothing before the first tick")
		time.Sleep(time.Nanosecond)
		synctest.Wait()
		require.Len(t, sink.AllLogs(), 1)
		requireLogsEqual(t, copyOf(ld, 0, 0, first), sink.AllLogs()[0])

		time.Sleep(time.Hour - time.Nanosecond)
		synctest.Wait()
		assert.Len(t, sink.AllLogs(), 1, "nothing before the next interval")
		time.Sleep(time.Nanosecond)
		synctest.Wait()
		require.Len(t, sink.AllLogs(), 2)
		requireLogsEqual(t, copyOf(ld, 0, 0, first.Add(time.Hour)), sink.AllLogs()[1])

		require.NoError(t, conn.Shutdown(ctx))
		time.Sleep(2 * time.Hour)
		synctest.Wait()
		assert.Len(t, sink.AllLogs(), 2, "nothing after Shutdown")
	})
}

func TestReplayJitterBound(t *testing.T) {
	cfg := testConfig()
	cfg.Interval = time.Minute
	conn, _ := newTestConnector(t, cfg, consumertest.NewNop())
	var got time.Duration
	conn.jitter = func(n time.Duration) time.Duration { got = n; return 0 }
	conn.startReplaying()
	conn.cancelReplaying()
	require.NoError(t, conn.waitReplaying(t.Context()))
	assert.Equal(t, 10*time.Second, got)
}

func TestRandomJitter(t *testing.T) {
	for range 100 {
		j := randomJitter(maxStartupJitter)
		assert.GreaterOrEqual(t, j, time.Duration(0))
		assert.Less(t, j, maxStartupJitter)
	}
}

// TestReplayExactCopies checks that one tick sends every kept record in one
// call, sorted by rule and key, and that each copy equals the original
// record with its resource and scope, except the two times.
func TestReplayExactCopies(t *testing.T) {
	cfg := testConfig()
	cfg.Rules = append(cfg.Rules, RuleConfig{Name: "app_ready", BodyContains: []string{"app ready"}, Identity: []string{"k8s.pod.uid"}})
	sink := new(consumertest.LogsSink)
	conn, _ := newTestConnector(t, cfg, sink)
	ld := plog.NewLogs()
	sl2 := addResource(ld, podResource("p2"))
	addRecord(sl2, base, "app started version=2", podFile("p2"))
	sl1 := addResource(ld, podResource("p1"))
	addRecord(sl1, base, "noise", podFile("p1"))
	lr := addRecord(sl1, base, "app started version=1", podFile("p1"))
	lr.SetSeverityNumber(plog.SeverityNumberWarn)
	lr.SetEventName("app.start")
	lr.SetFlags(plog.DefaultLogRecordFlags.WithIsSampled(true))
	lr.Attributes().PutInt("pid", 7)
	lr.Attributes().PutEmptySlice("tags").AppendEmpty().SetStr("a")
	addRecord(sl1, base, "app ready", podFile("p1"))
	require.NoError(t, conn.ConsumeLogs(t.Context(), ld))

	conn.tick(t.Context())
	require.Len(t, sink.AllLogs(), 1, "one call per tick")
	want := mergeLogs(
		// app_start, by key: p1 before p2.
		copyOf(ld, 1, 1, testNow),
		copyOf(ld, 0, 0, testNow),
		// app_ready.
		copyOf(ld, 1, 2, testNow),
	)
	requireLogsEqual(t, want, sink.AllLogs()[0])

	// The kept records are unchanged, so the next tick sends the same copies.
	conn.tick(t.Context())
	require.Len(t, sink.AllLogs(), 2)
	requireLogsEqual(t, want, sink.AllLogs()[1])
}

func TestReplayEmptyState(t *testing.T) {
	sink := new(consumertest.LogsSink)
	conn, _ := newTestConnector(t, testConfig(), sink)
	conn.tick(t.Context())
	assert.Empty(t, sink.AllLogs())
}

func TestReplaySourceGone(t *testing.T) {
	sink := new(consumertest.LogsSink)
	conn, logs := newTestConnector(t, testConfig(), sink)
	files := newFakeFiles()
	conn.checkSource = files.stat
	changes := 0
	conn.changed = func() { changes++ }
	ld := mergeLogs(podLogs("p1", base, "app started"), podLogs("p2", base, "app started"))
	require.NoError(t, conn.ConsumeLogs(t.Context(), ld))
	changes = 0

	files.remove(podFile("p1"))
	conn.tick(t.Context())
	require.Len(t, sink.AllLogs(), 1)
	requireLogsEqual(t, copyOf(ld, 1, 0, testNow), sink.AllLogs()[0])
	assert.Equal(t, []string{podKey("p2")}, keys(conn.state, 0), "the entry of the deleted pod is removed")
	assert.Equal(t, 2, changes, "the removal and the time of the copies")
	removed := logs.FilterMessage("Removed kept log records").All()
	require.Len(t, removed, 1)
	assert.Equal(t, zapcore.DebugLevel, removed[0].Level)
	assert.Equal(t, int64(1), removed[0].ContextMap()["source_gone"])

	// The removed entry is not checked again.
	files.remove(podFile("p2"))
	conn.tick(t.Context())
	assert.Len(t, sink.AllLogs(), 1, "nothing left to send")
	assert.Equal(t, 2, files.count(podFile("p1")), "a missing file is checked twice")
	assert.Empty(t, conn.state.entries())
}

func TestReplaySourceCheckError(t *testing.T) {
	sink := new(consumertest.LogsSink)
	conn, logs := newTestConnector(t, testConfig(), sink)
	files := newFakeFiles()
	conn.checkSource = files.stat
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("p1", base, "app started")))

	files.setErr(podFile("p1"), &fs.PathError{Op: "stat", Path: podFile("p1"), Err: fs.ErrPermission})
	conn.tick(t.Context())
	conn.tick(t.Context())
	assert.Empty(t, sink.AllLogs(), "a record whose source cannot be checked is not sent")
	assert.Len(t, conn.state.entries(), 1, "and it stays kept")
	warns := logs.FilterMessage("Kept log record not sent: its source file cannot be checked").FilterLevelExact(zapcore.WarnLevel).All()
	require.Len(t, warns, 1, "one Warn per minute")
	assert.Equal(t, podFile("p1"), warns[0].ContextMap()["source_file"])

	files.setErr(podFile("p1"), nil)
	conn.tick(t.Context())
	assert.Len(t, sink.AllLogs(), 1)
}

func TestRemoveGoneKeepsReplacedEntry(t *testing.T) {
	conn, _ := newTestConnector(t, testConfig(), consumertest.NewNop())
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("p1", base, "app started version=1")))
	checked := conn.state.entries()
	// A new pod with the same name logs its startup line during the check.
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("p1", base.Add(time.Minute), "app started version=2")))
	assert.Empty(t, conn.state.removeGone(checked))
	assert.Len(t, conn.state.entries(), 1)
}

// errConsumer returns err from every call.
type errConsumer struct {
	consumertest.LogsSink
	err error
}

func (c *errConsumer) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	_ = c.LogsSink.ConsumeLogs(ctx, ld)
	return c.err
}

func TestReplayDownstreamError(t *testing.T) {
	next := &errConsumer{err: errors.New("throttled")}
	conn, logs := newTestConnector(t, testConfig(), next)
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("p1", base, "app started")))
	conn.tick(t.Context())
	conn.tick(t.Context())
	assert.Len(t, next.AllLogs(), 2)
	warns := logs.FilterMessage("Failed to send the log record copies to the next consumer").All()
	require.Len(t, warns, 1, "one Warn per minute")
	assert.Equal(t, "throttled", warns[0].ContextMap()["error"])
	assert.Len(t, conn.state.entries(), 1, "the record stays kept for the next tick")
}

// blockingConsumer blocks each call until its context ends.
type blockingConsumer struct {
	entered chan struct{}
}

func (c *blockingConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (c *blockingConsumer) ConsumeLogs(ctx context.Context, _ plog.Logs) error {
	close(c.entered)
	<-ctx.Done()
	return ctx.Err()
}

func TestShutdownCancelsTickInProgress(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := context.Background()
		next := &blockingConsumer{entered: make(chan struct{})}
		conn, logs := newTestConnector(t, testConfig(), next)
		require.NoError(t, conn.Start(ctx, componenttest.NewNopHost()))
		require.NoError(t, conn.ConsumeLogs(ctx, podLogs("p1", base, "app started")))
		time.Sleep(startupReplayDelay)
		<-next.entered
		require.NoError(t, conn.Shutdown(ctx))
		assert.Equal(t, 1, logs.FilterMessage("Log record copies not sent: the replay was canceled").Len())
		assert.Zero(t, logs.FilterMessage("Failed to send the log record copies to the next consumer").Len())
	})
}

func TestShutdownWaitIsBounded(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		conn, _ := newTestConnector(t, testConfig(), consumertest.NewNop())
		// A replay goroutine that never stops.
		conn.replayDone = make(chan struct{})
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.ErrorIs(t, conn.Shutdown(ctx), context.DeadlineExceeded)
	})
}

func TestShutdownWithoutStart(t *testing.T) {
	conn, _ := newTestConnector(t, testConfig(), consumertest.NewNop())
	require.NoError(t, conn.Shutdown(t.Context()))
	require.NoError(t, conn.Shutdown(t.Context()))
}

func TestSetTimes(t *testing.T) {
	ld := mergeLogs(podLogs("p1", base, "a", "b"), podLogs("p2", base, "c"))
	setTimes(ld, 42)
	n := 0
	for _, rl := range ld.ResourceLogs().All() {
		for _, sl := range rl.ScopeLogs().All() {
			for _, lr := range sl.LogRecords().All() {
				assert.EqualValues(t, 42, lr.Timestamp())
				assert.EqualValues(t, 42, lr.ObservedTimestamp())
				n++
			}
		}
	}
	assert.Equal(t, 3, n)
}

// TestLstatFile checks the source check on real files: a symbolic link
// whose target is missing still exists, as during a kubelet log rotation,
// and a removed link does not.
func TestLstatFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "0.log")
	link := filepath.Join(dir, "pod_ns_app-1.log")
	require.NoError(t, os.WriteFile(target, []byte("x\n"), 0o600))
	if err := os.Symlink(target, link); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symbolic links need a privilege on Windows: %v", err)
		}
		require.NoError(t, err)
	}
	assert.NoError(t, lstatFile(link))
	require.NoError(t, os.Remove(target))
	assert.NoError(t, lstatFile(link), "a dangling link still exists")
	require.NoError(t, os.Remove(link))
	assert.ErrorIs(t, lstatFile(link), fs.ErrNotExist)
	assert.NoError(t, lstatFile(dir))
}

// TestReplaySourceRotation checks that a source file that is missing at the
// first check and back at the second, as during a log rotation that
// renames the file and then creates a new one, keeps its record.
func TestReplaySourceRotation(t *testing.T) {
	sink := new(consumertest.LogsSink)
	conn, _ := newTestConnector(t, testConfig(), sink)
	calls := 0
	conn.checkSource = func(path string) error {
		calls++
		if calls == 1 {
			return &fs.PathError{Op: "lstat", Path: path, Err: fs.ErrNotExist}
		}
		return nil
	}
	ld := podLogs("p1", base, "app started")
	require.NoError(t, conn.ConsumeLogs(t.Context(), ld))
	conn.tick(t.Context())
	assert.Equal(t, 2, calls)
	assert.Len(t, conn.state.entries(), 1)
	require.Len(t, sink.AllLogs(), 1)
	requireLogsEqual(t, copyOf(ld, 0, 0, testNow), sink.AllLogs()[0])
}

// TestReplayRecheckWaits checks that the second check of a missing file
// comes sourceRecheckDelay after the first, and that a shutdown during the
// wait removes nothing.
func TestReplayRecheckWaits(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		conn, _ := newTestConnector(t, testConfig(), new(consumertest.LogsSink))
		conn.recheckDelay = sourceRecheckDelay
		var checks []time.Time
		conn.checkSource = func(path string) error {
			checks = append(checks, time.Now())
			return &fs.PathError{Op: "lstat", Path: path, Err: fs.ErrNotExist}
		}
		require.NoError(t, conn.ConsumeLogs(context.Background(), podLogs("p1", base, "app started")))
		conn.tick(context.Background())
		require.Len(t, checks, 2)
		assert.Equal(t, sourceRecheckDelay, checks[1].Sub(checks[0]))
		assert.Empty(t, conn.state.entries())

		require.NoError(t, conn.ConsumeLogs(context.Background(), podLogs("p2", base, "app started")))
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(sourceRecheckDelay / 2)
			cancel()
		}()
		conn.tick(ctx)
		assert.Len(t, conn.state.entries(), 1, "a canceled wait removes nothing")
	})
}

// TestReplayChunks checks that one tick sends at most maxSendBytes of kept
// records per call, in order, and every record one time.
func TestReplayChunks(t *testing.T) {
	sink := new(consumertest.LogsSink)
	conn, _ := newTestConnector(t, testConfig(), sink)
	pad := strings.Repeat("x", 60<<10)
	var lds []plog.Logs
	for i := range 40 {
		lds = append(lds, podLogs(fmt.Sprintf("p%02d", i), base, "app started "+pad))
	}
	require.NoError(t, conn.ConsumeLogs(t.Context(), mergeLogs(lds...)))
	kept := conn.state.entries()
	require.Len(t, kept, 40)
	conn.tick(t.Context())

	calls := sink.AllLogs()
	require.Greater(t, len(calls), 1)
	var pods []string
	for i, ld := range calls {
		size := 0
		for _, rl := range ld.ResourceLogs().All() {
			pod, _ := rl.Resource().Attributes().Get("k8s.pod.name")
			pods = append(pods, pod.Str())
		}
		for _, k := range kept[len(pods)-ld.ResourceLogs().Len() : len(pods)] {
			size += len(k.entry.Record)
		}
		assert.LessOrEqual(t, size, maxSendBytes, "call %d", i)
	}
	want := make([]string, 40)
	for i := range want {
		want[i] = fmt.Sprintf("p%02d", i)
	}
	assert.Equal(t, want, pods, "every record one time, in state order")
}

func TestFirstDelay(t *testing.T) {
	conn, _ := newTestConnector(t, testConfig(), consumertest.NewNop())
	conn.jitter = func(time.Duration) time.Duration { return 3 * time.Second }
	startup := startupReplayDelay + 3*time.Second
	for _, tc := range []struct {
		name string
		last time.Time
		want time.Duration
	}{
		{name: "no_copies_yet", want: startup},
		{name: "copies_5m_ago", last: testNow.Add(-5 * time.Minute), want: 55 * time.Minute},
		{name: "due_in_the_startup_delay", last: testNow.Add(-time.Hour + time.Second), want: startup},
		{name: "copies_2h_ago", last: testNow.Add(-2 * time.Hour), want: startup},
		{name: "copies_ahead_of_the_clock", last: testNow.Add(3 * time.Hour), want: time.Hour},
	} {
		t.Run(tc.name, func(t *testing.T) {
			last := int64(0)
			if !tc.last.IsZero() {
				last = tc.last.UnixNano()
			}
			conn.state.setLastReplay(last)
			assert.Equal(t, tc.want, conn.firstDelay())
		})
	}
}
