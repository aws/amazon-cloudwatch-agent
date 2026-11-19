// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package signaltoinfometricconnector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
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
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/aws/amazon-cloudwatch-agent/internal/statestore"
)

var (
	testStorageID   = component.MustNewIDWithName("file_storage", "state")
	testConnectorID = component.MustNewIDWithName(TypeStr, "vllm")
)

// testHost is a component.Host with the given extensions.
type testHost map[component.ID]component.Component

func (h testHost) GetExtensions() map[component.ID]component.Component {
	return h
}

// memClient is an in-memory storage.Client that counts writes and closes,
// and can fail reads and writes. It keeps its data after Close, so a test
// can restart a connector on it.
type memClient struct {
	mu     sync.Mutex
	data   map[string][]byte
	sets   int
	closes int
	getErr error
	setErr error
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
	return nil
}

func (c *memClient) put(value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[stateRecordKey] = []byte(value)
}

func (c *memClient) raw() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.data[stateRecordKey]
}

func (c *memClient) setCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sets
}

func (c *memClient) closeCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closes
}

func (c *memClient) failGets(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.getErr = err
}

func (c *memClient) failSets(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setErr = err
}

// record decodes the stored record and checks its envelope.
func (c *memClient) record(t *testing.T) stateSnapshot {
	t.Helper()
	b := c.raw()
	require.NotNil(t, b, "no record stored")
	var r struct {
		Version int           `json:"version"`
		Data    stateSnapshot `json:"data"`
	}
	require.NoError(t, json.Unmarshal(b, &r))
	require.Equal(t, stateRecordVersion, r.Version)
	return r.Data
}

// memStorage is a storage extension that returns the same memClient to
// every GetClient call and records what it was asked for.
type memStorage struct {
	component.StartFunc
	component.ShutdownFunc
	client *memClient

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
	return m.client, nil
}

func (m *memStorage) host() component.Host {
	return testHost{testStorageID: m}
}

