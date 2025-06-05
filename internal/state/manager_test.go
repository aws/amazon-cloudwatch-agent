// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package state

import (
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
				assert.Equal(t, testCase.cfg.Name, got.name)
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
		assert.Empty(t, got)
	})
	t.Run("Restore/Invalid", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		manager := NewFileRangeManager(ManagerConfig{StateFileDir: tmpDir, Name: "missing.log"})
		assert.NoError(t, os.WriteFile(manager.(*rangeManager).stateFilePath, []byte("invalid"), FileMode))
		got, err := manager.Restore()
		assert.Error(t, err)
		assert.Nil(t, got)
	})
	t.Run("Enqueue/Multiple", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		manager := NewFileRangeManager(ManagerConfig{StateFileDir: tmpDir, Name: "overwrite.log"})

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
	t.Run("Run/Notification/Delete", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		cfg := ManagerConfig{StateFileDir: tmpDir, Name: "delete.log"}
		manager := NewFileRangeManager(cfg).(*rangeManager)

		tree := newRangeTree()
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
		manager := NewFileRangeManager(ManagerConfig{StateFileDir: tmpDir, Name: "test.log", SaveInterval: time.Hour})

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
