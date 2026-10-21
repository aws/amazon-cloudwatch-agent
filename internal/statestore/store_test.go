// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package statestore

import (
	"context"
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

const testKey = "state"

// fakeClient is an in-memory storage.Client with error injection.
type fakeClient struct {
	mu       sync.Mutex
	data     map[string][]byte
	sets     int
	closes   int
	getErr   error
	setErr   error
	closeErr error
}

var _ storage.Client = (*fakeClient)(nil)

func newFakeClient() *fakeClient {
	return &fakeClient{data: map[string][]byte{}}
}

func (c *fakeClient) Get(_ context.Context, key string) ([]byte, error) {
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

func (c *fakeClient) Set(_ context.Context, key string, value []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.setErr != nil {
		return c.setErr
	}
	c.sets++
	c.data[key] = append([]byte(nil), value...)
	return nil
}

func (c *fakeClient) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
	return nil
}

func (c *fakeClient) Batch(ctx context.Context, ops ...*storage.Operation) error {
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

func (c *fakeClient) Close(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closes++
	return c.closeErr
}

func (c *fakeClient) put(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = []byte(value)
}

func (c *fakeClient) stored(key string) []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.data[key]
}

func (c *fakeClient) setCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sets
}

func (c *fakeClient) closeCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closes
}

func (c *fakeClient) failSets(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setErr = err
}

type record struct {
	Pods  map[string]string `json:"pods"`
	Ratio float64           `json:"ratio,omitempty"`
}

func testOptions(logger *zap.Logger) Options {
	return Options{Key: testKey, Version: 1, MaxBytes: 1 << 10, Logger: logger}
}

// newTestStore returns a Store whose saver effectively never ticks unless mutate sets Debounce or
// IdleFlush, so tests that drive saves directly are not raced by a real-time tick.
func newTestStore(t *testing.T, c storage.Client, mutate func(*Options)) (*Store[record], *observer.ObservedLogs) {
	t.Helper()
	core, logs := observer.New(zapcore.InfoLevel)
	opts := testOptions(zap.New(core))
	opts.Debounce = time.Hour
	opts.IdleFlush = time.Hour
	if mutate != nil {
		mutate(&opts)
	}
	s, err := New[record](c, opts)
	require.NoError(t, err)
	return s, logs
}

func everySecond(o *Options) {
	o.Debounce = time.Second
}

func constSnapshot(r record) func() (record, error) {
	return func() (record, error) { return r, nil }
}

func TestNewValidatesOptions(t *testing.T) {
	logger := zap.NewNop()
	tests := map[string]struct {
		client storage.Client
		mutate func(*Options)
	}{
		"nil client":         {client: nil, mutate: func(*Options) {}},
		"empty key":          {client: newFakeClient(), mutate: func(o *Options) { o.Key = "" }},
		"zero version":       {client: newFakeClient(), mutate: func(o *Options) { o.Version = 0 }},
		"zero max bytes":     {client: newFakeClient(), mutate: func(o *Options) { o.MaxBytes = 0 }},
		"negative debounce":  {client: newFakeClient(), mutate: func(o *Options) { o.Debounce = -time.Second }},
		"negative idleflush": {client: newFakeClient(), mutate: func(o *Options) { o.IdleFlush = -time.Second }},
		"nil logger":         {client: newFakeClient(), mutate: func(o *Options) { o.Logger = nil }},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			opts := testOptions(logger)
			tc.mutate(&opts)
			s, err := New[record](tc.client, opts)
			assert.Error(t, err)
			assert.Nil(t, s)
		})
	}

	t.Run("defaults", func(t *testing.T) {
		s, err := New[record](newFakeClient(), testOptions(logger))
		require.NoError(t, err)
		assert.Equal(t, defaultDebounce, s.opts.Debounce)
		assert.Equal(t, defaultIdleFlush, s.opts.IdleFlush)
	})
}

func TestLoadMissing(t *testing.T) {
	s, logs := newTestStore(t, newFakeClient(), nil)
	got, found, err := s.Load(context.Background())
	require.NoError(t, err)
	assert.False(t, found)
	assert.Zero(t, got)
	assert.Equal(t, Stats{}, s.Stats())
	assert.Zero(t, logs.Len())
}