// newStorageConnector returns a test connector (see newTestConnector) with
// storage configured as testStorageID, the ID testConnectorID, and no start
// jitter.
func newStorageConnector(t testing.TB, cfg *Config, next consumer.Metrics) (*infoMetricConnector, *observer.ObservedLogs) {
	t.Helper()
	cfg.Storage = &testStorageID
	conn, logs := newTestConnector(t, cfg, next)
	conn.id = testConnectorID
	conn.jitter = func(time.Duration) time.Duration { return 0 }
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

// TestPersistRestartVLLM restarts the connector on a real file_storage
// extension. The first instance consumes the vLLM fixtures. The second
// instance, with a new extension on the same directory, receives no logs
// (the container log rotated away), and its first tick emits the five
// series with the labels and start times of the first instance.
//
// It runs in a synctest bubble, so the test can show that Start does not
// emit and that the first tick comes exactly after the startup delay. This
// works with bbolt: it uses only system calls (open, flock, mmap, write),
// which a bubble allows, and no timer or goroutine of its own, because the
// lock is free and file_storage compaction is off.
func TestPersistRestartVLLM(t *testing.T) {
	golden := loadGolden(t)
	fixtures := loadFixtures(t)
	synctest.Test(t, func(t *testing.T) {
		ctx := context.Background()
		dir := t.TempDir()

		ext := startFileStorage(t, dir)
		sink1 := new(consumertest.MetricsSink)
		conn1, _ := newStorageConnector(t, loadVLLMRuleFile(t), sink1)
		require.NoError(t, conn1.Start(ctx, testHost{testStorageID: ext}))
		require.NoError(t, conn1.ConsumeLogs(ctx, fixtureLogs(fixtures)))
		require.Len(t, sink1.AllMetrics(), 1, "emit on change")
		first := toGolden(t, allSeries(t, sink1.AllMetrics()))
		assertGolden(t, golden, first)
		require.NoError(t, conn1.Shutdown(ctx))
		require.NoError(t, ext.Shutdown(ctx))
		assert.FileExists(t, filepath.Join(dir, "connector_signaltoinfometric_vllm"))

		ext = startFileStorage(t, dir)
		sink2 := new(consumertest.MetricsSink)
		conn2, logs2 := newStorageConnector(t, loadVLLMRuleFile(t), sink2)
		conn2.startupDelay = time.Second
		require.NoError(t, conn2.Start(ctx, testHost{testStorageID: ext}))
		fields := restoredLog(t, logs2)
		assert.Equal(t, int64(5), fields["kept"])
		assert.Equal(t, int64(0), fields["dropped"])

		synctest.Wait()
		assert.Empty(t, sink2.AllMetrics(), "Start does not emit the restored entities")
		time.Sleep(conn2.startupDelay - time.Nanosecond)
		synctest.Wait()
		assert.Empty(t, sink2.AllMetrics(), "no emission before the first tick")
		time.Sleep(time.Nanosecond)
		synctest.Wait()
		require.Len(t, sink2.AllMetrics(), 1, "the first tick emits the restored entities in one call")
		assertGolden(t, first, toGolden(t, allSeries(t, sink2.AllMetrics())))

		require.NoError(t, conn2.Shutdown(ctx))
		require.NoError(t, ext.Shutdown(ctx))
	})
}

// restoredLog returns the fields of the one Info log of a restore.
func restoredLog(t *testing.T, logs *observer.ObservedLogs) map[string]any {
	t.Helper()
	entries := logs.FilterMessage("Restored the Info metric state").All()
	require.Len(t, entries, 1)
	assert.Equal(t, zapcore.InfoLevel, entries[0].Level)
	return entries[0].ContextMap()
}

// restoreLogs are the logs of the first instance in the restore tests: u1,
// u2, and u333, each a second more recent than the one before.
func restoreLogs() plog.Logs {
	ld := podLogs("u1", base, "app started version=1.2.3 mode=fast region=eu")
	addRecord(addResource(ld, podResource("u2")), base.Add(time.Second), "app started version=2")
	addRecord(addResource(ld, podResource("u333")), base.Add(2*time.Second), "app started version=3 region=us")
	return ld
}

// restoredEntity is what the restore tests check of an entity.
type restoredEntity struct {
	resource []kv
	facts    []kv
}

func restoredEntities(snap stateSnapshot, rule string) map[string]restoredEntity {
	out := map[string]restoredEntity{}
	for _, e := range snap.Rules[rule] {
		out[e.Identity[0].Value] = restoredEntity{resource: e.Resource, facts: e.Facts}
	}
	return out
}

func TestPersistRestoreNormalizes(t *testing.T) {
	podName := func(uid string) []kv { return []kv{{"k8s.pod.name", "pod-" + uid}, {"k8s.pod.uid", uid}} }
	asStored := map[string]restoredEntity{
		"u1":   {resource: podName("u1"), facts: []kv{{"version", "1.2.3"}, {"mode", "fast"}, {"region", "eu"}}},
		"u2":   {resource: podName("u2"), facts: []kv{{"version", "2"}, {"mode", "default"}}},
		"u333": {resource: podName("u333"), facts: []kv{{"version", "3"}, {"mode", "default"}, {"region", "us"}}},
	}
	for _, tc := range []struct {
		name string
		// before edits the configuration of the first instance, after the
		// one of the second.
		before, after func(*Config)
		want          map[string]restoredEntity
		// kept, dropped, and normalized are the fields of the restore log.
		kept, dropped, normalized int64
	}{
		{
			name:  "same_configuration",
			after: func(*Config) {},
			want:  asStored,
			kept:  3,
		},
		{
			name:  "removed_attribute",
			after: func(cfg *Config) { cfg.Rules[0].Attributes = cfg.Rules[0].Attributes[:2] },
			want: map[string]restoredEntity{
				"u1":   {resource: podName("u1"), facts: []kv{{"version", "1.2.3"}, {"mode", "fast"}}},
				"u2":   asStored["u2"],
				"u333": {resource: podName("u333"), facts: []kv{{"version", "3"}, {"mode", "default"}}},
			},
			kept:       3,
			normalized: 2,
		},
		{
			name: "added_required_attribute",
			after: func(cfg *Config) {
				cfg.Rules[0].Attributes = append(cfg.Rules[0].Attributes,
					AttributeConfig{Key: "build", Value: `ExtractPatterns(log.body, "build=(?P<v>\\S+)")["v"]`})
			},
			want:    map[string]restoredEntity{},
			dropped: 3,
		},
		{
			name: "added_optional_and_defaulted_attributes",
			after: func(cfg *Config) {
				cfg.Rules[0].Attributes = append(cfg.Rules[0].Attributes,
					AttributeConfig{Key: "zone", Value: `ExtractPatterns(log.body, "zone=(?P<v>\\S+)")["v"]`, Optional: true},
					AttributeConfig{Key: "tier", Value: `ExtractPatterns(log.body, "tier=(?P<v>\\S+)")["v"]`, DefaultValue: "std"})
			},
			// Absent until a record brings them.
			want: asStored,
			kept: 3,
		},
		{
			name: "reordered_attributes",
			after: func(cfg *Config) {
				a := cfg.Rules[0].Attributes
				cfg.Rules[0].Attributes = []AttributeConfig{a[2], a[0], a[1]}
			},
			want: map[string]restoredEntity{
				"u1":   {resource: podName("u1"), facts: []kv{{"region", "eu"}, {"version", "1.2.3"}, {"mode", "fast"}}},
				"u2":   asStored["u2"],
				"u333": {resource: podName("u333"), facts: []kv{{"region", "us"}, {"version", "3"}, {"mode", "default"}}},
			},
			kept:       3,
			normalized: 2,
		},
		{
			name: "shorter_max_value_length",
			after: func(cfg *Config) {
				cfg.MaxValueLength = 3
				cfg.Rules[0].Attributes[1].DefaultValue = "dft"
			},
			// The identity u333 is longer than 3 bytes, so no record gives
			// its key any more.
			want: map[string]restoredEntity{
				"u1": {resource: []kv{{"k8s.pod.name", "pod"}, {"k8s.pod.uid", "u1"}}, facts: []kv{{"version", "1.2"}, {"mode", "fas"}, {"region", "eu"}}},
				"u2": {resource: []kv{{"k8s.pod.name", "pod"}, {"k8s.pod.uid", "u2"}}, facts: []kv{{"version", "2"}, {"mode", "def"}}},
			},
			kept:       2,
			dropped:    1,
			normalized: 2,
		},
		{
			name: "changed_include_resource_attributes",
			before: func(cfg *Config) {
				cfg.Rules[0].IncludeResourceAttributes = []string{"k8s.pod.uid", "k8s.pod.name"}
			},
			after: func(cfg *Config) {
				cfg.Rules[0].IncludeResourceAttributes = []string{"k8s.namespace.name", "k8s.pod.uid"}
			},
			// The removed key is gone, and the added key has no value until
			// a record brings it.
			want: map[string]restoredEntity{
				"u1":   {resource: []kv{{"k8s.pod.uid", "u1"}}, facts: asStored["u1"].facts},
				"u2":   {resource: []kv{{"k8s.pod.uid", "u2"}}, facts: asStored["u2"].facts},
				"u333": {resource: []kv{{"k8s.pod.uid", "u333"}}, facts: asStored["u333"].facts},
			},
			kept:       3,
			normalized: 3,
		},
		{
			name:    "renamed_rule",
			after:   func(cfg *Config) { cfg.Rules[0].Name = "app_info_v2" },
			want:    map[string]restoredEntity{},
			dropped: 3,
		},
		{
			name:    "lower_max_entities",
			after:   func(cfg *Config) { cfg.Rules[0].MaxEntities = 2 },
			want:    map[string]restoredEntity{"u2": asStored["u2"], "u333": asStored["u333"]},
			kept:    2,
			dropped: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			mem := newMemStorage()
			cfg1 := testConfig()
			if tc.before != nil {
				tc.before(cfg1)
			}
			conn1, _ := newStorageConnector(t, cfg1, consumertest.NewNop())
			require.NoError(t, conn1.Start(ctx, mem.host()))
			require.NoError(t, conn1.ConsumeLogs(ctx, restoreLogs()))
			require.NoError(t, conn1.Shutdown(ctx))
			saved := mem.client.setCount()
			require.Equal(t, 1, saved)

			cfg2 := testConfig()
			tc.after(cfg2)
			conn2, logs := newStorageConnector(t, cfg2, consumertest.NewNop())
			require.NoError(t, conn2.Start(ctx, mem.host()))
			assert.Equal(t, tc.want, restoredEntities(conn2.state.snapshot(), cfg2.Rules[0].Name))
			fields := restoredLog(t, logs)
			assert.Equal(t, tc.kept, fields["kept"], "kept")
			assert.Equal(t, tc.dropped, fields["dropped"], "dropped")
			assert.Equal(t, tc.normalized, fields["normalized"], "normalized")

			// A restore that dropped or changed entities saves the cleaned
			// state; one that did not, writes nothing.
			require.NoError(t, conn2.Shutdown(ctx))
			if tc.dropped == 0 && tc.normalized == 0 {
				assert.Equal(t, saved, mem.client.setCount())
				return
			}
			assert.Equal(t, saved+1, mem.client.setCount())
			assert.Equal(t, tc.want, restoredEntities(mem.client.record(t), cfg2.Rules[0].Name))
		})
	}
}

