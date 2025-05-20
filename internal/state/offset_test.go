// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package state

import (
	"math"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFileOffset(t *testing.T) {
	t.Run("NewOffset", func(t *testing.T) {
		t.Parallel()
		offset := NewFileOffset(0)
		assert.Zero(t, offset.Get())
	})
	t.Run("GetSet", func(t *testing.T) {
		t.Parallel()
		offset := NewFileOffset(math.MaxUint64)
		assert.Equal(t, uint64(math.MaxUint64), offset.Get())
		assert.Zero(t, offset.GetInt64())
		assert.EqualValues(t, 0, offset.seq)
		offset.SetInt64(50)
		assert.EqualValues(t, 50, offset.Get())
		assert.EqualValues(t, 50, offset.GetInt64())
		assert.EqualValues(t, 1, offset.seq)
		offset.SetInt64(-10)
		assert.EqualValues(t, 50, offset.Get())
		assert.EqualValues(t, 50, offset.GetInt64())
		assert.EqualValues(t, 1, offset.seq)
	})
	t.Run("Unmarshal/Loop", func(t *testing.T) {
		t.Parallel()
		offset := NewFileOffset(100)

		data, err := offset.MarshalText()
		assert.NoError(t, err)

		var restored FileOffset
		assert.NoError(t, restored.UnmarshalText(data))
		assert.EqualValues(t, offset.Get(), restored.Get())
	})
	t.Run("Unmarshal/Invalid", func(t *testing.T) {
		t.Parallel()
		var offset FileOffset
		assert.Error(t, offset.UnmarshalText([]byte("test")))
		assert.Zero(t, offset.Get())
	})
	t.Run("Compare", func(t *testing.T) {
		t.Parallel()
		testCases := map[string]struct {
			a, b FileOffset
			want int
		}{
			"Equal": {
				a:    FileOffset{seq: 1, offset: 100},
				b:    FileOffset{seq: 1, offset: 100},
				want: 0,
			},
			"Offset/Greater": {
				a:    FileOffset{seq: 1, offset: 200},
				b:    FileOffset{seq: 1, offset: 100},
				want: 1,
			},
			"Offset/Lesser": {
				a:    FileOffset{seq: 1, offset: 100},
				b:    FileOffset{seq: 1, offset: 200},
				want: -1,
			},
			"Sequence/Greater": {
				a:    FileOffset{seq: 2, offset: 100},
				b:    FileOffset{seq: 1, offset: 200},
				want: 1,
			},
			"Sequence/Lesser": {
				a:    FileOffset{seq: 1, offset: 200},
				b:    FileOffset{seq: 2, offset: 100},
				want: -1,
			},
		}
		for name, testCase := range testCases {
			t.Run(name, func(t *testing.T) {
				assert.Equal(t, testCase.want, testCase.a.Compare(testCase.b))
			})
		}
	})
}

func TestFileOffsetManager(t *testing.T) {
	t.Run("New", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		testCases := map[string]struct {
			cfg              ManagerConfig
			wantFilePath     string
			wantQueueSize    int
			wantSaveInterval time.Duration
		}{
			"ValidConfig": {
				cfg: ManagerConfig{
					StateFileDir:    tmpDir,
					StateFilePrefix: "test_prefix_",
					Name:            "valid.log",
					QueueSize:       10,
					SaveInterval:    time.Millisecond,
				},
				wantFilePath:     filepath.Join(tmpDir, "test_prefix_valid.log"),
				wantQueueSize:    10,
				wantSaveInterval: time.Millisecond,
			},
			"InvalidConfig": {
				cfg: ManagerConfig{
					StateFileDir:    "",
					StateFilePrefix: "test_prefix_",
					Name:            "valid.log",
					QueueSize:       -1,
					SaveInterval:    0,
				},
				wantFilePath:     "",
				wantQueueSize:    defaultQueueSize,
				wantSaveInterval: defaultSaveInterval,
			},
		}
		for name, testCase := range testCases {
			t.Run(name, func(t *testing.T) {
				got := NewFileOffsetManager(testCase.cfg).(*fileOffsetManager)
				assert.Equal(t, testCase.cfg.Name, got.name)
				assert.Equal(t, testCase.wantFilePath, got.stateFilePath)
				assert.Equal(t, testCase.wantQueueSize, cap(got.offsetCh))
				assert.Equal(t, testCase.wantSaveInterval, got.saveInterval)
			})
		}
	})
	t.Run("Restore/Missing", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		manager := NewFileOffsetManager(ManagerConfig{StateFileDir: tmpDir, Name: "missing.log"})
		_, err := manager.Restore()
		assert.Error(t, err)
	})
	t.Run("Enqueue/Multiple", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		manager := NewFileOffsetManager(ManagerConfig{StateFileDir: tmpDir, Name: "overwrite.log"})

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

		offset1 := NewFileOffset(100)
		manager.Enqueue(offset1)

		offset2 := NewFileOffset(200)
		manager.Enqueue(offset2)

		time.Sleep(2 * defaultSaveInterval)

		restored, err := manager.Restore()
		assert.NoError(t, err)
		assert.Equal(t, offset2.Get(), restored.Get())

		close(notification.Done)
	})
	t.Run("Run/Notification/Delete", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		cfg := ManagerConfig{StateFileDir: tmpDir, Name: "delete.log"}
		manager := NewFileOffsetManager(cfg)

		err := manager.Save(NewFileOffset(100))
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
		manager := NewFileOffsetManager(ManagerConfig{StateFileDir: tmpDir, Name: "test.log"})
		manager.(*fileOffsetManager).saveInterval = time.Hour

		notification := Notification{
			Done: make(chan struct{}),
		}

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			manager.Run(notification)
		}()

		initial := NewFileOffset(100)
		manager.Enqueue(initial)

		time.Sleep(time.Millisecond)

		close(notification.Done)
		wg.Wait()

		restored, err := manager.Restore()
		assert.NoError(t, err)
		assert.Equal(t, initial.Get(), restored.Get())
	})
	t.Run("Enqueue/QueueOverflow", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		manager := NewFileOffsetManager(ManagerConfig{
			StateFileDir: tmpDir,
			Name:         "overflow.log",
			QueueSize:    10,
		})

		notification := Notification{
			Done: make(chan struct{}),
		}

		for i := 0; i <= 20; i++ {
			offset := FileOffset{}
			offset.SetInt64(int64(i))
			manager.Enqueue(offset)
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
		assert.EqualValues(t, 20, restored.Get())
	})
}
