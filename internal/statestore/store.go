// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package statestore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.uber.org/zap"
)

const (
	defaultDebounce  = time.Second
	defaultIdleFlush = 5 * time.Minute

	// logInterval bounds how often a Store logs each class of save failure, so a storage backend that
	// keeps failing is reported once a minute rather than at the Debounce rate.
	logInterval = time.Minute
)

// ErrTooLarge is returned by SaveNow when the encoded record exceeds Options.MaxBytes. Nothing is
// written, so the previously stored record is left in place.
var ErrTooLarge = errors.New("statestore: encoded record exceeds MaxBytes")

var (
	errNotStarted = errors.New("statestore: SaveNow called before Start")
	errClosed     = errors.New("statestore: store is shut down")
)

// Options configures a Store.
type Options struct {
	// Key is the storage key the record is kept under. Required.
	Key string
	// Version is the record schema version, at least 1. Load discards a record with any other version.
	Version int
	// MaxBytes caps the encoded record size; larger records are refused with ErrTooLarge. Must be > 0.
	MaxBytes int
	// Debounce bounds how long after MarkChanged the record is saved. Marks within one interval are
	// coalesced into one save. Defaults to 1s.
	Debounce time.Duration
	// IdleFlush bounds how long after MarkTouched the record is saved when nothing changed.
	// Defaults to 5m.
	IdleFlush time.Duration
	// Logger is required.
	Logger *zap.Logger
}

// Stats are cumulative counters for one Store.
type Stats struct {
	// Saves counts records written successfully.
	Saves uint64
	// SaveErrors counts saves that failed in the snapshot, the encoding, or the storage write.
	SaveErrors uint64
	// Rejected counts saves refused with ErrTooLarge.
	Rejected uint64
	// LastBytes is the encoded size of the record most recently loaded or saved.
	LastBytes int
}

// envelope is the stored record format.
type envelope[T any] struct {
	Version int `json:"version"`
	Data    T   `json:"data"`
}

// Store keeps one versioned record of type T in a storage.Client. MarkChanged and MarkTouched are
// safe to call from any goroutine; the other methods are safe to call concurrently with each other.
type Store[T any] struct {
	client storage.Client
	opts   Options
	logger *zap.Logger

	// changed and touched are generation counters bumped by MarkChanged and MarkTouched. A save records
	// the generations it read before taking its snapshot, so a mark that races with the save leaves its
	// counter ahead of the recorded value and is picked up by the next pass.
	changed atomic.Uint64
	touched atomic.Uint64

	// saveMu serializes saves from the saver goroutine, SaveNow, and Shutdown, and guards the fields
	// below it.
	saveMu        sync.Mutex
	snapshot      func() (T, error)
	savedChanged  uint64
	savedTouched  uint64
	closed        bool
	lastErrorLog  time.Time
	lastRejectLog time.Time

	// lifeMu guards the saver lifecycle fields below it.
	lifeMu   sync.Mutex
	started  bool
	stopping bool
	cancel   context.CancelFunc
	// done is closed when run() returns, so Shutdown can wait for the saver before the final save.
	done         chan struct{}
	shutdownOnce sync.Once

	saves      atomic.Uint64
	saveErrors atomic.Uint64
	rejected   atomic.Uint64
	lastBytes  atomic.Int64
}

// New validates opts, applies the Debounce and IdleFlush defaults, and returns a Store that reads and
// writes through client. The Store owns client and closes it on Shutdown.
func New[T any](client storage.Client, opts Options) (*Store[T], error) {
	switch {
	case client == nil:
		return nil, errors.New("statestore: client is required")
	case opts.Key == "":
		return nil, errors.New("statestore: Key is required")
	case opts.Version < 1:
		return nil, fmt.Errorf("statestore: Version must be at least 1, got %d", opts.Version)
	case opts.MaxBytes <= 0:
		return nil, fmt.Errorf("statestore: MaxBytes must be positive, got %d", opts.MaxBytes)
	case opts.Debounce < 0:
		return nil, fmt.Errorf("statestore: Debounce must not be negative, got %s", opts.Debounce)
	case opts.IdleFlush < 0:
		return nil, fmt.Errorf("statestore: IdleFlush must not be negative, got %s", opts.IdleFlush)
	case opts.Logger == nil:
		return nil, errors.New("statestore: Logger is required")
	}
	if opts.Debounce == 0 {
		opts.Debounce = defaultDebounce
	}
	if opts.IdleFlush == 0 {
		opts.IdleFlush = defaultIdleFlush
	}
	return &Store[T]{
		client: client,
		opts:   opts,
		logger: opts.Logger.With(zap.String("key", opts.Key)),
	}, nil
}