// TestPersistRestoreKeepsTimes checks that a restored entity keeps its
// times, so that an older record is still ignored and the series keeps its
// start time.
func TestPersistRestoreKeepsTimes(t *testing.T) {
	ctx := context.Background()
	mem := newMemStorage()
	conn1, _ := newStorageConnector(t, testConfig(), consumertest.NewNop())
	require.NoError(t, conn1.Start(ctx, mem.host()))
	require.NoError(t, conn1.ConsumeLogs(ctx, podLogs("u1", base, "app started version=1")))
	require.NoError(t, conn1.ConsumeLogs(ctx, podLogs("u1", base.Add(time.Hour), "app started version=2")))
	require.NoError(t, conn1.ConsumeLogs(ctx, podLogs("u1", base.Add(2*time.Hour), "noise")))
	want := conn1.state.snapshot()
	require.NoError(t, conn1.Shutdown(ctx))

	sink := new(consumertest.MetricsSink)
	conn2, _ := newStorageConnector(t, testConfig(), sink)
	require.NoError(t, conn2.Start(ctx, mem.host()))
	assert.Equal(t, want, conn2.state.snapshot())
	require.NoError(t, conn2.ConsumeLogs(ctx, podLogs("u1", base.Add(30*time.Minute), "app started version=1")))
	assert.Empty(t, sink.AllMetrics(), "the older record is ignored")
	require.NoError(t, conn2.Shutdown(ctx))
}

func TestNormalizeEntity(t *testing.T) {
	rc := &testConfig().Rules[0]
	identity := []kv{{"k8s.pod.uid", "u1"}}
	e := entity{
		Identity: identity,
		Resource: []kv{{"k8s.pod.name", "p"}, {"k8s.pod.name", "second"}, {"other", "x"}},
		// A duplicate key keeps the first value, and an empty value is no
		// value.
		Facts:     []kv{{"region", ""}, {"version", "1"}, {"version", "2"}, {"unknown", "x"}},
		FirstSeen: 1,
		Updated:   2,
		Activity:  3,
	}
	got, ok := normalizeEntity(e, rc, 256)
	require.True(t, ok)
	assert.Equal(t, entity{
		Identity:  identity,
		Resource:  []kv{{"k8s.pod.name", "p"}},
		Facts:     []kv{{"version", "1"}},
		FirstSeen: 1,
		Updated:   2,
		Activity:  3,
	}, got)

	// A cut that leaves nothing of a required value drops the entity.
	e.Facts = []kv{{"version", "日"}}
	_, ok = normalizeEntity(e, rc, 2)
	assert.False(t, ok)
	e.Facts = []kv{{"version", "1"}}
	_, ok = normalizeEntity(e, rc, 2)
	assert.True(t, ok)
	// So does an identity value longer than max_value_length.
	_, ok = normalizeEntity(e, rc, 1)
	assert.False(t, ok)

	// Invalid UTF-8 in a fact or resource value is replaced with U+FFFD,
	// then cut to max_value_length.
	e.Facts = []kv{{"version", "1\xff"}, {"region", "abc\x80"}}
	e.Resource = []kv{{"k8s.pod.name", "p\xfe\xfd"}}
	got, ok = normalizeEntity(e, rc, 4)
	require.True(t, ok)
	assert.Equal(t, []kv{{"version", "1\uFFFD"}, {"region", "abc"}}, got.Facts)
	assert.Equal(t, []kv{{"k8s.pod.name", "p\uFFFD"}}, got.Resource)
	// An identity value that is not valid UTF-8 drops the entity: a record
	// gives the replaced value, so no record gives its state key.
	e.Identity = []kv{{"k8s.pod.uid", "u\xff"}}
	_, ok = normalizeEntity(e, rc, 256)
	assert.False(t, ok)
}

