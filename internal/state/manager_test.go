// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//nolint:gosec
package state

import (
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFileRangeManager(t *testing.T) {
	t.Run("New", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		testCases := map[string]struct {
			cfg                 ManagerConfig
			wantFilePath        string
			wantQueueSize       int
			wantSaveInterval    time.Duration
			wantMaxPersistItems int
		}{
			"ValidConfig": {
				cfg: ManagerConfig{
					StateFileDir:    tmpDir,
					StateFilePrefix: "test_prefix_",
					Name:            "valid.log",
					QueueSize:       10,
					SaveInterval:    time.Millisecond,
					MaxPersistItems: 5,
				},
				wantFilePath:        filepath.Join(tmpDir, "test_prefix_valid.log"),
				wantQueueSize:       10,
				wantSaveInterval:    time.Millisecond,
				wantMaxPersistItems: 5,
			},
			"InvalidConfig": {
				cfg: ManagerConfig{
					StateFileDir:    "",
					StateFilePrefix: "test_prefix_",
					Name:            "valid.log",
					QueueSize:       -1,
					SaveInterval:    0,
					MaxPersistItems: 0,
				},
				wantFilePath:        "",
				wantQueueSize:       defaultQueueSize,
				wantSaveInterval:    defaultSaveInterval,
				wantMaxPersistItems: 0,
			},
		}
		for name, testCase := range testCases {
			t.Run(name, func(t *testing.T) {
				got := NewFileRangeManager(testCase.cfg).(*rangeManager)
				assert.Equal(t, testCase.cfg.Name, got.ID())
				assert.Equal(t, testCase.wantFilePath, got.stateFilePath)
				assert.Equal(t, testCase.wantQueueSize, cap(got.queue))
				assert.Equal(t, testCase.wantSaveInterval, got.saveInterval)
				assert.Equal(t, testCase.wantMaxPersistItems, got.maxPersistItems)
			})
		}
	})
	t.Run("Restore/Missing", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		manager := NewFileRangeManager(ManagerConfig{StateFileDir: tmpDir, Name: "missing.log"})
		got, err := manager.Restore()
		assert.Error(t, err)
		assert.NotNil(t, got)
		assert.Len(t, got, 0)
		assert.EqualValues(t, 0, got.Last().EndOffset())
	})
	t.Run("Restore/Invalid", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		cfg := ManagerConfig{StateFileDir: tmpDir, Name: "invalid.log"}
		manager := NewFileRangeManager(cfg)
		assert.NoError(t, os.WriteFile(cfg.StateFilePath(), []byte("invalid"), FileMode))
		got, err := manager.Restore()
		assert.Error(t, err)
		assert.NotNil(t, got)
		assert.Len(t, got, 0)
		assert.EqualValues(t, 0, got.Last().EndOffset())
	})
	t.Run("Restore/Valid/BackwardsCompatible", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		cfg := ManagerConfig{StateFileDir: tmpDir, Name: "valid.log"}
		manager := NewFileRangeManager(cfg)
		assert.NoError(t, os.WriteFile(cfg.StateFilePath(), []byte("2760\nvalid.log"), FileMode))
		got, err := manager.Restore()
		assert.NoError(t, err)
		assert.NotNil(t, got)
		assert.Equal(t, RangeList{
			Range{start: 0, end: 2760},
		}, got)
	})
	t.Run("Restore/Valid/Ranges", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		cfg := ManagerConfig{StateFileDir: tmpDir, Name: "valid.log"}
		manager := NewFileRangeManager(cfg)
		assert.NoError(t, os.WriteFile(cfg.StateFilePath(), []byte("2760\nvalid.log\n100-1056,1640-2760"), FileMode))
		got, err := manager.Restore()
		assert.NoError(t, err)
		assert.NotNil(t, got)
		assert.Equal(t, RangeList{
			Range{start: 100, end: 1056},
			Range{start: 1640, end: 2760},
		}, got)
	})
	t.Run("Restore/ReplacesExistingTree", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		manager := NewFileRangeManager(ManagerConfig{
			StateFileDir: tmpDir,
			Name:         "replace.log",
		}).(*rangeManager)

		notification := Notification{
			Delete: make(chan struct{}),
			Done:   make(chan struct{}),
		}

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			manager.Run(notification)
		}()

		manager.Enqueue(Range{start: 0, end: 100})

		time.Sleep(2 * defaultSaveInterval)

		tree := newRangeTracker("replace.log", 10)
		tree.Insert(Range{start: 500, end: 600})
		assert.NoError(t, manager.save(tree))
		time.Sleep(2 * defaultSaveInterval)

		restored, err := manager.Restore()
		assert.NoError(t, err)
		assert.Equal(t, RangeList{
			Range{start: 500, end: 600},
		}, restored)
		time.Sleep(defaultSaveInterval)
		manager.Enqueue(Range{start: 600, end: 700})
		time.Sleep(2 * defaultSaveInterval)
		restored, err = manager.Restore()
		assert.NoError(t, err)
		assert.Equal(t, RangeList{
			Range{start: 500, end: 700},
		}, restored)

		close(notification.Done)
		wg.Wait()
	})
	t.Run("Enqueue/Merge", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		manager := NewFileRangeManager(ManagerConfig{StateFileDir: tmpDir, Name: "merge.log"})

		notification := Notification{
			Delete: make(chan struct{}),
			Done:   make(chan struct{}),
		}

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			manager.Run(notification)
		}()

		r := Range{}
		r.Shift(100)
		manager.Enqueue(r)

		r.Shift(200)
		manager.Enqueue(r)

		time.Sleep(2 * defaultSaveInterval)

		restored, err := manager.Restore()
		assert.NoError(t, err)
		assert.Equal(t, RangeList{
			Range{start: 0, end: 200},
		}, restored)

		close(notification.Done)
		wg.Wait()
	})
	t.Run("Enqueue/QueueOverflow", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		manager := NewFileRangeManager(ManagerConfig{
			StateFileDir: tmpDir,
			Name:         "overflow.log",
			QueueSize:    10,
		})

		notification := Notification{
			Done: make(chan struct{}),
		}

		r := Range{}
		for i := 0; i <= 20; i++ {
			r.ShiftInt64(int64(i))
			manager.Enqueue(r)
		}

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			manager.Run(notification)
		}()

		time.Sleep(2 * defaultSaveInterval)
		close(notification.Done)
		wg.Wait()

		restored, err := manager.Restore()
		assert.NoError(t, err)
		assert.Equal(t, RangeList{
			Range{start: 10, end: 20},
		}, restored)
	})
	t.Run("Enqueue/Concurrent", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		manager := NewFileRangeManager(ManagerConfig{
			StateFileDir: tmpDir,
			Name:         "concurrent.log",
		})

		notification := Notification{
			Done: make(chan struct{}),
		}

		var managerWg sync.WaitGroup
		managerWg.Add(1)
		go func() {
			defer managerWg.Done()
			manager.Run(notification)
		}()

		var enqueueWg sync.WaitGroup
		numThreads := uint64(50)
		rangePerThread := uint64(20)
		for i := uint64(0); i < numThreads; i++ {
			enqueueWg.Add(1)
			go func(id uint64) {
				defer enqueueWg.Done()
				start := id * rangePerThread
				rs := buildTestRanges(t, start, start+rangePerThread, 5)
				for _, r := range rs {
					manager.Enqueue(r)
					time.Sleep(time.Duration(rand.Intn(10)+5) * time.Millisecond)
				}
			}(i)
		}
		enqueueWg.Wait()

		time.Sleep(2 * defaultSaveInterval)

		close(notification.Done)
		managerWg.Wait()

		restored, err := manager.Restore()
		assert.NoError(t, err)
		assert.Equal(t, RangeList{
			Range{start: 0, end: numThreads * rangePerThread},
		}, restored)
	})
	t.Run("Run/Notification/Delete", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		cfg := ManagerConfig{StateFileDir: tmpDir, Name: "delete.log"}
		manager := NewFileRangeManager(cfg).(*rangeManager)

		tree := newRangeTracker("delete.log", 10)
		tree.Insert(Range{start: 100, end: 200})
		assert.NoError(t, manager.save(tree))
		_, err := os.Stat(cfg.StateFilePath())
		assert.NoError(t, err)

		notification := Notification{
			Delete: make(chan struct{}),
		}

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			manager.Run(notification)
		}()
		close(notification.Delete)
		wg.Wait()

		_, err = os.Stat(cfg.StateFilePath())
		assert.True(t, os.IsNotExist(err))
	})
	t.Run("Run/Notification/Done", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		manager := NewFileRangeManager(ManagerConfig{
			StateFileDir: tmpDir,
			Name:         "test.log",
			SaveInterval: time.Hour,
		})

		notification := Notification{
			Done: make(chan struct{}),
		}

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			manager.Run(notification)
		}()

		initial := Range{start: 100, end: 200}
		manager.Enqueue(initial)

		time.Sleep(time.Millisecond)

		close(notification.Done)
		wg.Wait()

		restored, err := manager.Restore()
		assert.NoError(t, err)
		assert.Equal(t, RangeList{
			Range{start: 100, end: 200},
		}, restored)
	})
	t.Run("Truncation/ClearsTree", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		manager := NewFileRangeManager(ManagerConfig{
			StateFileDir: tmpDir,
			Name:         "truncate.log",
		})

		notification := Notification{
			Done: make(chan struct{}),
		}

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			manager.Run(notification)
		}()

		r := Range{}
		r.Shift(200)
		manager.Enqueue(r)

		r.Shift(100)
		manager.Enqueue(r)

		time.Sleep(2 * defaultSaveInterval)
		close(notification.Done)
		wg.Wait()

		restored, err := manager.Restore()
		assert.NoError(t, err)
		assert.Equal(t, RangeList{
			Range{start: 0, end: 100},
		}, restored)
	})
}

