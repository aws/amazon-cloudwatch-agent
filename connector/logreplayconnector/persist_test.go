// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logreplayconnector

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

var (
	testStorageID   = component.MustNewIDWithName("file_storage", "state")
	testConnectorID = component.MustNewIDWithName(TypeStr, "vllm")
)

type testHost map[component.ID]component.Component

func (h testHost) GetExtensions() map[component.ID]component.Component {
	return h
}

// memClient keeps its data after Close, so that a test can restart a connector on it.
type memClient struct {
	mu       sync.Mutex
	data     map[string][]byte
	sets     int
	closes   int
	getErr   error
	setErr   error
	closeErr error
}

var _ storage.Client = (*memClient)(nil)

func (c *memClient) Get(_ context.Context, key string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.getErr != nil {
		return nil, c.getErr
	}
	v, ok := c.data[key]
	if !ok {
		return nil, nil
	}
	return append([]byte(nil), v...), nil
}

func (c *memClient) Set(_ context.Context, key string, value []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.setErr != nil {
		return c.setErr
	}
	c.sets++
	c.data[key] = append([]byte(nil), value...)
	return nil
}

func (c *memClient) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
	return nil
}

func (c *memClient) Batch(ctx context.Context, ops ...*storage.Operation) error {
	for _, op := range ops {
		var err error
		switch op.Type {
		case storage.Get:
			op.Value, err = c.Get(ctx, op.Key)
		case storage.Set:
			err = c.Set(ctx, op.Key, op.Value)
		case storage.Delete:
			err = c.Delete(ctx, op.Key)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *memClient) Close(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closes++
	return c.closeErr
}

func (c *memClient) put(value []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[stateRecordKey] = value
}

func (c *memClient) setCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sets
}

func (c *memClient) record(t *testing.T) stateSnapshot {
	t.Helper()
	c.mu.Lock()
	b := c.data[stateRecordKey]
	c.mu.Unlock()
	require.NotNil(t, b, "no record stored")
	var r struct {
		Version int           `json:"version"`
		Data    stateSnapshot `json:"data"`
	}
	require.NoError(t, json.Unmarshal(b, &r))
	require.Equal(t, stateRecordVersion, r.Version)
	return r.Data
}

type memStorage struct {
	component.StartFunc
	component.ShutdownFunc
	client *memClient
	err    error

	mu   sync.Mutex
	kind component.Kind
	id   component.ID
}

var _ storage.Extension = (*memStorage)(nil)

func newMemStorage() *memStorage {
	return &memStorage{client: &memClient{data: map[string][]byte{}}}
}

func (m *memStorage) GetClient(_ context.Context, kind component.Kind, id component.ID, _ string) (storage.Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.kind, m.id = kind, id
	if m.err != nil {
		return nil, m.err
	}
	return m.client, nil
}

func (m *memStorage) host() component.Host {
	return testHost{testStorageID: m}
}

func newStorageConnector(t testing.TB, cfg *Config, next consumer.Logs) (*replayConnector, *observer.ObservedLogs) {
	t.Helper()
	cfg.Storage = &testStorageID
	conn, logs := newTestConnector(t, cfg, next)
	conn.id = testConnectorID
	return conn, logs
}

func startFileStorage(t *testing.T, dir string) extension.Extension {
	t.Helper()
	f := filestorage.NewFactory()
	cfg := f.CreateDefaultConfig().(*filestorage.Config)
	cfg.Directory = dir
	ext, err := f.Create(context.Background(), extensiontest.NewNopSettings(f.Type()), cfg)
	require.NoError(t, err)
	require.NoError(t, ext.Start(context.Background(), componenttest.NewNopHost()))
	return ext
}

func restoredLog(t *testing.T, logs *observer.ObservedLogs) map[string]any {
	t.Helper()
	entries := logs.FilterMessage("Restored the log_replay state").All()
	require.Len(t, entries, 1)
	assert.Equal(t, zapcore.InfoLevel, entries[0].Level)
	return entries[0].ContextMap()
}

type notStorage struct {
	component.StartFunc
	component.ShutdownFunc
}

func keptPods(t *testing.T, conn *replayConnector) []string {
	t.Helper()
	var pods []string
	for _, k := range conn.state.entries() {
		ld, err := decodeRecord(k.entry.Record)
		require.NoError(t, err)
		pod, ok := ld.ResourceLogs().At(0).Resource().Attributes().Get("k8s.pod.name")
		require.True(t, ok)
		pods = append(pods, pod.Str())
	}
	return pods
}

// TestPersistRestartFileStorage restarts the connector on a real file_storage extension, in a
// synctest bubble. bbolt works in a bubble: with a free lock and no compaction, it makes
// only system calls.
func TestPersistRestartFileStorage(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := context.Background()
		dir := t.TempDir()
		// The bubble clock starts in 2000. The records are an hour older, so
		// that the restore does not cap their times.
		ts := time.Now().Add(-time.Hour)
		ld := mergeLogs(podLogs("p1", ts, "app started version=1"), podLogs("p2", ts, "app started version=2"))

		ext := startFileStorage(t, dir)
		conn1, _ := newStorageConnector(t, testConfig(), consumertest.NewNop())
		require.NoError(t, conn1.Start(ctx, testHost{testStorageID: ext}))
		require.NoError(t, conn1.ConsumeLogs(ctx, ld))
		require.NoError(t, conn1.Shutdown(ctx))
		require.NoError(t, ext.Shutdown(ctx))
		assert.FileExists(t, filepath.Join(dir, "connector_logreplay_vllm"))

		ext = startFileStorage(t, dir)
		sink := new(consumertest.LogsSink)
		conn2, logs2 := newStorageConnector(t, testConfig(), sink)
		conn2.now = time.Now
		require.NoError(t, conn2.Start(ctx, testHost{testStorageID: ext}))
		fields := restoredLog(t, logs2)
		assert.Equal(t, int64(2), fields["kept"])
		assert.Equal(t, int64(0), fields["dropped"])
		assert.Equal(t, int64(0), fields["changed"])

		synctest.Wait()
		assert.Empty(t, sink.AllLogs(), "Start does not send the restored records")
		time.Sleep(startupReplayDelay - time.Nanosecond)
		synctest.Wait()
		assert.Empty(t, sink.AllLogs(), "nothing before the first tick")
		time.Sleep(time.Nanosecond)
		synctest.Wait()
		require.Len(t, sink.AllLogs(), 1, "the first tick sends the restored records in one call")
		now := time.Now()
		requireLogsEqual(t, mergeLogs(copyOf(ld, 0, 0, now), copyOf(ld, 1, 0, now)), sink.AllLogs()[0])

		require.NoError(t, conn2.Shutdown(ctx))
		require.NoError(t, ext.Shutdown(ctx))
	})
}

