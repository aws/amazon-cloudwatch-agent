// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logreplayconnector

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func newMemClient() *memClient {
	return &memClient{data: map[string][]byte{}}
}

func newTestStore(c storage.Client) (*stateStore, *observer.ObservedLogs) {
	core, logs := observer.New(zapcore.InfoLevel)
	return newStateStore(c, zap.New(core)), logs
}

func oneEntry(pod string) func() stateSnapshot {
	return func() stateSnapshot {
		return stateSnapshot{Rules: map[string]map[string]entry{
			"app_start": {pod: {Record: json.RawMessage(`{}`), Time: 1}},
		}}
	}
}

type nilClientStorage struct {
	component.StartFunc
	component.ShutdownFunc
}

func (nilClientStorage) GetClient(context.Context, component.Kind, component.ID, string) (storage.Client, error) {
	return nil, nil
}

func TestGetStorageClient(t *testing.T) {
	ctx := t.Context()
	_, err := getStorageClient(ctx, testHost{}, testStorageID, testConnectorID)
	assert.ErrorIs(t, err, errExtensionNotFound)

	_, err = getStorageClient(ctx, testHost{testStorageID: notStorage{}}, testStorageID, testConnectorID)
	assert.ErrorIs(t, err, errNotStorageExtension)

	m := newMemStorage()
	m.err = errors.New("permission denied")
	_, err = getStorageClient(ctx, m.host(), testStorageID, testConnectorID)
	assert.ErrorContains(t, err, "cannot give a client: permission denied")
	assert.NotErrorIs(t, err, errExtensionNotFound)

	_, err = getStorageClient(ctx, testHost{testStorageID: nilClientStorage{}}, testStorageID, testConnectorID)
	assert.ErrorContains(t, err, "gave no client")

	m = newMemStorage()
	client, err := getStorageClient(ctx, m.host(), testStorageID, testConnectorID)
	require.NoError(t, err)
	assert.Same(t, m.client, client)
	assert.Equal(t, component.KindConnector, m.kind)
	assert.Equal(t, component.MustNewIDWithName("logreplay", "vllm"), m.id)
}

// panicStorage is a storage extension whose GetClient panics.
type panicStorage struct {
	component.StartFunc
	component.ShutdownFunc
}

func (panicStorage) GetClient(context.Context, component.Kind, component.ID, string) (storage.Client, error) {
	panic("unexpected fault address")
}

func TestGetStorageClientRecoversPanic(t *testing.T) {
	_, err := getStorageClient(t.Context(), testHost{testStorageID: panicStorage{}}, testStorageID, testConnectorID)
	assert.ErrorContains(t, err, "its file can be damaged: unexpected fault address")
}

func TestStoreLoad(t *testing.T) {
	ctx := t.Context()
	c := newMemClient()
	s, logs := newTestStore(c)
	_, found, err := s.load(ctx)
	require.NoError(t, err)
	assert.False(t, found, "no record")

	s.snapshot = oneEntry("p1")
	s.markChanged()
	require.NoError(t, s.save(ctx))
	snap, found, err := s.load(ctx)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, oneEntry("p1")(), snap)

	for _, tc := range []struct {
		stored, reason string
	}{
		{`{"version":2,"data":{}}`, "unknown record version 2, want 1"},
		{`{"version":1`, "corrupt record"},
		{`{"version":1}`, "corrupt record: missing data"},
		{`{"version":1,"data":[]}`, "corrupt record data"},
	} {
		c.put([]byte(tc.stored))
		_, found, err := s.load(ctx)
		require.NoError(t, err, tc.stored)
		assert.False(t, found, tc.stored)
		warns := logs.TakeAll()
		require.Len(t, warns, 1, tc.stored)
		assert.Equal(t, "Discarding the stored log_replay state", warns[0].Message)
		assert.Contains(t, warns[0].ContextMap()["error"], tc.reason)
	}

	c.getErr = errors.New("io error")
	_, _, err = s.load(ctx)
	assert.ErrorContains(t, err, "io error")

	c.getPanic = true
	_, _, err = s.load(ctx)
	assert.ErrorContains(t, err, "its file can be damaged: unexpected fault address")
}