// TestNormalizeSnapshotInvalidUTF8 checks that the normalization replaces
// invalid UTF-8 in the restored facts and resource values, and counts the
// entity as changed, so that the cleaned state is written. It gives the
// snapshot directly, because the JSON decoder of the store already replaces
// invalid bytes (see TestPersistRestoreInvalidUTF8).
func TestNormalizeSnapshotInvalidUTF8(t *testing.T) {
	cfg := testConfig()
	stored := func(uid string) entity {
		return entity{
			Identity: []kv{{"k8s.pod.uid", uid}},
			Resource: []kv{{"k8s.pod.name", "pod-\xff"}, {"k8s.pod.uid", uid}},
			Facts:    []kv{{"version", "1.\xfe\xfd"}, {"mode", "fast"}},
		}
	}
	snap := stateSnapshot{Rules: map[string]map[string]entity{"app_info": {
		key("u1"):    stored("u1"),
		key("u\xff"): stored("u\xff"),
	}}}
	assert.Equal(t, normalizeResult{dropped: 1, changed: 1}, normalizeSnapshot(&snap, cfg))
	assert.Equal(t, map[string]entity{key("u1"): {
		Identity: []kv{{"k8s.pod.uid", "u1"}},
		Resource: []kv{{"k8s.pod.name", "pod-\uFFFD"}, {"k8s.pod.uid", "u1"}},
		Facts:    []kv{{"version", "1.\uFFFD"}, {"mode", "fast"}},
	}}, snap.Rules["app_info"])
}

// TestPersistRestoreInvalidUTF8 restores a stored record with invalid UTF-8
// bytes in a fact and a resource value. The restored values are valid
// UTF-8, and the first tick marshals to OTLP protobuf. The JSON decoder
// replaces each invalid byte with U+FFFD, so here the run of two bytes gives
// two U+FFFD.
func TestPersistRestoreInvalidUTF8(t *testing.T) {
	ctx := context.Background()
	mem := newMemStorage()
	mem.client.put(strings.NewReplacer("<ff>", "\xff", "<fe>", "\xfe").Replace(`{"version":1,"data":{"rules":{"app_info":{"2:u1":{` +
		`"identity":[{"key":"k8s.pod.uid","value":"u1"}],` +
		`"resource":[{"key":"k8s.pod.name","value":"pod-<ff>"},{"key":"k8s.pod.uid","value":"u1"}],` +
		`"facts":[{"key":"version","value":"1.<fe><ff>"},{"key":"mode","value":"fast"}],` +
		`"first_seen":1,"updated":2,"activity":3}}}}}`))
	sink := new(consumertest.MetricsSink)
	conn, _ := newStorageConnector(t, testConfig(), sink)
	require.NoError(t, conn.Start(ctx, mem.host()))
	e, ok := conn.state.snapshot().Rules["app_info"][key("u1")]
	require.True(t, ok)
	assert.Equal(t, []kv{{"k8s.pod.name", "pod-\uFFFD"}, {"k8s.pod.uid", "u1"}}, e.Resource)
	assert.Equal(t, []kv{{"version", "1.\uFFFD\uFFFD"}, {"mode", "fast"}}, e.Facts)

	conn.tick(ctx, &conn.groups[0])
	require.Len(t, sink.AllMetrics(), 1)
	assertValidOTLP(t, sink.AllMetrics()[0])
	require.NoError(t, conn.Shutdown(ctx))
}

// TestPersistUnusableRecord checks that a missing, corrupt, or other
// version record gives an empty state without an error, and is replaced by
// the current state after Debounce, without any change, so that the next
// start finds a valid record and logs no warning.
func TestPersistUnusableRecord(t *testing.T) {
	const warning = "discarding unreadable stored state"
	for name, tc := range map[string]struct {
		stored   string
		warnings int
	}{
		"missing":       {},
		"corrupt":       {stored: `{"version":1,"data":{"rules":`, warnings: 1},
		"other_version": {stored: `{"version":2,"data":{"rules":{"app_info":{}}}}`, warnings: 1},
	} {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx := context.Background()
				mem := newMemStorage()
				if tc.stored != "" {
					mem.client.put(tc.stored)
				}
				conn, logs := newStorageConnector(t, testConfig(), consumertest.NewNop())
				require.NoError(t, conn.Start(ctx, mem.host()))
				assert.Empty(t, conn.state.snapshot().Rules)
				assert.Equal(t, tc.warnings, logs.FilterMessage(warning).FilterLevelExact(zapcore.WarnLevel).Len())
				assert.Zero(t, logs.FilterMessage("Restored the Info metric state").Len())

				time.Sleep(time.Second - time.Nanosecond)
				synctest.Wait()
				assert.Zero(t, mem.client.setCount())
				time.Sleep(time.Nanosecond)
				synctest.Wait()
				require.Equal(t, 1, mem.client.setCount(), "the record is replaced after Debounce")
				assert.Empty(t, mem.client.record(t).Rules)
				require.NoError(t, conn.Shutdown(ctx))
				assert.Equal(t, 1, mem.client.setCount(), "nothing pending at Shutdown")

				// The next start finds a valid record: no warning, no write.
				conn, logs = newStorageConnector(t, testConfig(), consumertest.NewNop())
				require.NoError(t, conn.Start(ctx, mem.host()))
				assert.Zero(t, logs.FilterMessage(warning).Len())
				time.Sleep(time.Minute)
				synctest.Wait()
				require.NoError(t, conn.Shutdown(ctx))
				assert.Equal(t, 1, mem.client.setCount())
			})
		})
	}
}