// Load reads the stored record. A missing record returns found=false with no error. A record that
// does not decode, or carries a version other than Options.Version, is logged and also returns
// found=false with no error, so the caller starts fresh. Only a storage read failure is an error.
func (s *Store[T]) Load(ctx context.Context) (T, bool, error) {
	var zero T
	b, err := s.client.Get(ctx, s.opts.Key)
	if err != nil {
		return zero, false, fmt.Errorf("statestore: get %q: %w", s.opts.Key, err)
	}
	if len(b) == 0 {
		return zero, false, nil
	}
	data, err := s.decode(b)
	if err != nil {
		s.logger.Warn("discarding unreadable stored state", zap.Error(err), zap.Int("bytes", len(b)))
		return zero, false, nil
	}
	s.lastBytes.Store(int64(len(b)))
	return data, true, nil
}

func (s *Store[T]) decode(b []byte) (T, error) {
	var zero T
	// Check the version before decoding data, so a record from another schema version is reported as
	// such rather than as corrupt.
	var raw struct {
		Version int             `json:"version"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return zero, fmt.Errorf("corrupt record: %w", err)
	}
	if raw.Version != s.opts.Version {
		return zero, fmt.Errorf("unknown record version %d, want %d", raw.Version, s.opts.Version)
	}
	if len(raw.Data) == 0 {
		return zero, errors.New("corrupt record: missing data")
	}
	var data T
	if err := json.Unmarshal(raw.Data, &data); err != nil {
		return zero, fmt.Errorf("corrupt record data: %w", err)
	}
	return data, nil
}

// MarkChanged records that the state changed. The saver writes it within Debounce.
func (s *Store[T]) MarkChanged() {
	s.changed.Add(1)
}

// MarkTouched records that the state is still current and worth refreshing in storage. If nothing
// changed, the saver writes it within IdleFlush.
func (s *Store[T]) MarkTouched() {
	s.touched.Add(1)
}

// Start sets the snapshot function and starts the saver goroutine. snapshot must return a value the
// Store may encode without further synchronization. Marks made before Start are saved on the first
// pass. Start must be called at most once; later calls, and calls after Shutdown, are ignored.
func (s *Store[T]) Start(snapshot func() (T, error)) {
	s.lifeMu.Lock()
	defer s.lifeMu.Unlock()
	if snapshot == nil || s.started || s.stopping {
		s.logger.Warn("ignoring state store Start",
			zap.Bool("nil_snapshot", snapshot == nil), zap.Bool("started", s.started), zap.Bool("shut_down", s.stopping))
		return
	}
	s.started = true

	s.saveMu.Lock()
	s.snapshot = snapshot
	s.saveMu.Unlock()

	ctx, cancel := context.WithCancel(context.Background()) //nolint:gosec // G118: Shutdown calls s.cancel.
	s.cancel = cancel
	s.done = make(chan struct{})
	go s.run(ctx, s.done)
}

// run saves changed state on every Debounce tick and touched-only state on every IdleFlush tick, so a
// mark is written within one interval and a burst of marks becomes one write.
func (s *Store[T]) run(ctx context.Context, done chan struct{}) {
	defer close(done)
	debounce := time.NewTicker(s.opts.Debounce)
	defer debounce.Stop()
	idle := time.NewTicker(s.opts.IdleFlush)
	defer idle.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-debounce.C:
			_ = s.flush(ctx, true)
		case <-idle.C:
			_ = s.flush(ctx, false)
		}
	}
}

// SaveNow writes the record at once, not at the next pass of the saver. Use it when the record must
// be in storage before the next step, for example before a planned stop or in tests. It writes the
// record even when no mark is pending. It returns an error if Start has not been called, ErrTooLarge
// (wrapped) if the record exceeds MaxBytes, or the snapshot or storage error. A snapshot or storage
// failure leaves the marks pending so the saver retries.
func (s *Store[T]) SaveNow(ctx context.Context) error {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	return s.saveLocked(ctx)
}

// flush saves only if a mark is pending. With changedOnly set, a MarkTouched-only state is left for
// the IdleFlush pass.
func (s *Store[T]) flush(ctx context.Context, changedOnly bool) error {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	return s.flushLocked(ctx, changedOnly)
}

func (s *Store[T]) flushLocked(ctx context.Context, changedOnly bool) error {
	changed := s.changed.Load() != s.savedChanged
	touched := s.touched.Load() != s.savedTouched
	if !changed && (changedOnly || !touched) {
		return nil
	}
	return s.saveLocked(ctx)
}

func (s *Store[T]) saveLocked(ctx context.Context) error {
	if s.closed {
		return errClosed
	}
	if s.snapshot == nil {
		return errNotStarted
	}
	// Read the generations before the snapshot: a mark made after this point may not be reflected in
	// the snapshot, so it must stay pending.
	changedGen, touchedGen := s.changed.Load(), s.touched.Load()

	data, err := s.snapshot()
	if err != nil {
		s.saveErrors.Add(1)
		s.logSaveError("unable to snapshot state; will retry", err)
		return fmt.Errorf("statestore: snapshot: %w", err)
	}
	b, err := json.Marshal(envelope[T]{Version: s.opts.Version, Data: data})
	if err != nil {
		// Encoding the same snapshot again would fail the same way, so drop these marks and wait for
		// the next change instead of retrying every Debounce.
		s.recordSaved(changedGen, touchedGen)
		s.saveErrors.Add(1)
		s.logSaveError("unable to encode state; waiting for the next change", err)
		return fmt.Errorf("statestore: encode: %w", err)
	}
	if len(b) > s.opts.MaxBytes {
		// Same reasoning as an encode failure: the stored record stays as it was until a smaller
		// snapshot comes along with the next mark.
		s.recordSaved(changedGen, touchedGen)
		s.rejected.Add(1)
		s.logRejected(len(b))
		return fmt.Errorf("%w: %d bytes, limit %d", ErrTooLarge, len(b), s.opts.MaxBytes)
	}
	if err := s.client.Set(ctx, s.opts.Key, b); err != nil {
		s.saveErrors.Add(1)
		s.logSaveError("unable to save state; will retry", err)
		return fmt.Errorf("statestore: set %q: %w", s.opts.Key, err)
	}
	s.recordSaved(changedGen, touchedGen)
	s.saves.Add(1)
	s.lastBytes.Store(int64(len(b)))
	return nil
}

// recordSaved clears the marks made up to the given generations. Called with saveMu held.
func (s *Store[T]) recordSaved(changedGen, touchedGen uint64) {
	s.savedChanged = changedGen
	s.savedTouched = touchedGen
}

// logSaveError logs at most once per logInterval. Called with saveMu held.
func (s *Store[T]) logSaveError(msg string, err error) {
	now := time.Now()
	if !s.lastErrorLog.IsZero() && now.Sub(s.lastErrorLog) < logInterval {
		return
	}
	s.lastErrorLog = now
	s.logger.Error(msg, zap.Error(err), zap.Uint64("save_errors", s.saveErrors.Load()))
}

// logRejected logs at most once per logInterval. Called with saveMu held.
func (s *Store[T]) logRejected(size int) {
	now := time.Now()
	if !s.lastRejectLog.IsZero() && now.Sub(s.lastRejectLog) < logInterval {
		return
	}
	s.lastRejectLog = now
	s.logger.Warn("state record too large; not saved",
		zap.Int("bytes", size), zap.Int("max_bytes", s.opts.MaxBytes), zap.Uint64("rejected", s.rejected.Load()))
}

// Shutdown stops the saver, writes the record if any mark is pending, and closes the client. Errors
// from the final save and the close are joined. It is safe to call without Start, and calls after
// the first return nil.
//
// If ctx expires before the saver stops, the saver may still be inside the snapshot or a storage
// write, so Shutdown returns ctx's error without the final save and leaves the client open rather
// than closing it underneath that write.
func (s *Store[T]) Shutdown(ctx context.Context) error {
	var err error
	s.shutdownOnce.Do(func() {
		err = s.shutdown(ctx)
	})
	return err
}

func (s *Store[T]) shutdown(ctx context.Context) error {
	s.lifeMu.Lock()
	s.stopping = true
	started, cancel, done := s.started, s.cancel, s.done
	s.lifeMu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
			return fmt.Errorf("statestore: waiting for the saver to stop: %w", ctx.Err())
		}
	}

	var errs []error
	s.saveMu.Lock()
	// Without Start there is no snapshot to save, whatever was marked.
	if started {
		if err := s.flushLocked(ctx, false); err != nil {
			errs = append(errs, fmt.Errorf("statestore: final save: %w", err))
		}
	}
	// Mark closed before releasing saveMu so no SaveNow can reach the client once it is closing.
	s.closed = true
	s.saveMu.Unlock()

	if err := s.client.Close(ctx); err != nil {
		errs = append(errs, fmt.Errorf("statestore: close: %w", err))
	}
	return errors.Join(errs...)
}

// Stats returns the Store's cumulative counters.
func (s *Store[T]) Stats() Stats {
	return Stats{
		Saves:      s.saves.Load(),
		SaveErrors: s.saveErrors.Load(),
		Rejected:   s.rejected.Load(),
		LastBytes:  int(s.lastBytes.Load()),
	}
}
