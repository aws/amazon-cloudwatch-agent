// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logreplayconnector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.uber.org/zap"
)

// A change of the form of stateSnapshot needs a new stateRecordVersion.
const (
	stateRecordKey     = "state"
	stateRecordVersion = 1
	saveInterval       = time.Second
)

var (
	errExtensionNotFound   = errors.New("storage extension not found")
	errNotStorageExtension = errors.New("extension is not a storage extension")
)

type storedRecord struct {
	Version int           `json:"version"`
	Data    stateSnapshot `json:"data"`
}

func getStorageClient(ctx context.Context, host component.Host, storageID, id component.ID) (storage.Client, error) {
	ext, ok := host.GetExtensions()[storageID]
	if !ok {
		return nil, fmt.Errorf("%w: %q", errExtensionNotFound, storageID.String())
	}
	se, ok := ext.(storage.Extension)
	if !ok {
		return nil, fmt.Errorf("%w: %q", errNotStorageExtension, storageID.String())
	}
	// The type without underscores keeps the same storage file if the type is
	// renamed (https://github.com/open-telemetry/opentelemetry-collector/issues/14988).
	clientID := component.MustNewIDWithName(strings.ReplaceAll(id.Type().String(), "_", ""), id.Name())
	client, err := se.GetClient(ctx, component.KindConnector, clientID, "")
	if err != nil {
		return nil, fmt.Errorf("storage extension %q cannot give a client: %w", storageID.String(), err)
	}
	if client == nil {
		return nil, fmt.Errorf("storage extension %q gave no client", storageID.String())
	}
	return client, nil
}

type stateStore struct {
	client   storage.Client
	logger   *zap.Logger
	snapshot func() stateSnapshot

	pending  atomic.Bool
	cancel   context.CancelFunc
	done     chan struct{}
	stopOnce sync.Once
	errLog   logLimiter
}

func newStateStore(client storage.Client, logger *zap.Logger) *stateStore {
	return &stateStore{client: client, logger: logger}
}

// load gives found false for a missing or corrupt record, or one of another version. Only a read
// error is an error.
func (s *stateStore) load(ctx context.Context) (stateSnapshot, bool, error) {
	b, err := s.client.Get(ctx, stateRecordKey)
	if err != nil {
		return stateSnapshot{}, false, err
	}
	if len(b) == 0 {
		return stateSnapshot{}, false, nil
	}
	snap, err := decodeStoredRecord(b)
	if err != nil {
		s.logger.Warn("Discarding the stored log_replay state", zap.Error(err), zap.Int("bytes", len(b)))
		return stateSnapshot{}, false, nil
	}
	return snap, true, nil
}

func decodeStoredRecord(b []byte) (stateSnapshot, error) {
	var rec struct {
		Version int             `json:"version"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(b, &rec); err != nil {
		return stateSnapshot{}, fmt.Errorf("corrupt record: %w", err)
	}
	if rec.Version != stateRecordVersion {
		return stateSnapshot{}, fmt.Errorf("unknown record version %d, want %d", rec.Version, stateRecordVersion)
	}
	if len(rec.Data) == 0 {
		return stateSnapshot{}, errors.New("corrupt record: missing data")
	}
	var snap stateSnapshot
	if err := json.Unmarshal(rec.Data, &snap); err != nil {
		return stateSnapshot{}, fmt.Errorf("corrupt record data: %w", err)
	}
	return snap, nil
}

// start starts the saver. It must be called at most once.
func (s *stateStore) start(snapshot func() stateSnapshot) {
	s.snapshot = snapshot
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel, s.done = cancel, make(chan struct{})
	go s.run(ctx)
}

func (s *stateStore) markChanged() {
	s.pending.Store(true)
}

func (s *stateStore) run(ctx context.Context) {
	defer close(s.done)
	ticker := time.NewTicker(saveInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = s.save(ctx)
		}
	}
}

// save clears the pending flag before the snapshot, so that a change during the write is written
// at the next pass.
func (s *stateStore) save(ctx context.Context) error {
	if !s.pending.Swap(false) {
		return nil
	}
	b, err := json.Marshal(storedRecord{Version: stateRecordVersion, Data: s.snapshot()})
	if err != nil {
		// The same state fails the same way, so this waits for the next change.
		s.saveFailed("Failed to encode the log_replay state", err)
		return err
	}
	if err := s.client.Set(ctx, stateRecordKey, b); err != nil {
		s.pending.Store(true)
		s.saveFailed("Failed to save the log_replay state", err)
		return err
	}
	return nil
}

func (s *stateStore) saveFailed(msg string, err error) {
	if suppressed, ok := s.errLog.allow(time.Now().UnixNano(), 1); ok {
		s.logger.Error(msg, zap.Error(err), zap.Int64("suppressed_since_last_log", suppressed))
	}
}

// If ctx ends before the saver stops, the saver can still be writing, so shutdown leaves the
// client open.
func (s *stateStore) shutdown(ctx context.Context) error {
	var err error
	s.stopOnce.Do(func() { err = s.stop(ctx) })
	return err
}

func (s *stateStore) stop(ctx context.Context) error {
	var errs error
	if s.cancel != nil {
		s.cancel()
		select {
		case <-s.done:
		case <-ctx.Done():
			return fmt.Errorf("waiting for the state saver to stop: %w", ctx.Err())
		}
		if err := s.save(ctx); err != nil {
			errs = fmt.Errorf("final save of the state: %w", err)
		}
	}
	return errors.Join(errs, s.client.Close(ctx))
}