func storedState(t *testing.T, m *memStorage, cfg *Config, ld plog.Logs) {
	t.Helper()
	conn, _ := newStorageConnector(t, cfg, consumertest.NewNop())
	require.NoError(t, conn.Start(t.Context(), m.host()))
	require.NoError(t, conn.ConsumeLogs(t.Context(), ld))
	require.NoError(t, conn.Shutdown(t.Context()))
}

func TestPersistRestoreChecksEntries(t *testing.T) {
	logs := func() plog.Logs {
		ld := mergeLogs(
			podLogs("p1", base, "app started version=1"),
			podLogs("p2", base.Add(time.Second), "app started version=2 big"),
		)
		ld.ResourceLogs().At(1).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("log.iostream", "stderr")
		// Each record is larger than the smallest max_record_bytes.
		for _, rl := range ld.ResourceLogs().All() {
			rl.Resource().Attributes().PutStr("k8s.pod.label.long", strings.Repeat("x", minMaxRecordBytes))
		}
		return ld
	}
	for _, tc := range []struct {
		name  string
		after func(*Config)
		want  []string
		// src is nil for podFile.
		src              func(pod string) string
		dropped, changed int64
	}{
		{name: "same_configuration", after: func(*Config) {}, want: []string{"p1", "p2"}},
		{
			name:    "rule_renamed",
			after:   func(c *Config) { c.Rules[0].Name = "other" },
			dropped: 2,
		},
		{
			name:    "body_contains_changed",
			after:   func(c *Config) { c.Rules[0].BodyContains = []string{"big"} },
			want:    []string{"p2"},
			dropped: 1,
		},
		{
			name:    "condition_added",
			after:   func(c *Config) { c.Rules[0].Conditions = []string{`attributes["log.iostream"] == "stdout"`} },
			want:    []string{"p1"},
			dropped: 1,
		},
		{
			name:    "identity_changed",
			after:   func(c *Config) { c.Rules[0].Identity = []string{"k8s.pod.uid"} },
			dropped: 2,
		},
		{
			// Another attribute that both records have: the source file is
			// read again from the record.
			name:    "source_file_attribute_changed",
			after:   func(c *Config) { c.SourceFileAttribute = "log.iostream" },
			want:    []string{"p1", "p2"},
			src:     func(pod string) string { return map[string]string{"p1": "stdout", "p2": "stderr"}[pod] },
			changed: 2,
		},
		{
			name:    "source_file_attribute_missing",
			after:   func(c *Config) { c.SourceFileAttribute = "log.file.name" },
			dropped: 2,
		},
		{
			name:    "max_entries_lowered",
			after:   func(c *Config) { c.Rules[0].MaxEntries = 1 },
			want:    []string{"p2"},
			dropped: 1,
		},
		{
			name:    "max_record_bytes_lowered",
			after:   func(c *Config) { c.MaxRecordBytes = minMaxRecordBytes },
			dropped: 2,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newMemStorage()
			storedState(t, m, testConfig(), logs())

			cfg := testConfig()
			tc.after(cfg)
			conn, logs2 := newStorageConnector(t, cfg, consumertest.NewNop())
			require.NoError(t, conn.Start(t.Context(), m.host()))
			fields := restoredLog(t, logs2)
			assert.Equal(t, tc.dropped, fields["dropped"])
			assert.Equal(t, tc.changed, fields["changed"])
			pods := keptPods(t, conn)
			assert.ElementsMatch(t, tc.want, pods)
			src := tc.src
			if src == nil {
				src = podFile
			}
			for i, k := range conn.state.entries() {
				assert.Equal(t, src(pods[i]), k.entry.SourceFile)
			}
			require.NoError(t, conn.Shutdown(t.Context()))
			if tc.dropped > 0 || tc.changed > 0 {
				assert.Len(t, m.client.record(t).Rules[cfg.Rules[0].Name], len(tc.want), "the record is rewritten")
			}
		})
	}
}