func TestPersistGetError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := context.Background()
		mem := newMemStorage()
		mem.client.failGets(errors.New("disk on fire"))
		conn, logs := newStorageConnector(t, testConfig(), consumertest.NewNop())
		require.NoError(t, conn.Start(ctx, mem.host()), "a read error does not fail Start")
		assert.Empty(t, conn.state.snapshot().Rules)
		warns := logs.FilterMessage("Failed to read the stored Info metric state. Starting with an empty state.").All()
		require.Len(t, warns, 1)
		assert.Equal(t, zapcore.WarnLevel, warns[0].Level)
		assert.Contains(t, warns[0].ContextMap()["error"], "disk on fire")

		// The record can be valid and the error transient, so it is not
		// replaced without a change.
		time.Sleep(time.Minute)
		synctest.Wait()
		assert.Zero(t, mem.client.setCount())

		// The state is still saved.
		require.NoError(t, conn.ConsumeLogs(ctx, podLogs("u1", base, "app started version=1")))
		require.NoError(t, conn.Shutdown(ctx))
		assert.Len(t, mem.client.record(t).Rules["app_info"], 1)
	})
}

// TestPersistStartErrors checks that a storage ID that does not name a
// storage extension fails Start. This is a configuration error.
func TestPersistStartErrors(t *testing.T) {
	nopFactory := extensiontest.NewNopFactory()
	nop, err := nopFactory.Create(context.Background(), extensiontest.NewNopSettings(nopFactory.Type()), nopFactory.CreateDefaultConfig())
	require.NoError(t, err)
	for name, tc := range map[string]struct {
		host     component.Host
		sentinel error
		want     string
	}{
		"missing_extension": {
			host:     componenttest.NewNopHost(),
			sentinel: statestore.ErrExtensionNotFound,
			want:     `failed to get the storage client: statestore: storage extension not found: "file_storage/state"`,
		},
		"not_storage": {
			host:     testHost{testStorageID: nop},
			sentinel: statestore.ErrNotStorageExtension,
			want:     `failed to get the storage client: statestore: extension is not a storage extension: "file_storage/state"`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			conn, logs := newStorageConnector(t, testConfig(), consumertest.NewNop())
			err := conn.Start(context.Background(), tc.host)
			assert.EqualError(t, err, tc.want)
			assert.ErrorIs(t, err, tc.sentinel)
			assert.Nil(t, conn.store)
			assert.Zero(t, logs.FilterLevelExact(zapcore.ErrorLevel).Len(), "the error goes to the collector, not to the log")
			require.NoError(t, conn.Shutdown(context.Background()))
		})
	}
}

// brokenStorage is a storage extension whose GetClient returns client and
// err, as file_storage does when it cannot open or create its file.
type brokenStorage struct {
	component.StartFunc
	component.ShutdownFunc
	client storage.Client
	err    error
}

var _ storage.Extension = (*brokenStorage)(nil)

func (b *brokenStorage) GetClient(context.Context, component.Kind, component.ID, string) (storage.Client, error) {
	return b.client, b.err
}

// TestPersistStorageUnavailable checks that a storage extension that exists
// but gives no usable client does not stop the connector. Start logs one
// Error, and the connector works as without storage: it consumes logs and
// emits, has no store and no store telemetry, and shuts down cleanly.
func TestPersistStorageUnavailable(t *testing.T) {
	const msg = "Failed to open the Info metric state storage. The state is kept in memory only and does not survive a restart."
	for _, tc := range []struct {
		name string
		ext  func(t *testing.T) component.Component
		// wantErr is a substring of the error field of the log.
		wantErr string
	}{
		{
			name: "get_client_error",
			ext: func(*testing.T) component.Component {
				return &brokenStorage{err: errors.New("open /var/lib/amazon-cloudwatch-agent/state/connector_signaltoinfometric_vllm: permission denied")}
			},
			wantErr: `statestore: storage extension "file_storage/state" cannot give a client: open /var/lib/amazon-cloudwatch-agent/state/connector_signaltoinfometric_vllm: permission denied`,
		},
		{
			// statestore.New refuses the nil client.
			name:    "nil_client",
			ext:     func(*testing.T) component.Component { return &brokenStorage{} },
			wantErr: "failed to create the state store: statestore: client is required",
		},
		{
			// A real file_storage extension whose directory is gone after
			// its start.
			name: "file_storage_cannot_open",
			ext: func(t *testing.T) component.Component {
				dir := t.TempDir()
				ext := startFileStorage(t, dir)
				t.Cleanup(func() { assert.NoError(t, ext.Shutdown(context.Background())) })
				require.NoError(t, os.RemoveAll(dir))
				return ext
			},
			// The OS error text that follows differs: "no such file or directory" on
			// Linux, "The system cannot find the path specified." on Windows.
			wantErr: `statestore: storage extension "file_storage/state" cannot give a client: open `,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			cfg := testConfig()
			cfg.Storage = &testStorageID
			sink := new(consumertest.MetricsSink)
			conn, tel, logs := newTelemetryConnector(t, cfg, sink)
			conn.id = testConnectorID

			require.NoError(t, conn.Start(ctx, testHost{testStorageID: tc.ext(t)}))
			entries := logs.FilterMessage(msg).All()
			require.Len(t, entries, 1)
			assert.Equal(t, zapcore.ErrorLevel, entries[0].Level)
			fields := entries[0].ContextMap()
			assert.Equal(t, testStorageID.String(), fields["storage"])
			assert.Contains(t, fields["error"], tc.wantErr)
			assert.Equal(t, 1, logs.FilterLevelExact(zapcore.ErrorLevel).Len(), "no other Error")
			assert.Nil(t, conn.store)

			require.NoError(t, conn.ConsumeLogs(ctx, podLogs("u1", base, "app started version=1")))
			got := allSeries(t, sink.AllMetrics())
			require.Len(t, got, 1, "emit on change")
			assert.Equal(t, []kv{{"k8s.pod.name", "pod-u1"}, {"k8s.pod.uid", "u1"}}, got[0].resource)
			assert.Equal(t, []kv{{"version", "1"}, {"mode", "default"}}, got[0].attrs)

			metrics := collect(t, tel)
			assert.Equal(t, map[string]int64{"rule=app_info": 1}, metrics[metricEntities])
			for _, name := range []string{metricStateSaves, metricStateSaveErrors, metricStateSaveRejected, metricStateSavedBytes} {
				assert.NotContains(t, metrics, name, "no store telemetry")
			}

			require.NoError(t, conn.Shutdown(ctx))
			assert.Equal(t, 1, logs.FilterMessage(msg).Len(), "logged once")
		})
	}
}