func TestLoadRoundTrip(t *testing.T) {
	c := newFakeClient()
	want := record{Pods: map[string]string{"ns/pod-a": "uid-1", "ns/pod-b": "uid-2"}}

	writer, _ := newTestStore(t, c, nil)
	writer.Start(constSnapshot(want))
	require.NoError(t, writer.SaveNow(context.Background()))
	assert.JSONEq(t, `{"version":1,"data":{"pods":{"ns/pod-a":"uid-1","ns/pod-b":"uid-2"}}}`, string(c.stored(testKey)))

	reader, _ := newTestStore(t, c, nil)
	got, found, err := reader.Load(context.Background())
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, want, got)
	assert.Equal(t, Stats{LastBytes: len(c.stored(testKey))}, reader.Stats())

	require.NoError(t, writer.Shutdown(context.Background()))
}

func TestLoadVersionMismatch(t *testing.T) {
	c := newFakeClient()
	c.put(testKey, `{"version":2,"data":{"pods":{"a":"b"}}}`)
	s, logs := newTestStore(t, c, nil)

	got, found, err := s.Load(context.Background())
	require.NoError(t, err)
	assert.False(t, found)
	assert.Zero(t, got)
	assertDiscardLogged(t, logs, "unknown record version 2, want 1")
}

func TestLoadCorrupt(t *testing.T) {
	tests := map[string]struct {
		stored string
		reason string
	}{
		"not json":           {stored: `not json`, reason: "corrupt record"},
		"truncated":          {stored: `{"version":1,"data":{"pods":`, reason: "corrupt record"},
		"missing data":       {stored: `{"version":1}`, reason: "corrupt record: missing data"},
		"missing version":    {stored: `{"data":{}}`, reason: "unknown record version 0, want 1"},
		"data of wrong type": {stored: `{"version":1,"data":{"pods":["a"]}}`, reason: "corrupt record data"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			c := newFakeClient()
			c.put(testKey, tc.stored)
			s, logs := newTestStore(t, c, nil)

			got, found, err := s.Load(context.Background())
			require.NoError(t, err)
			assert.False(t, found)
			assert.Zero(t, got)
			assert.Equal(t, Stats{}, s.Stats())
			assertDiscardLogged(t, logs, tc.reason)
		})
	}
}

func assertDiscardLogged(t *testing.T, logs *observer.ObservedLogs, reason string) {
	t.Helper()
	entries := logs.FilterLevelExact(zapcore.WarnLevel).All()
	require.Len(t, entries, 1)
	fields := entries[0].ContextMap()
	assert.Equal(t, testKey, fields["key"])
	assert.Contains(t, fields["error"], reason)
}

func TestLoadGetError(t *testing.T) {
	c := newFakeClient()
	c.getErr = errors.New("disk on fire")
	s, _ := newTestStore(t, c, nil)

	got, found, err := s.Load(context.Background())
	assert.ErrorIs(t, err, c.getErr)
	assert.False(t, found)
	assert.Zero(t, got)
}

func TestSaveNowBeforeStart(t *testing.T) {
	c := newFakeClient()
	s, _ := newTestStore(t, c, nil)
	assert.ErrorIs(t, s.SaveNow(context.Background()), errNotStarted)
	assert.Zero(t, c.setCount())
}

func TestSaveNowSuccess(t *testing.T) {
	c := newFakeClient()
	s, _ := newTestStore(t, c, nil)
	s.Start(constSnapshot(record{Pods: map[string]string{"a": "b"}}))
	s.MarkChanged()
	s.MarkTouched()

	require.NoError(t, s.SaveNow(context.Background()))
	assert.Equal(t, 1, c.setCount())
	assert.Equal(t, Stats{Saves: 1, LastBytes: len(c.stored(testKey))}, s.Stats())

	// Both marks were covered by that save, so a flush has nothing to do.
	require.NoError(t, s.flush(context.Background(), false))
	assert.Equal(t, 1, c.setCount())

	require.NoError(t, s.Shutdown(context.Background()))
}