func TestPersistRestoreDropsBadEntries(t *testing.T) {
	m := newMemStorage()
	storedState(t, m, testConfig(), podLogs("p1", base, "app started"))
	snap := m.client.record(t)
	good := snap.Rules["app_start"][podKey("p1")]

	bad := func(mod func(*entry)) entry {
		e := good
		mod(&e)
		return e
	}
	two, err := jsonMarshaler.MarshalLogs(mergeLogs(podLogs("p2", base, "app started"), podLogs("p3", base, "app started")))
	require.NoError(t, err)
	snap.Rules["app_start"] = map[string]entry{
		podKey("p1"):  good,
		"wrong key":   good,
		podKey("bad"): bad(func(e *entry) { e.Record = json.RawMessage(`{"resourceLogs":1}`) }),
		podKey("p2"):  bad(func(e *entry) { e.Record = two }),
		podKey("p4"):  bad(func(e *entry) { e.Time = 0 }),
		podKey("p5"):  bad(func(e *entry) { e.Identity = nil }),
	}
	snap.Rules["gone_rule"] = map[string]entry{podKey("p1"): good}
	b, err := json.Marshal(map[string]any{"version": stateRecordVersion, "data": snap})
	require.NoError(t, err)
	m.client.put(b)

	conn, logs := newStorageConnector(t, testConfig(), consumertest.NewNop())
	require.NoError(t, conn.Start(t.Context(), m.host()))
	fields := restoredLog(t, logs)
	assert.Equal(t, int64(1), fields["kept"])
	assert.Equal(t, int64(6), fields["dropped"])
	assert.Equal(t, []string{podKey("p1")}, keys(conn.state, 0))
	require.NoError(t, conn.Shutdown(t.Context()))
}