func TestPersistUsesConnectorKindAndID(t *testing.T) {
	mem := newMemStorage()
	conn, _ := newStorageConnector(t, testConfig(), consumertest.NewNop())
	require.NoError(t, conn.Start(context.Background(), mem.host()))
	assert.Equal(t, component.KindConnector, mem.kind)
	assert.Equal(t, component.MustNewIDWithName("signaltoinfometric", "vllm"), mem.id)
	require.NoError(t, conn.Shutdown(context.Background()))
}

func TestPersistDisabled(t *testing.T) {
	conn, _ := newTestConnector(t, testConfig(), consumertest.NewNop())
	require.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))
	assert.Nil(t, conn.store)
	require.NoError(t, conn.ConsumeLogs(context.Background(), podLogs("u1", base, "app started version=1")))
	require.NoError(t, conn.Shutdown(context.Background()))
}

// TestPersistMarks checks through the stored record that an insert, a
// change, and an eviction are saved after Debounce (1s), and that an
// activity-only batch is saved after IdleFlush (5m).
func TestPersistMarks(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := context.Background()
		mem := newMemStorage()
		cfg := testConfig()
		cfg.Rules[0].MaxEntities = 1
		conn, _ := newStorageConnector(t, cfg, consumertest.NewNop())
		start := time.Now()
		require.NoError(t, conn.Start(ctx, mem.host()))
		// at sleeps until d after Start.
		at := func(d time.Duration) {
			time.Sleep(time.Until(start.Add(d)))
			synctest.Wait()
		}
		stored := func() map[string]entity {
			return mem.client.record(t).Rules["app_info"]
		}

		// Insert.
		require.NoError(t, conn.ConsumeLogs(ctx, podLogs("u1", base, "app started version=1")))
		at(time.Second - time.Nanosecond)
		assert.Zero(t, mem.client.setCount())
		at(time.Second)
		require.Equal(t, 1, mem.client.setCount())
		assert.Equal(t, "1", stored()[key("u1")].Facts[0].Value)

		// Change.
		require.NoError(t, conn.ConsumeLogs(ctx, podLogs("u1", base.Add(time.Second), "app started version=2")))
		at(2 * time.Second)
		require.Equal(t, 2, mem.client.setCount())
		assert.Equal(t, "2", stored()[key("u1")].Facts[0].Value)

		// Insert that evicts for max_entities.
		require.NoError(t, conn.ConsumeLogs(ctx, podLogs("u2", base.Add(2*time.Second), "app started version=1")))
		at(3 * time.Second)
		require.Equal(t, 3, mem.client.setCount())
		assert.Equal(t, []string{key("u2")}, keysOf(stored()))

		// Activity only: not saved on the Debounce tick, but on the IdleFlush
		// tick.
		require.NoError(t, conn.ConsumeLogs(ctx, podLogs("u2", base.Add(3*time.Second), "noise")))
		at(4 * time.Second)
		assert.Equal(t, 3, mem.client.setCount())
		at(5*time.Minute - time.Nanosecond)
		assert.Equal(t, 3, mem.client.setCount())
		at(5 * time.Minute)
		require.Equal(t, 4, mem.client.setCount())
		assert.Equal(t, base.Add(3*time.Second).UnixNano(), stored()[key("u2")].Activity)

		// Records of an unknown pod and older facts mark nothing.
		require.NoError(t, conn.ConsumeLogs(ctx, podLogs("u9", base.Add(time.Hour), "noise", "INFO request handled")))
		require.NoError(t, conn.ConsumeLogs(ctx, podLogs("u2", base, "app started version=0")))
		at(15 * time.Minute)
		assert.Equal(t, 4, mem.client.setCount())

		require.NoError(t, conn.Shutdown(ctx))
		assert.Equal(t, 4, mem.client.setCount(), "nothing pending at Shutdown")
		assert.Equal(t, 1, mem.client.closeCount())
	})
}