func TestSaveNowTooLarge(t *testing.T) {
	c := newFakeClient()
	c.put(testKey, `{"version":1,"data":{"pods":{}}}`)
	before := c.stored(testKey)
	s, logs := newTestStore(t, c, func(o *Options) { o.MaxBytes = 32 })
	s.Start(constSnapshot(record{Pods: map[string]string{"ns/a-very-long-pod-name": "uid"}}))
	s.MarkChanged()

	err := s.SaveNow(context.Background())
	require.ErrorIs(t, err, ErrTooLarge)
	assert.Equal(t, before, c.stored(testKey), "the previous record must be left in place")
	assert.Zero(t, c.setCount())
	assert.Equal(t, uint64(1), s.Stats().Rejected)

	// Retrying the same snapshot cannot help, so the mark is dropped rather than retried every tick.
	require.NoError(t, s.flush(context.Background(), false))
	assert.Equal(t, uint64(1), s.Stats().Rejected)

	// A second rejection within logInterval is counted but not logged.
	require.ErrorIs(t, s.SaveNow(context.Background()), ErrTooLarge)
	assert.Equal(t, uint64(2), s.Stats().Rejected)
	warns := logs.FilterMessage("state record too large; not saved").All()
	require.Len(t, warns, 1)
	assert.Equal(t, zapcore.WarnLevel, warns[0].Level)
	assert.Equal(t, testKey, warns[0].ContextMap()["key"])

	require.NoError(t, s.Shutdown(context.Background()))
}

func TestSaveNowEncodeError(t *testing.T) {
	c := newFakeClient()
	s, _ := newTestStore(t, c, nil)
	s.Start(constSnapshot(record{Ratio: math.NaN()}))
	s.MarkChanged()

	assert.ErrorContains(t, s.SaveNow(context.Background()), "statestore: encode")
	assert.Zero(t, c.setCount())
	assert.Equal(t, uint64(1), s.Stats().SaveErrors)
	// Like ErrTooLarge, the mark is dropped instead of re-encoding the same snapshot every tick.
	require.NoError(t, s.flush(context.Background(), false))
	assert.Equal(t, uint64(1), s.Stats().SaveErrors)

	require.NoError(t, s.Shutdown(context.Background()))
}

func TestSaveNowSnapshotErrorKeepsMarks(t *testing.T) {
	c := newFakeClient()
	s, _ := newTestStore(t, c, nil)
	snapErr := errors.New("not ready")
	var fail atomic.Bool
	fail.Store(true)
	s.Start(func() (record, error) {
		if fail.Load() {
			return record{}, snapErr
		}
		return record{}, nil
	})
	s.MarkChanged()

	assert.ErrorIs(t, s.SaveNow(context.Background()), snapErr)
	fail.Store(false)
	require.NoError(t, s.flush(context.Background(), true))
	assert.Equal(t, 1, c.setCount(), "the mark must survive a snapshot failure")

	require.NoError(t, s.Shutdown(context.Background()))
}

func TestSaveNowSetErrorThenRetry(t *testing.T) {
	c := newFakeClient()
	s, logs := newTestStore(t, c, nil)
	s.Start(constSnapshot(record{Pods: map[string]string{"a": "b"}}))
	s.MarkChanged()

	setErr := errors.New("disk full")
	c.failSets(setErr)
	require.ErrorIs(t, s.SaveNow(context.Background()), setErr)
	require.ErrorIs(t, s.flush(context.Background(), true), setErr, "the mark must still be pending")
	assert.Equal(t, uint64(2), s.Stats().SaveErrors)
	assert.Nil(t, c.stored(testKey))

	errorLogs := logs.FilterLevelExact(zapcore.ErrorLevel).All()
	require.Len(t, errorLogs, 1, "repeated failures within logInterval are logged once")
	assert.Equal(t, testKey, errorLogs[0].ContextMap()["key"])

	c.failSets(nil)
	require.NoError(t, s.flush(context.Background(), true))
	assert.Equal(t, 1, c.setCount())
	assert.NotNil(t, c.stored(testKey))
	require.NoError(t, s.flush(context.Background(), false))
	assert.Equal(t, 1, c.setCount(), "nothing is pending after the retry succeeds")

	require.NoError(t, s.Shutdown(context.Background()))
}