type mockFileRangeQueue struct {
	enqueued []Range
}

var _ FileRangeQueue = (*mockFileRangeQueue)(nil)

func (m *mockFileRangeQueue) ID() string {
	return "mockFileRangeQueue"
}

func (m *mockFileRangeQueue) Enqueue(r Range) {
	m.enqueued = append(m.enqueued, r)
}

func TestRangeQueueBatcher(t *testing.T) {
	t.Run("NilQueue", func(t *testing.T) {
		b := NewRangeQueueBatcher(nil)
		assert.NotPanics(t, func() {
			b.Merge(Range{start: 10, end: 20})
			b.Done()
		})
	})
	t.Run("InvalidRange", func(t *testing.T) {
		q := &mockFileRangeQueue{}
		b := NewRangeQueueBatcher(q)
		b.Done()
		assert.Len(t, q.enqueued, 0)
		b.Merge(Range{})
		b.Done()
		assert.Len(t, q.enqueued, 0)
	})
	t.Run("SingleRange", func(t *testing.T) {
		q := &mockFileRangeQueue{}
		b := NewRangeQueueBatcher(q)
		b.Merge(Range{})
		b.Merge(Range{start: 10, end: 20})
		b.Merge(Range{})
		b.Done()
		assert.Len(t, q.enqueued, 1)
		assert.Equal(t, Range{start: 10, end: 20}, q.enqueued[0])
	})
	t.Run("MultipleRanges/Continuous", func(t *testing.T) {
		q := &mockFileRangeQueue{}
		b := NewRangeQueueBatcher(q)
		b.Merge(Range{start: 10, end: 20})
		b.Merge(Range{start: 20, end: 30})
		b.Merge(Range{start: 5, end: 10})
		b.Done()
		assert.Len(t, q.enqueued, 1)
		assert.Equal(t, Range{start: 5, end: 30}, q.enqueued[0])
	})
	t.Run("MultipleRanges/Distinct", func(t *testing.T) {
		q := &mockFileRangeQueue{}
		b := NewRangeQueueBatcher(q)
		b.Merge(Range{start: 100, end: 200})
		b.Merge(Range{start: 20, end: 30})
		b.Merge(Range{start: 5, end: 10})
		b.Done()
		assert.Len(t, q.enqueued, 1)
		assert.Equal(t, Range{start: 5, end: 200}, q.enqueued[0])
	})
}

func buildTestRanges(t *testing.T, start, end uint64, maxChunkSize int) RangeList {
	t.Helper()
	var chunks RangeList
	if end < start {
		return nil
	}
	current := start
	for current < end {
		remaining := end - current
		size := rand.Intn(maxChunkSize) + 1
		if uint64(size) > remaining {
			size = int(remaining)
		}
		r := Range{start: current, end: current + uint64(size)}
		chunks = append(chunks, r)
		current += uint64(size)
	}
	rand.Shuffle(len(chunks), func(i, j int) {
		chunks[i], chunks[j] = chunks[j], chunks[i]
	})
	return chunks
}