func TestPersistUnusableRecord(t *testing.T) {
	for name, raw := range map[string]string{
		"corrupt":       "{",
		"other_version": `{"version":99,"data":{"rules":{}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				m := newMemStorage()
				m.client.put([]byte(raw))
				conn, logs := newStorageConnector(t, testConfig(), consumertest.NewNop())
				require.NoError(t, conn.Start(context.Background(), m.host()))
				assert.Zero(t, logs.FilterMessage("Restored the log_replay state").Len())
				assert.Empty(t, conn.state.entries())
				time.Sleep(2 * time.Second)
				synctest.Wait()
				assert.Equal(t, 1, m.client.setCount(), "the unusable record is replaced")
				assert.Empty(t, m.client.record(t).Rules)
				require.NoError(t, conn.Shutdown(context.Background()))
			})
		})
	}
}

func TestPersistNoRecordWritesOne(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m := newMemStorage()
		conn, _ := newStorageConnector(t, testConfig(), consumertest.NewNop())
		require.NoError(t, conn.Start(context.Background(), m.host()))
		time.Sleep(2 * time.Second)
		synctest.Wait()
		assert.Equal(t, 1, m.client.setCount())
		require.NoError(t, conn.Shutdown(context.Background()))
	})
}

// TestPersistGetError checks that a read error at start leaves the stored record as it is.
func TestPersistGetError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := context.Background()
		m := newMemStorage()
		m.client.put([]byte(`{"version":1,"data":{"rules":{}}}`))
		m.client.getErr = errors.New("disk read failed")
		conn, logs := newStorageConnector(t, testConfig(), consumertest.NewNop())
		require.NoError(t, conn.Start(ctx, m.host()))
		warns := logs.FilterMessage("Failed to read the stored log_replay state. The kept log records are in memory only until the next restart, so that the stored state is not replaced.").All()
		require.Len(t, warns, 1)
		assert.Nil(t, conn.store)
		assert.Equal(t, 1, m.client.closes, "the client is closed")
		require.NoError(t, conn.ConsumeLogs(ctx, podLogs("p1", base, "app started")))
		assert.Len(t, conn.state.entries(), 1, "the records are kept in memory")
		time.Sleep(10 * time.Minute)
		synctest.Wait()
		require.NoError(t, conn.Shutdown(ctx))
		assert.Zero(t, m.client.setCount(), "nothing replaces the stored record")
	})
}

func TestPersistStartErrors(t *testing.T) {
	conn, _ := newStorageConnector(t, testConfig(), consumertest.NewNop())
	err := conn.Start(t.Context(), testHost{})
	require.ErrorIs(t, err, errExtensionNotFound)
	require.NoError(t, conn.Shutdown(t.Context()))

	conn, _ = newStorageConnector(t, testConfig(), consumertest.NewNop())
	err = conn.Start(t.Context(), testHost{testStorageID: notStorage{}})
	require.ErrorIs(t, err, errNotStorageExtension)
	require.NoError(t, conn.Shutdown(t.Context()))
}

func TestPersistStorageUnavailable(t *testing.T) {
	m := newMemStorage()
	m.err = errors.New("permission denied")
	sink := new(consumertest.LogsSink)
	conn, logs := newStorageConnector(t, testConfig(), sink)
	require.NoError(t, conn.Start(t.Context(), m.host()))
	errs := logs.FilterMessage("Failed to open the log_replay state storage. The kept log records are in memory only and do not survive a restart.").All()
	require.Len(t, errs, 1)
	assert.Equal(t, zapcore.ErrorLevel, errs[0].Level)
	assert.Nil(t, conn.store)

	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("p1", base, "app started")))
	conn.tick(t.Context())
	assert.Len(t, sink.AllLogs(), 1)
	require.NoError(t, conn.Shutdown(t.Context()))
}

func TestPersistUsesConnectorKindAndID(t *testing.T) {
	m := newMemStorage()
	conn, _ := newStorageConnector(t, testConfig(), consumertest.NewNop())
	require.NoError(t, conn.Start(t.Context(), m.host()))
	require.NoError(t, conn.Shutdown(t.Context()))
	assert.Equal(t, component.KindConnector, m.kind)
	assert.Equal(t, component.MustNewIDWithName("logreplay", "vllm"), m.id)
	assert.Equal(t, 1, m.client.closes)
}

func TestPersistSavesChanges(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := context.Background()
		m := newMemStorage()
		conn, _ := newStorageConnector(t, testConfig(), consumertest.NewNop())
		files := newFakeFiles()
		conn.checkSource = files.stat
		require.NoError(t, conn.Start(ctx, m.host()))
		time.Sleep(2 * time.Second)
		synctest.Wait()
		sets := m.client.setCount()

		require.NoError(t, conn.ConsumeLogs(ctx, mergeLogs(podLogs("p1", base, "app started"), podLogs("p2", base, "app started"))))
		time.Sleep(2 * time.Second)
		synctest.Wait()
		assert.Equal(t, sets+1, m.client.setCount(), "a change is saved within the debounce")
		assert.Len(t, m.client.record(t).Rules["app_start"], 2)

		// One write saves the removal and the time of the copies.
		files.remove(podFile("p1"))
		conn.tick(ctx)
		time.Sleep(2 * time.Second)
		synctest.Wait()
		assert.Equal(t, sets+2, m.client.setCount())
		rec := m.client.record(t)
		assert.Len(t, rec.Rules["app_start"], 1)
		assert.Equal(t, testNow.UnixNano(), rec.LastReplay)

		// A tick that only sends saves the time of its copies.
		conn.tick(ctx)
		time.Sleep(2 * time.Second)
		synctest.Wait()
		assert.Equal(t, sets+3, m.client.setCount())

		// A tick whose send fails saves nothing.
		conn.next = &errConsumer{err: errors.New("throttled")}
		conn.tick(ctx)
		time.Sleep(10 * time.Minute)
		synctest.Wait()
		assert.Equal(t, sets+3, m.client.setCount())
		require.NoError(t, conn.Shutdown(ctx))
	})
}

func TestPersistDisabled(t *testing.T) {
	conn, _ := newTestConnector(t, testConfig(), consumertest.NewNop())
	require.NoError(t, conn.Start(t.Context(), testHost{}))
	assert.Nil(t, conn.store)
	require.NoError(t, conn.Shutdown(t.Context()))
}

func TestPersistFullStateFits(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := context.Background()
		m := newMemStorage()
		cfg := testConfig()
		cfg.MaxStateBytes = minMaxStateBytes
		cfg.MaxRecordBytes = 4 << 10
		conn, _ := newStorageConnector(t, cfg, consumertest.NewNop())
		require.NoError(t, conn.Start(ctx, m.host()))
		for i := range 100 {
			pod := "pod-<&>-" + string(rune('a'+i%26)) + string(rune('a'+i/26))
			require.NoError(t, conn.ConsumeLogs(ctx, podLogs(pod, base.Add(time.Duration(i)), "app started   "+string(make([]byte, 500)))))
		}
		require.NoError(t, conn.Shutdown(ctx))
		assert.Greater(t, len(m.client.data[stateRecordKey]), minMaxStateBytes/2)
	})
}

// A stored time ahead of the clock comes back as now, so that it cannot hold back newer records.
func TestPersistRestoreCapsTimes(t *testing.T) {
	m := newMemStorage()
	future := testNow.Add(24 * time.Hour)
	first, _ := newStorageConnector(t, testConfig(), consumertest.NewNop())
	first.now = func() time.Time { return future.Add(time.Hour) }
	require.NoError(t, first.Start(t.Context(), m.host()))
	require.NoError(t, first.ConsumeLogs(t.Context(), podLogs("p1", future, "app started")))
	first.tick(t.Context())
	require.NoError(t, first.Shutdown(t.Context()))
	require.Equal(t, future.Add(time.Hour).UnixNano(), m.client.record(t).LastReplay)

	conn, logs := newStorageConnector(t, testConfig(), consumertest.NewNop())
	require.NoError(t, conn.Start(t.Context(), m.host()))
	fields := restoredLog(t, logs)
	assert.Equal(t, int64(1), fields["changed"])
	kept := conn.state.entries()
	require.Len(t, kept, 1)
	assert.Equal(t, testNow.UnixNano(), kept[0].entry.Time)
	assert.Equal(t, testNow.UnixNano(), conn.state.lastReplayTime())
	require.NoError(t, conn.Shutdown(t.Context()))
}

func TestPersistRestartWaitsForInterval(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := context.Background()
		m := newMemStorage()
		ts := time.Now().Add(-time.Hour)
		first, _ := newStorageConnector(t, testConfig(), consumertest.NewNop())
		first.now = time.Now
		require.NoError(t, first.Start(ctx, m.host()))
		require.NoError(t, first.ConsumeLogs(ctx, podLogs("p1", ts, "app started")))
		time.Sleep(startupReplayDelay)
		synctest.Wait()
		require.NoError(t, first.Shutdown(ctx))
		lastCopies := time.Now()

		time.Sleep(5 * time.Minute)
		sink := new(consumertest.LogsSink)
		conn, _ := newStorageConnector(t, testConfig(), sink)
		conn.now = time.Now
		require.NoError(t, conn.Start(ctx, m.host()))
		time.Sleep(time.Until(lastCopies.Add(time.Hour)) - time.Nanosecond)
		synctest.Wait()
		assert.Empty(t, sink.AllLogs(), "no copies before one interval after the latest ones")
		time.Sleep(time.Nanosecond)
		synctest.Wait()
		assert.Len(t, sink.AllLogs(), 1)
		require.NoError(t, conn.Shutdown(ctx))
	})
}

// gateConsumer blocks until release is closed, and ignores ctx, like the batch processor.
type gateConsumer struct {
	once    sync.Once
	entered chan struct{}
	release chan struct{}
}

func newGateConsumer() *gateConsumer {
	return &gateConsumer{entered: make(chan struct{}), release: make(chan struct{})}
}

func (*gateConsumer) Capabilities() consumer.Capabilities { return consumer.Capabilities{} }

func (g *gateConsumer) ConsumeLogs(context.Context, plog.Logs) error {
	g.once.Do(func() { close(g.entered) })
	<-g.release
	return nil
}

func TestPersistShutdownDuringTick(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := context.Background()
		m := newMemStorage()
		gate := newGateConsumer()
		conn, _ := newStorageConnector(t, testConfig(), gate)
		require.NoError(t, conn.Start(ctx, m.host()))
		require.NoError(t, conn.ConsumeLogs(ctx, podLogs("p1", base, "app started")))
		<-gate.entered
		done := make(chan error, 1)
		go func() { done <- conn.Shutdown(ctx) }()
		synctest.Wait()
		close(gate.release)
		require.NoError(t, <-done)
		assert.Equal(t, testNow.UnixNano(), m.client.record(t).LastReplay)

		restarted, _ := newStorageConnector(t, testConfig(), consumertest.NewNop())
		require.NoError(t, restarted.Start(ctx, m.host()))
		assert.Equal(t, restarted.cfg.Interval, restarted.firstDelay())
		require.NoError(t, restarted.Shutdown(ctx))
	})
}

func TestPersistShutdownStuckTick(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := context.Background()
		m := newMemStorage()
		gate := newGateConsumer()
		conn, _ := newStorageConnector(t, testConfig(), gate)
		require.NoError(t, conn.Start(ctx, m.host()))
		require.NoError(t, conn.ConsumeLogs(ctx, podLogs("p2", base, "app started")))
		<-gate.entered
		closes := m.client.closes
		done := make(chan error, 1)
		go func() { done <- conn.Shutdown(ctx) }()
		time.Sleep(maxReplayWait)
		synctest.Wait()
		m.client.mu.Lock()
		assert.Equal(t, closes+1, m.client.closes, "the store is saved and closed after maxReplayWait")
		m.client.mu.Unlock()
		assert.Len(t, m.client.record(t).Rules["app_start"], 1)
		close(gate.release)
		require.NoError(t, <-done)
	})
}