func keysOf(m map[string]entity) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestPersistShutdown(t *testing.T) {
	t.Run("final_save_has_the_last_change", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			ctx := context.Background()
			mem := newMemStorage()
			conn, _ := newStorageConnector(t, testConfig(), consumertest.NewNop())
			require.NoError(t, conn.Start(ctx, mem.host()))
			require.NoError(t, conn.ConsumeLogs(ctx, podLogs("u1", base, "app started version=1")))
			time.Sleep(time.Second)
			synctest.Wait()
			require.Equal(t, 1, mem.client.setCount())

			// The change is not saved yet when Shutdown starts.
			require.NoError(t, conn.ConsumeLogs(ctx, podLogs("u1", base.Add(time.Second), "app started version=2")))
			require.NoError(t, conn.Shutdown(ctx))
			assert.Equal(t, 2, mem.client.setCount())
			assert.Equal(t, "2", mem.client.record(t).Rules["app_info"][key("u1")].Facts[0].Value)
			assert.Equal(t, 1, mem.client.closeCount())

			require.NoError(t, conn.Shutdown(ctx), "a second Shutdown is safe")
			assert.Equal(t, 2, mem.client.setCount())
			assert.Equal(t, 1, mem.client.closeCount())
		})
	})

	t.Run("without_start", func(t *testing.T) {
		mem := newMemStorage()
		conn, _ := newStorageConnector(t, testConfig(), consumertest.NewNop())
		require.NoError(t, conn.Shutdown(context.Background()))
		require.NoError(t, conn.Shutdown(context.Background()))
		assert.Zero(t, mem.client.closeCount())
	})

	t.Run("stuck_tick_does_not_block_the_final_save", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			ctx := context.Background()
			mem := newMemStorage()
			cfg := testConfig()
			cfg.EmitOnChange = false
			// The next consumer ignores its context and blocks until release.
			next := &blockingConsumer{release: make(chan struct{})}
			conn, _ := newStorageConnector(t, cfg, next)
			require.NoError(t, conn.Start(ctx, mem.host()))
			require.NoError(t, conn.ConsumeLogs(ctx, podLogs("u1", base, "app started version=1")))
			time.Sleep(conn.startupDelay)
			synctest.Wait()
			require.Equal(t, 1, next.calls(), "a tick is in the next consumer")
			// A change that is not saved yet.
			require.NoError(t, conn.ConsumeLogs(ctx, podLogs("u1", base.Add(time.Second), "app started version=2")))

			shutdownCtx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()
			start := time.Now()
			assert.ErrorIs(t, conn.Shutdown(shutdownCtx), context.DeadlineExceeded, "the wait for the tick")
			assert.Equal(t, time.Second, time.Since(start))
			assert.Equal(t, "2", mem.client.record(t).Rules["app_info"][key("u1")].Facts[0].Value, "the final save has the change")
			assert.Equal(t, 1, mem.client.closeCount())

			close(next.release)
			require.NoError(t, conn.Shutdown(ctx), "the group stops once the call returns")
			assert.Equal(t, 1, mem.client.closeCount())
		})
	})

	t.Run("save_error", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			ctx := context.Background()
			mem := newMemStorage()
			conn, _ := newStorageConnector(t, testConfig(), consumertest.NewNop())
			require.NoError(t, conn.Start(ctx, mem.host()))
			require.NoError(t, conn.ConsumeLogs(ctx, podLogs("u1", base, "app started version=1")))
			mem.client.failSets(errors.New("disk full"))
			assert.ErrorContains(t, conn.Shutdown(ctx), "disk full")
			assert.Equal(t, 1, mem.client.closeCount())
		})
	})
}

// vllmPodLogs returns one ResourceLogs per pod, pods from to to-1, each with
// one vLLM startup line of the fixtures. The pod names and the facts have
// realistic lengths. The vLLM rule also keeps the pod UID of the resource.
func vllmPodLogs(bodies []string, from, to int) plog.Logs {
	ld := plog.NewLogs()
	for i := from; i < to; i++ {
		lrs := addResource(ld, map[string]string{
			"k8s.pod.uid":        fmt.Sprintf("%08x-9a1d-4e2b-8c3f-%012x", i, i),
			"k8s.pod.name":       fmt.Sprintf("vllm-%05d-7d9f8b6c5d-x2k4p", i),
			"k8s.namespace.name": "inference",
			"k8s.container.name": "vllm",
		})
		addRecord(lrs, base.Add(time.Duration(i)*time.Millisecond), bodies[i%len(bodies)])
	}
	return ld
}

// fillState consumes batches from logs(from, to) until the state evicts,
// which means it is at max_state_bytes, and returns its size estimate.
func fillState(t *testing.T, conn *infoMetricConnector, logs func(from, to int) plog.Logs) int {
	t.Helper()
	const batchSize = 100
	counts := make([]int, len(conn.rules))
	for n := 0; ; n += batchSize {
		require.NoError(t, conn.ConsumeLogs(context.Background(), logs(n, n+batchSize)))
		bytes := conn.state.sizes(counts)
		if counts[0] < n+batchSize {
			return bytes
		}
		require.Less(t, n, 1_000_000, "the state does not fill")
	}
}