func TestSaveKeepsMarkThatRacesWithSnapshot(t *testing.T) {
	c := newFakeClient()
	s, _ := newTestStore(t, c, nil)
	var snapshots atomic.Int32
	s.Start(func() (record, error) {
		if snapshots.Add(1) == 1 {
			// A change landing while the save is in progress is not covered by this snapshot.
			s.MarkChanged()
		}
		return record{}, nil
	})
	s.MarkChanged()

	require.NoError(t, s.SaveNow(context.Background()))
	require.NoError(t, s.flush(context.Background(), true))
	assert.Equal(t, 2, c.setCount(), "the racing mark must trigger another save")
	require.NoError(t, s.flush(context.Background(), true))
	assert.Equal(t, 2, c.setCount())

	require.NoError(t, s.Shutdown(context.Background()))
}

func TestSaverDebounceCoalesces(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newFakeClient()
		s, _ := newTestStore(t, c, everySecond)
		var snapshots atomic.Int32
		s.Start(func() (record, error) {
			snapshots.Add(1)
			return record{}, nil
		})

		for range 100 {
			s.MarkChanged()
		}
		time.Sleep(999 * time.Millisecond)
		synctest.Wait()
		assert.Zero(t, c.setCount(), "no save before Debounce elapses")

		time.Sleep(time.Millisecond)
		synctest.Wait()
		assert.Equal(t, 1, c.setCount(), "100 marks coalesce into one save")

		// Marks spread across the next interval also become one save.
		for range 5 {
			s.MarkChanged()
			time.Sleep(100 * time.Millisecond)
		}
		time.Sleep(500 * time.Millisecond)
		synctest.Wait()
		assert.Equal(t, 2, c.setCount())

		// Nothing pending: the saver keeps ticking but does not write.
		time.Sleep(time.Minute)
		synctest.Wait()
		assert.Equal(t, 2, c.setCount())
		assert.Equal(t, int32(2), snapshots.Load())

		require.NoError(t, s.Shutdown(context.Background()))
		assert.Equal(t, 2, c.setCount(), "nothing pending, so no final save")
		assert.Equal(t, 1, c.closeCount())
	})
}

func TestSaverIdleFlush(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newFakeClient()
		s, _ := newTestStore(t, c, func(o *Options) {
			o.Debounce = time.Second
			o.IdleFlush = time.Minute
		})
		s.Start(constSnapshot(record{}))

		s.MarkTouched()
		time.Sleep(time.Minute - time.Millisecond)
		synctest.Wait()
		assert.Zero(t, c.setCount(), "a touch alone waits for IdleFlush, not Debounce")

		time.Sleep(time.Millisecond)
		synctest.Wait()
		assert.Equal(t, 1, c.setCount())

		// A change saves on the Debounce tick and covers a touch made alongside it.
		s.MarkTouched()
		s.MarkChanged()
		time.Sleep(time.Second)
		synctest.Wait()
		assert.Equal(t, 2, c.setCount())
		time.Sleep(2 * time.Minute)
		synctest.Wait()
		assert.Equal(t, 2, c.setCount())

		require.NoError(t, s.Shutdown(context.Background()))
	})
}

func TestSaverRetriesAfterSetError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newFakeClient()
		s, logs := newTestStore(t, c, everySecond)
		s.Start(constSnapshot(record{}))
		c.failSets(errors.New("disk full"))
		s.MarkChanged()

		// Fails on every tick for 90s: 90 errors, logged at 1s and again after logInterval at 61s.
		time.Sleep(90 * time.Second)
		synctest.Wait()
		assert.Zero(t, c.setCount())
		assert.Equal(t, uint64(90), s.Stats().SaveErrors)
		assert.Equal(t, 2, logs.FilterLevelExact(zapcore.ErrorLevel).Len())

		c.failSets(nil)
		time.Sleep(time.Second)
		synctest.Wait()
		assert.Equal(t, 1, c.setCount())
		assert.Equal(t, uint64(1), s.Stats().Saves)

		require.NoError(t, s.Shutdown(context.Background()))
	})
}

