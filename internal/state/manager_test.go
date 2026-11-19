// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//nolint:gosec
package state

import (
	"bytes"
	"io"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

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
					StateFileDir:      tmpDir,
					StateFilePrefix:   "test_prefix_",
					Name:              "valid.log",
					QueueSize:         10,
					SaveInterval:      time.Millisecond,
					MaxPersistedItems: 5,
				},
				wantFilePath:        filepath.Join(tmpDir, "test_prefix_valid.log"),
				wantQueueSize:       10,
				wantSaveInterval:    time.Millisecond,
				wantMaxPersistItems: 5,
			},
			"InvalidConfig": {
				cfg: ManagerConfig{
					StateFileDir:      "",
					StateFilePrefix:   "test_prefix_",
					Name:              "valid.log",
					QueueSize:         -1,
					SaveInterval:      0,
					MaxPersistedItems: 0,
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
				assert.Equal(t, testCase.wantMaxPersistItems, got.maxPersistedItems)
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
		cfg := ManagerConfig{StateFileDir: tmpDir, Name: "valid.log", MaxPersistedItems: 10}
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
			StateFileDir:      tmpDir,
			Name:              "replace.log",
			MaxPersistedItems: 10,
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
			StateFileDir:      tmpDir,
			Name:              "overflow.log",
			QueueSize:         10,
			MaxPersistedItems: 10,
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
	t.Run("Enqueue/NonBlocking", func(t *testing.T) {
		// Regression test: Enqueue must never block. With a competing receiver
		// draining the queue, the full queue observed by the fast path select can
		// be empty by the time the slow path runs.
		prevOutput := log.Writer()
		log.SetOutput(io.Discard)
		t.Cleanup(func() { log.SetOutput(prevOutput) })

		manager := NewFileRangeManager(ManagerConfig{
			StateFileDir: t.TempDir(),
			Name:         "nonblocking.log",
			QueueSize:    1,
		})
		m := manager.(*rangeManager)

		stop := make(chan struct{})
		var consumerWg sync.WaitGroup
		consumerWg.Add(1)
		go func() {
			defer consumerWg.Done()
			for {
				select {
				case <-stop:
					return
				case <-m.queue:
				default:
					runtime.Gosched()
				}
			}
		}()

		producerDone := make(chan struct{})
		go func() {
			defer close(producerDone)
			var r Range
			for i := 0; i < 500000; i++ {
				r.ShiftInt64(int64(i))
				manager.Enqueue(r)
			}
		}()

		select {
		case <-producerDone:
		case <-time.After(30 * time.Second):
			t.Fatal("Enqueue blocked: producer did not complete")
		}
		close(stop)
		consumerWg.Wait()
		assert.Greater(t, m.Dropped(), uint64(0))
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
			StateFileDir:      tmpDir,
			Name:              "test.log",
			MaxPersistedItems: 10,
			SaveInterval:      time.Hour,
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

// TestFileRangeManagerSaveNeverExposesEmptyFile guards against a non-atomic save. A save that truncates the
// state file in place and then writes it leaves a window where the file is empty or partial. Whatever a concurrent
// reader can observe is also what a process kill (e.g. a service being force-stopped during an OS reboot) can
// freeze on disk, and an empty state file is restored as offset 0, causing the whole source to be re-sent.
func TestFileRangeManagerSaveNeverExposesEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewFileRangeManager(ManagerConfig{
		StateFileDir: tmpDir,
		Name:         "atomic.log",
	}).(*rangeManager)
	tracker := newRangeTracker(m.name, 0)
	tracker.Insert(Range{start: 0, end: 12345})
	want, err := tracker.MarshalText()
	assert.NoError(t, err)
	assert.NoError(t, m.save(tracker))

	const saves = 5000
	var saved atomic.Int64
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < saves; i++ {
			if err := m.save(tracker); err != nil {
				// On Windows the rename can be denied while the reader holds the file open; the Run loop retries
				// on the next tick, and the previous complete state file stays in place.
				if runtime.GOOS != "windows" {
					t.Errorf("save %d: %v", i, err)
					return
				}
				continue
			}
			saved.Add(1)
		}
	}()

	var reads, bad int
	for {
		select {
		case <-done:
			t.Logf("saves=%d reads=%d badObservations=%d", saved.Load(), reads, bad)
			assert.Positive(t, saved.Load(), "no save succeeded")
			assert.Positive(t, reads, "no read succeeded")
			assert.Zero(t, bad, "state file was observed empty or partial during save")
			content, err := os.ReadFile(m.stateFilePath)
			assert.NoError(t, err)
			assert.Equal(t, string(want), string(content))
			assert.Empty(t, tempFiles(t, tmpDir), "temporary file left behind after a successful save")
			return
		default:
		}
		content, err := os.ReadFile(m.stateFilePath)
		runtime.Gosched()
		if err != nil {
			if runtime.GOOS != "windows" {
				t.Fatalf("read: %v", err)
			}
			// Windows may briefly deny access while the file is being replaced; that is not an empty state.
			continue
		}
		reads++
		if !bytes.Equal(content, want) {
			bad++
		}
	}
}

// tempFiles lists the temporary save files in dir.
func tempFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	assert.NoError(t, err)
	var names []string
	for _, entry := range entries {
		if strings.Contains(entry.Name(), TempFileSuffix) {
			names = append(names, entry.Name())
		}
	}
	return names
}

func TestFileRangeManagerTempFile(t *testing.T) {
	newManager := func(dir, name string) *rangeManager {
		return NewFileRangeManager(ManagerConfig{StateFileDir: dir, Name: name}).(*rangeManager)
	}
	newTracker := func(name string, end uint64) RangeTracker {
		tracker := newRangeTracker(name, 0)
		tracker.Insert(Range{start: 0, end: end})
		return tracker
	}
	plant := func(t *testing.T, path string, age time.Duration) {
		t.Helper()
		assert.NoError(t, os.WriteFile(path, []byte("partial"), 0600))
		if age > 0 {
			old := time.Now().Add(-age)
			assert.NoError(t, os.Chtimes(path, old, old))
		}
	}

	t.Run("NoCollisionWithOtherSource", func(t *testing.T) {
		dir := t.TempDir()
		a := newManager(dir, "app.log")
		b := newManager(dir, "app.log.tmp")
		assert.NoError(t, b.save(newTracker(b.name, 20)))
		assert.NoError(t, a.save(newTracker(a.name, 10)))
		restored, err := b.Restore()
		assert.NoError(t, err)
		assert.Equal(t, uint64(20), restored.Last().EndOffset())
		restored, err = a.Restore()
		assert.NoError(t, err)
		assert.Equal(t, uint64(10), restored.Last().EndOffset())
	})

	t.Run("ConcurrentManagersSamePath", func(t *testing.T) {
		// Two collect_list entries matching the same file get two managers on one state file.
		dir := t.TempDir()
		a := newManager(dir, "app.log")
		b := newManager(dir, "app.log")
		tracker := newTracker(a.name, 10)
		want, err := tracker.MarshalText()
		assert.NoError(t, err)
		assert.NoError(t, a.save(tracker))
		var wg sync.WaitGroup
		stop := make(chan struct{})
		for _, m := range []*rangeManager{a, b} {
			wg.Add(1)
			go func(m *rangeManager) {
				defer wg.Done()
				for i := 0; i < 2000; i++ {
					if err := m.save(tracker); err != nil && runtime.GOOS != "windows" {
						t.Errorf("save: %v", err)
						return
					}
				}
			}(m)
		}
		go func() {
			wg.Wait()
			close(stop)
		}()
		var reads, bad int
		for done := false; !done; {
			select {
			case <-stop:
				done = true
			default:
			}
			content, err := os.ReadFile(a.stateFilePath)
			runtime.Gosched()
			if err != nil {
				continue
			}
			reads++
			if !bytes.Equal(content, want) {
				bad++
			}
		}
		t.Logf("reads=%d badObservations=%d", reads, bad)
		assert.Positive(t, reads)
		assert.Zero(t, bad, "state file was observed empty or partial with two concurrent managers")
		assert.Empty(t, tempFiles(t, dir))
	})

	t.Run("CreatedWithStateFileModeAndUmask", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("unix permissions")
		}
		dir := t.TempDir()
		m := newManager(dir, "app.log")
		assert.NoError(t, m.save(newTracker(m.name, 10)))
		// The previous implementation (os.WriteFile with FileMode) is the reference, umask included.
		reference := filepath.Join(dir, "reference")
		assert.NoError(t, os.WriteFile(reference, nil, FileMode))
		want, err := os.Stat(reference)
		assert.NoError(t, err)
		got, err := os.Stat(m.stateFilePath)
		assert.NoError(t, err)
		assert.Equal(t, want.Mode().Perm(), got.Mode().Perm())
	})

	t.Run("LongStateFileName", func(t *testing.T) {
		dir := t.TempDir()
		// A 250-byte state file name is valid, but appending the temp suffix and random part would exceed 255.
		name := strings.Repeat("a", 250)
		m := newManager(dir, name)
		assert.Len(t, filepath.Base(m.stateFilePath), 250)
		assert.NoError(t, m.save(newTracker(m.name, 10)))
		restored, err := m.Restore()
		assert.NoError(t, err)
		assert.Equal(t, uint64(10), restored.Last().EndOffset())
		prefix := tempFilePrefix(m.stateFilePath)
		assert.LessOrEqual(t, len(prefix)+maxTempRandLen, maxFileNameLen)
		assert.Contains(t, prefix, TempFileSuffix)
		other := tempFilePrefix(filepath.Join(dir, strings.Repeat("a", 249)+"b"))
		assert.NotEqual(t, prefix, other, "shortened prefixes must stay unique per state file")
		multiByte := tempFilePrefix(filepath.Join(dir, strings.Repeat("é", 125)))
		assert.True(t, utf8.ValidString(multiByte))
		assert.LessOrEqual(t, len(multiByte)+maxTempRandLen, maxFileNameLen)
	})

	t.Run("CleanupSparesOtherSourceNamedWithTmp", func(t *testing.T) {
		// Pins the reason TempFileSuffix contains a space: with a ".tmp" suffix, the temp prefix of "app.log" would
		// equal the state file name of the source "app.log.tmp", and cleanup would delete that source's state.
		dir := t.TempDir()
		a := newManager(dir, "app.log")
		b := newManager(dir, "app.log.tmp")
		assert.NoError(t, b.save(newTracker(b.name, 20)))
		old := time.Now().Add(-2 * StaleTempFileAge)
		assert.NoError(t, os.Chtimes(b.stateFilePath, old, old))
		assert.NoError(t, a.save(newTracker(a.name, 10)))
		a.removeTempFiles(StaleTempFileAge)
		assert.FileExists(t, b.stateFilePath)
		a.removeTempFiles(0)
		assert.FileExists(t, b.stateFilePath)
		restored, err := b.Restore()
		assert.NoError(t, err)
		assert.Equal(t, uint64(20), restored.Last().EndOffset())
	})

	t.Run("FinalSaveRecreatesRemovedStateFile", func(t *testing.T) {
		dir := t.TempDir()
		m := NewFileRangeManager(ManagerConfig{StateFileDir: dir, Name: "app.log", SaveInterval: time.Millisecond}).(*rangeManager)
		done := make(chan struct{})
		finished := make(chan struct{})
		go func() {
			defer close(finished)
			m.Run(Notification{Done: done})
		}()
		r := Range{}
		r.Shift(7)
		m.Enqueue(r)
		assert.Eventually(t, func() bool {
			_, err := os.Stat(m.stateFilePath)
			return err == nil
		}, time.Second, time.Millisecond)
		// Nothing changes after the periodic save, but the file is removed externally before shutdown.
		assert.NoError(t, os.Remove(m.stateFilePath))
		close(done)
		<-finished
		restored, err := m.Restore()
		assert.NoError(t, err)
		assert.Equal(t, uint64(7), restored.Last().EndOffset())
	})

	t.Run("RunRemovesOnlyStaleOwnTempFiles", func(t *testing.T) {
		dir := t.TempDir()
		m := newManager(dir, "app.log")
		stale := m.stateFilePath + TempFileSuffix + "1"
		fresh := m.stateFilePath + TempFileSuffix + "2"
		otherStale := filepath.Join(dir, "other.log"+TempFileSuffix+"3")
		plant(t, stale, 2*StaleTempFileAge)
		plant(t, fresh, 0)
		plant(t, otherStale, 2*StaleTempFileAge)
		done := make(chan struct{})
		finished := make(chan struct{})
		go func() {
			defer close(finished)
			m.Run(Notification{Done: done})
		}()
		close(done)
		<-finished
		assert.NoFileExists(t, stale)
		assert.FileExists(t, fresh, "a young temp file may belong to a save in progress")
		assert.FileExists(t, otherStale, "another source's temp file is not this manager's to remove")
	})

	t.Run("DeleteRemovesTempFiles", func(t *testing.T) {
		dir := t.TempDir()
		m := newManager(dir, "app.log")
		assert.NoError(t, m.save(newTracker(m.name, 10)))
		fresh := m.stateFilePath + TempFileSuffix + "1"
		plant(t, fresh, 0)
		del := make(chan struct{})
		finished := make(chan struct{})
		go func() {
			defer close(finished)
			m.Run(Notification{Delete: del})
		}()
		close(del)
		<-finished
		assert.NoFileExists(t, m.stateFilePath)
		assert.NoFileExists(t, fresh)
	})

	t.Run("FinalSaveWritesState", func(t *testing.T) {
		dir := t.TempDir()
		m := newManager(dir, "app.log")
		assert.NoError(t, m.saveFinal(newTracker(m.name, 42)))
		restored, err := m.Restore()
		assert.NoError(t, err)
		assert.Equal(t, uint64(42), restored.Last().EndOffset())
		assert.Empty(t, tempFiles(t, dir))
	})

	t.Run("FinalSaveOnDoneAfterChange", func(t *testing.T) {
		dir := t.TempDir()
		m := NewFileRangeManager(ManagerConfig{StateFileDir: dir, Name: "app.log", SaveInterval: time.Hour}).(*rangeManager)
		done := make(chan struct{})
		finished := make(chan struct{})
		go func() {
			defer close(finished)
			m.Run(Notification{Done: done})
		}()
		r := Range{}
		r.Shift(7)
		m.Enqueue(r)
		assert.Eventually(t, func() bool { return len(m.queue) == 0 }, time.Second, time.Millisecond)
		close(done)
		<-finished
		restored, err := m.Restore()
		assert.NoError(t, err)
		assert.Equal(t, uint64(7), restored.Last().EndOffset())
	})
}