// TestPersistFullStateSaves fills the state to max_state_bytes and checks
// that the record is saved, not refused as too large.
func TestPersistFullStateSaves(t *testing.T) {
	var bodies []string
	for _, f := range loadFixtures(t) {
		for _, line := range f.lines {
			if strings.Contains(line.body, " with config: ") {
				bodies = append(bodies, line.body)
			}
		}
	}
	require.Len(t, bodies, 5)

	// valuesOf returns version values made only of characters that
	// encoding/json escapes: up to 6 bytes in JSON for 1 byte in the state.
	escaped := strings.Repeat(`<&"\`+"\x01", 200)
	escapedLogs := func(from, to int) plog.Logs {
		ld := plog.NewLogs()
		for i := from; i < to; i++ {
			addRecord(addResource(ld, podResource(fmt.Sprintf("u%06d", i))), base, "app started version="+escaped)
		}
		return ld
	}

	for _, tc := range []struct {
		name     string
		cfg      func() *Config
		maxBytes int
		logs     func(from, to int) plog.Logs
	}{
		{name: "vllm_min", cfg: func() *Config { return loadVLLMRuleFile(t) }, maxBytes: minMaxStateBytes,
			logs: func(from, to int) plog.Logs { return vllmPodLogs(bodies, from, to) }},
		{name: "vllm_default", cfg: func() *Config { return loadVLLMRuleFile(t) }, maxBytes: defaultMaxStateBytes,
			logs: func(from, to int) plog.Logs { return vllmPodLogs(bodies, from, to) }},
		{name: "escaped_values", cfg: func() *Config {
			cfg := testConfig()
			cfg.MaxValueLength = maxMaxValueLength
			return cfg
		}, maxBytes: minMaxStateBytes, logs: escapedLogs},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			cfg := tc.cfg()
			cfg.MaxStateBytes = tc.maxBytes
			cfg.Rules[0].MaxEntities = maxMaxEntities
			mem := newMemStorage()
			conn, logs := newStorageConnector(t, cfg, consumertest.NewNop())
			require.NoError(t, conn.Start(ctx, mem.host()))

			bytes := fillState(t, conn, tc.logs)
			require.LessOrEqual(t, bytes, tc.maxBytes)
			snap := conn.state.snapshot()
			var largest int
			for k, e := range snap.Rules[cfg.Rules[0].Name] {
				largest = max(largest, entitySize(k, &e))
			}
			require.Greater(t, bytes, tc.maxBytes-largest, "the state is full")

			require.NoError(t, conn.store.SaveNow(ctx))
			stats := conn.store.Stats()
			assert.Zero(t, stats.Rejected)
			assert.Zero(t, logs.FilterMessage("state record too large; not saved").Len())
			// The estimate is at least the JSON size of the entities with
			// their commas: the record is at most the estimate plus the
			// envelope {"version":1,"data":{"rules":{}}} (33 bytes) and
			// "<rule name>":{} (the name plus 5 bytes).
			assert.LessOrEqual(t, stats.LastBytes, bytes+33+len(cfg.Rules[0].Name)+5)
			assert.LessOrEqual(t, stats.LastBytes, maxRecordBytes(cfg))
			require.NoError(t, conn.Shutdown(ctx))
		})
	}
}

func TestMaxRecordBytes(t *testing.T) {
	cfg := testConfig()
	second := cfg.Rules[0]
	// The name needs escaping: a\u003cb in JSON.
	second.Name = `a<b`
	cfg.Rules = append(cfg.Rules, second)
	assert.Equal(t, cfg.MaxStateBytes+4096+len("app_info")+16+len(`a\u003cb`)+16, maxRecordBytes(cfg))
}

func TestPersistTelemetry(t *testing.T) {
	// In a bubble, so that no Debounce tick saves between the steps.
	synctest.Test(t, testPersistTelemetry)
}

func testPersistTelemetry(t *testing.T) {
	ctx := context.Background()
	mem := newMemStorage()
	cfg := testConfig()
	cfg.Storage = &testStorageID
	conn, tel, _ := newTelemetryConnector(t, cfg, consumertest.NewNop())
	conn.id = testConnectorID
	mem.client.put(`{"version":1,"data":{"rules":{}}}`)
	require.NoError(t, conn.Start(ctx, mem.host()))
	got := collect(t, tel)
	assert.Equal(t, map[string]int64{"": 0}, got[metricStateSaves])
	assert.Equal(t, map[string]int64{"": 0}, got[metricStateSaveErrors])
	assert.Equal(t, map[string]int64{"": 0}, got[metricStateSaveRejected])
	assert.Equal(t, map[string]int64{"": int64(len(`{"version":1,"data":{"rules":{}}}`))}, got[metricStateSavedBytes], "the loaded record")

	require.NoError(t, conn.ConsumeLogs(ctx, podLogs("u1", base, "app started version=1")))
	require.NoError(t, conn.store.SaveNow(ctx))
	mem.client.failSets(errors.New("disk full"))
	require.Error(t, conn.store.SaveNow(ctx))
	got = collect(t, tel)
	assert.Equal(t, map[string]int64{"": 1}, got[metricStateSaves])
	assert.Equal(t, map[string]int64{"": 1}, got[metricStateSaveErrors])
	assert.Equal(t, map[string]int64{"": 0}, got[metricStateSaveRejected])
	assert.Equal(t, map[string]int64{"": int64(len(mem.client.raw()))}, got[metricStateSavedBytes])

	// Shutdown unregisters the store instruments.
	mem.client.failSets(nil)
	require.NoError(t, conn.Shutdown(ctx))
	got = collect(t, tel)
	for _, name := range []string{metricStateSaves, metricStateSaveErrors, metricStateSaveRejected, metricStateSavedBytes} {
		assert.NotContains(t, got, name)
	}
}

func TestTelemetryRegisterStore(t *testing.T) {
	conn, tel, _ := newTelemetryConnector(t, testConfig(), consumertest.NewNop())
	got := collect(t, tel)
	assert.NotContains(t, got, metricStateSaves, "no store instruments without storage")

	require.NoError(t, conn.tel.registerStore(func() statestore.Stats {
		return statestore.Stats{Saves: 1, SaveErrors: 2, Rejected: 3, LastBytes: 4}
	}))
	got = collect(t, tel)
	assert.Equal(t, map[string]int64{"": 1}, got[metricStateSaves])
	assert.Equal(t, map[string]int64{"": 2}, got[metricStateSaveErrors])
	assert.Equal(t, map[string]int64{"": 3}, got[metricStateSaveRejected])
	assert.Equal(t, map[string]int64{"": 4}, got[metricStateSavedBytes])
}

// ctxConsumer blocks every ConsumeMetrics call until its context ends, then
// returns the context error.
type ctxConsumer struct {
	mu sync.Mutex
	n  int
}

func (c *ctxConsumer) ConsumeMetrics(ctx context.Context, _ pmetric.Metrics) error {
	c.mu.Lock()
	c.n++
	c.mu.Unlock()
	<-ctx.Done()
	return ctx.Err()
}

func (*ctxConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (c *ctxConsumer) calls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

func TestClampInt64(t *testing.T) {
	assert.Equal(t, int64(0), clampInt64(0))
	assert.Equal(t, int64(math.MaxInt64), clampInt64(math.MaxInt64))
	assert.Equal(t, int64(math.MaxInt64), clampInt64(math.MaxInt64+1))
	assert.Equal(t, int64(math.MaxInt64), clampInt64(math.MaxUint64))
}