func TestShutdownFinalSave(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newFakeClient()
		s, _ := newTestStore(t, c, nil)
		s.Start(constSnapshot(record{Pods: map[string]string{"a": "b"}}))
		s.MarkChanged()

		require.NoError(t, s.Shutdown(context.Background()))
		assert.Equal(t, 1, c.setCount(), "the pending change is saved before Debounce elapses")
		assert.Equal(t, 1, c.closeCount())

		require.NoError(t, s.Shutdown(context.Background()), "second Shutdown is a no-op")
		assert.Equal(t, 1, c.setCount())
		assert.Equal(t, 1, c.closeCount())

		assert.ErrorIs(t, s.SaveNow(context.Background()), errClosed)
		s.MarkChanged()
		time.Sleep(time.Hour)
		synctest.Wait()
		assert.Equal(t, 1, c.setCount(), "the saver is stopped")
	})
}

func TestShutdownTouchedOnly(t *testing.T) {
	c := newFakeClient()
	s, _ := newTestStore(t, c, nil)
	s.Start(constSnapshot(record{}))
	s.MarkTouched()
	require.NoError(t, s.Shutdown(context.Background()))
	assert.Equal(t, 1, c.setCount())
}

func TestShutdownWithoutStart(t *testing.T) {
	c := newFakeClient()
	s, _ := newTestStore(t, c, nil)
	s.MarkChanged()

	require.NoError(t, s.Shutdown(context.Background()))
	assert.Zero(t, c.setCount(), "no snapshot function, so nothing to save")
	assert.Equal(t, 1, c.closeCount())
	require.NoError(t, s.Shutdown(context.Background()))
	assert.Equal(t, 1, c.closeCount())

	// Start after Shutdown must not start a saver against the closed client.
	s.Start(constSnapshot(record{}))
	assert.ErrorIs(t, s.SaveNow(context.Background()), errClosed)
}

func TestShutdownJoinsErrors(t *testing.T) {
	c := newFakeClient()
	setErr := errors.New("set failed")
	closeErr := errors.New("close failed")
	c.setErr = setErr
	c.closeErr = closeErr
	s, _ := newTestStore(t, c, nil)
	s.Start(constSnapshot(record{}))
	s.MarkChanged()

	err := s.Shutdown(context.Background())
	assert.ErrorIs(t, err, setErr)
	assert.ErrorIs(t, err, closeErr)
	assert.Equal(t, 1, c.closeCount())
}

func TestShutdownTimesOutWaitingForSaver(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newFakeClient()
		s, _ := newTestStore(t, c, everySecond)
		entered := make(chan struct{})
		release := make(chan struct{})
		s.Start(func() (record, error) {
			close(entered)
			<-release
			return record{}, nil
		})
		s.MarkChanged()
		time.Sleep(time.Second)
		<-entered // the saver is now blocked inside the snapshot

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := s.Shutdown(ctx)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Zero(t, c.closeCount(), "the client is not closed under an in-flight save")

		close(release)
		synctest.Wait()
	})
}

func TestStartTwiceIgnored(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newFakeClient()
		s, logs := newTestStore(t, c, everySecond)
		s.Start(constSnapshot(record{Pods: map[string]string{"first": "1"}}))
		s.Start(constSnapshot(record{Pods: map[string]string{"second": "2"}}))
		assert.Equal(t, 1, logs.FilterMessage("ignoring state store Start").Len())

		s.MarkChanged()
		time.Sleep(time.Second)
		synctest.Wait()
		assert.Equal(t, 1, c.setCount(), "only one saver is running")
		assert.Contains(t, string(c.stored(testKey)), "first")

		require.NoError(t, s.Shutdown(context.Background()))
	})
}