func TestStoreSaverCoalesces(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newMemClient()
		s, _ := newTestStore(c)
		var snapshots atomic.Int32
		s.start(func() stateSnapshot {
			snapshots.Add(1)
			return stateSnapshot{}
		})

		for range 100 {
			s.markChanged()
		}
		time.Sleep(999 * time.Millisecond)
		synctest.Wait()
		assert.Zero(t, c.setCount())
		time.Sleep(time.Millisecond)
		synctest.Wait()
		assert.Equal(t, 1, c.setCount(), "100 changes are one write")

		for range 5 {
			s.markChanged()
			time.Sleep(100 * time.Millisecond)
		}
		time.Sleep(500 * time.Millisecond)
		synctest.Wait()
		assert.Equal(t, 2, c.setCount())

		time.Sleep(time.Minute)
		synctest.Wait()
		assert.Equal(t, 2, c.setCount(), "no write without a change")
		assert.Equal(t, int32(2), snapshots.Load())

		require.NoError(t, s.shutdown(context.Background()))
		assert.Equal(t, 2, c.setCount())
		assert.Equal(t, 1, c.closes)
	})
}

func TestStoreKeepsChangeDuringSave(t *testing.T) {
	ctx := t.Context()
	c := newMemClient()
	s, _ := newTestStore(c)
	var calls atomic.Int32
	s.snapshot = func() stateSnapshot {
		if calls.Add(1) == 1 {
			s.markChanged()
		}
		return stateSnapshot{}
	}
	s.markChanged()
	require.NoError(t, s.save(ctx))
	require.NoError(t, s.save(ctx))
	assert.Equal(t, 2, c.setCount(), "a change during the write is written at the next pass")
	require.NoError(t, s.save(ctx))
	assert.Equal(t, 2, c.setCount())
}

func TestStoreRetriesAfterSetError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newMemClient()
		s, logs := newTestStore(c)
		s.start(oneEntry("p1"))
		c.mu.Lock()
		c.setErr = errors.New("disk full")
		c.mu.Unlock()
		s.markChanged()

		// One failure per second for 90 s, logged at 1 s and at 61 s.
		time.Sleep(90 * time.Second)
		synctest.Wait()
		assert.Zero(t, c.setCount())
		assert.Equal(t, 2, logs.FilterMessage("Failed to save the log_replay state").Len())

		c.mu.Lock()
		c.setErr = nil
		c.mu.Unlock()
		time.Sleep(time.Second)
		synctest.Wait()
		assert.Equal(t, 1, c.setCount())
		require.NoError(t, s.shutdown(context.Background()))
	})
}

func TestStoreEncodeErrorWaitsForNextChange(t *testing.T) {
	ctx := t.Context()
	c := newMemClient()
	s, logs := newTestStore(c)
	s.snapshot = func() stateSnapshot {
		return stateSnapshot{Rules: map[string]map[string]entry{"app_start": {"p1": {Record: json.RawMessage(`{`)}}}}
	}
	s.markChanged()
	require.Error(t, s.save(ctx))
	require.NoError(t, s.save(ctx), "the same state is not encoded again")
	assert.Zero(t, c.setCount())
	assert.Equal(t, 1, logs.FilterMessage("Failed to encode the log_replay state").Len())
}

func TestStoreShutdown(t *testing.T) {
	ctx := t.Context()
	c := newMemClient()
	s, _ := newTestStore(c)
	s.start(oneEntry("p1"))
	s.markChanged()
	require.NoError(t, s.shutdown(ctx))
	assert.Equal(t, 1, c.setCount(), "the final save writes the pending change")
	assert.Equal(t, 1, c.closes)
	require.NoError(t, s.shutdown(ctx), "a second shutdown does nothing")
	assert.Equal(t, 1, c.closes)

	// Without start there is nothing to save, but the client is closed.
	c = newMemClient()
	s, _ = newTestStore(c)
	s.markChanged()
	require.NoError(t, s.shutdown(ctx))
	assert.Zero(t, c.setCount())
	assert.Equal(t, 1, c.closes)
}

func TestStoreShutdownJoinsErrors(t *testing.T) {
	c := newMemClient()
	c.setErr = errors.New("set failed")
	c.closeErr = errors.New("close failed")
	s, _ := newTestStore(c)
	s.start(oneEntry("p1"))
	s.markChanged()
	err := s.shutdown(t.Context())
	assert.ErrorIs(t, err, c.setErr)
	assert.ErrorIs(t, err, c.closeErr)
	assert.Equal(t, 1, c.closes)
}

func TestStoreShutdownTimesOutWaitingForSaver(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newMemClient()
		s, _ := newTestStore(c)
		entered := make(chan struct{})
		release := make(chan struct{})
		s.start(func() stateSnapshot {
			close(entered)
			<-release
			return stateSnapshot{}
		})
		s.markChanged()
		time.Sleep(time.Second)
		<-entered

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		assert.ErrorIs(t, s.shutdown(ctx), context.DeadlineExceeded)
		assert.Zero(t, c.closes, "the client stays open under a write in progress")

		close(release)
		synctest.Wait()
	})
}
