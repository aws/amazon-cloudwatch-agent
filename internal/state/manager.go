// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package state

import (
	"errors"
	"fmt"
	"hash/fnv"
	"log"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

// TempFileSuffix is appended (followed by a random string) to a state file name to name the temporary file used
// during an atomic save. It contains a space, which escapeFilePath never leaves in a state file name, so a temporary
// file can never collide with the state file of another source (e.g. "app.log" and "app.log.tmp"). Any file in the
// state folder whose name contains TempFileSuffix is a temporary file.
const TempFileSuffix = " .tmp"

// StaleTempFileAge is the age after which a temporary file is considered left behind by an interrupted save. Younger
// ones may belong to a save in progress (possibly by another manager on the same state file) and are kept.
const StaleTempFileAge = time.Minute

const (
	// defaultSaveInterval is the default duration between state file saves
	defaultSaveInterval = 100 * time.Millisecond
	// defaultQueueSize is the default capacity of the offset queue
	defaultQueueSize = 2000
	// dropLogInterval is the minimum duration between offset range drop logs.
	dropLogInterval = 30 * time.Second
	// finalSaveAttempts bounds the retries of the final save on shutdown. On Windows the rename can be refused
	// while another process (e.g. an antivirus scan) briefly holds the state file open.
	finalSaveAttempts = 5
	// finalSaveRetryDelay is the delay before the first final save retry. It doubles on each attempt.
	finalSaveRetryDelay = 20 * time.Millisecond
)

type rangeManager struct {
	name              string
	stateFilePath     string
	queue             chan Range
	saveInterval      time.Duration
	maxPersistedItems int
	replaceTrackerCh  chan RangeTracker

	// dropped counts offset ranges discarded because the queue was full. Accessed atomically.
	dropped     atomic.Uint64
	lastDropLog atomic.Int64
}

// FileRangeManager is a state manager that handles the Range.
type FileRangeManager Manager[Range, RangeList]

var _ FileRangeManager = (*rangeManager)(nil)

func NewFileRangeManager(cfg ManagerConfig) FileRangeManager {
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = defaultQueueSize
	}
	if cfg.SaveInterval <= 0 {
		cfg.SaveInterval = defaultSaveInterval
	}
	return &rangeManager{
		name:              cfg.Name,
		stateFilePath:     cfg.StateFilePath(),
		queue:             make(chan Range, cfg.QueueSize),
		saveInterval:      cfg.SaveInterval,
		maxPersistedItems: cfg.MaxPersistedItems,
		replaceTrackerCh:  make(chan RangeTracker, 1),
	}
}

func (m *rangeManager) ID() string {
	return m.name
}

// Enqueue the Range. Will drop the oldest in the queue if full.
//
// Both operations are non-blocking by design: Run drains the queue concurrently, so a queue
// observed full at the send may already be empty at the receive. Dropping the oldest keeps the
// freshest offsets, costing duplicate records after a restart rather than lost ones. Each
// iteration either enqueues the item or frees a slot, so the loop always terminates.
func (m *rangeManager) Enqueue(item Range) {
	for {
		select {
		case m.queue <- item:
			return
		default:
		}
		select {
		case old := <-m.queue:
			m.reportDrop(old)
		default:
		}
	}
}

// reportDrop records a dropped Range and logs at most once per dropLogInterval.
func (m *rangeManager) reportDrop(item Range) {
	total := m.dropped.Add(1)
	now := time.Now()
	last := m.lastDropLog.Load()
	if now.UnixNano()-last < int64(dropLogInterval) {
		return
	}
	// Only the CAS winner logs.
	if !m.lastDropLog.CompareAndSwap(last, now.UnixNano()) {
		return
	}
	log.Printf("D! Offset range queue is full for %s. Dropped offset range: %s (%d dropped in total)", m.stateFilePath, item, total)
}

// Dropped returns the number of offset ranges discarded because the queue was full.
func (m *rangeManager) Dropped() uint64 {
	return m.dropped.Load()
}

// Restore the ranges if the state file exists.
func (m *rangeManager) Restore() (RangeList, error) {
	content, err := os.ReadFile(m.stateFilePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Printf("D! No state file exists for %s", m.name)
		} else {
			log.Printf("W! Failed to read state file for %s: %v", m.name, err)
		}
		return RangeList{}, err
	}
	if len(content) == 0 {
		log.Printf("W! State file for %s is empty, reading from the start", m.name)
	}
	tracker := newRangeTracker(m.name, m.maxPersistedItems)
	if err = tracker.UnmarshalText(content); err != nil {
		log.Printf("W! Invalid state file content: %v", err)
		return RangeList{}, err
	}
	restored := tracker.Ranges()
	m.replaceTrackerCh <- tracker
	log.Printf("I! Reading from offset range %s in %s", restored, m.name)
	return restored, nil
}

// save the ranges in the state file.
func (m *rangeManager) save(tracker RangeTracker) error {
	return m.saveWithOptions(tracker, false)
}

// saveFinal saves the ranges durably (fsync) and retries a bounded number of times, since there is no later tick
// to retry on.
func (m *rangeManager) saveFinal(tracker RangeTracker) error {
	var err error
	delay := finalSaveRetryDelay
	for attempt := 1; attempt <= finalSaveAttempts; attempt++ {
		if err = m.saveWithOptions(tracker, true); err == nil {
			return nil
		}
		if attempt < finalSaveAttempts {
			time.Sleep(delay)
			delay *= 2
		}
	}
	return err
}

func (m *rangeManager) saveWithOptions(tracker RangeTracker, sync bool) error {
	if m.stateFilePath == "" {
		return nil
	}
	data, err := tracker.MarshalText()
	if err != nil {
		return err
	}
	return writeFileAtomic(m.stateFilePath, data, FileMode, sync)
}

// maxFileNameLen is the file name length limit (in bytes) of common filesystems (ext4, XFS, NTFS).
const maxFileNameLen = 255

// maxTempRandLen is the maximum length of the random part of a temporary file name.
const maxTempRandLen = 10

// tempFilePrefix returns the name prefix of the temporary files used to save the state file at path. It is the state
// file name followed by TempFileSuffix. If that would make the temporary file name exceed maxFileNameLen, the state
// file name is shortened and a hash of the full name is added to keep the prefix unique per state file.
func tempFilePrefix(path string) string {
	base := filepath.Base(path)
	if len(base)+len(TempFileSuffix)+maxTempRandLen <= maxFileNameLen {
		return base + TempFileSuffix
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(base))
	hash := fmt.Sprintf("-%08x", h.Sum32())
	short := base[:maxFileNameLen-len(TempFileSuffix)-maxTempRandLen-len(hash)]
	// Do not split a multi-byte character.
	for !utf8.ValidString(short) {
		short = short[:len(short)-1]
	}
	return short + hash + TempFileSuffix
}

// writeFileAtomic replaces the file at path with data so that the file is never observed (or left behind after
// the process is killed mid-save) truncated or partially written. The data is written to a new, uniquely named
// (O_EXCL) sibling temporary file that is then renamed over path, so concurrent savers of the same path never share
// a temporary file. If sync is set, the data is flushed to disk before the rename.
func writeFileAtomic(path string, data []byte, perm os.FileMode, sync bool) error {
	f, err := createTempFile(filepath.Dir(path), tempFilePrefix(path), perm)
	if err != nil {
		return err
	}
	tmpPath := f.Name()
	_, err = f.Write(data)
	if err == nil && sync {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tmpPath, path)
	}
	if err != nil {
		_ = os.Remove(tmpPath)
	}
	return err
}

// createTempFile creates a new file named prefix followed by a random number in dir. Unlike os.CreateTemp, the file
// is created with perm (subject to the umask), matching how the state file has always been created.
func createTempFile(dir, prefix string, perm os.FileMode) (*os.File, error) {
	for attempt := 0; ; attempt++ {
		name := filepath.Join(dir, prefix+strconv.FormatUint(uint64(rand.Uint32()), 10))
		f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
		if errors.Is(err, os.ErrExist) && attempt < 100 {
			continue
		}
		return f, err
	}
}

// Run starts the update/save loop.
func (m *rangeManager) Run(notification Notification) {
	t := time.NewTicker(m.saveInterval)
	defer t.Stop()

	// Remove temporary files left behind by saves interrupted in a previous run.
	m.removeTempFiles(StaleTempFileAge)

	var lastSeq uint64
	currentTracker := newRangeTracker(m.name, m.maxPersistedItems)
	shouldSave := false
	for {
		select {
		case replaceTracker := <-m.replaceTrackerCh:
			currentTracker = replaceTracker
		case item := <-m.queue:
			// truncation detected, clear tree
			if item.seq > lastSeq {
				lastSeq = item.seq
				currentTracker.Clear()
			}
			changed := currentTracker.Insert(item)
			shouldSave = shouldSave || changed
		case <-t.C:
			if !shouldSave {
				continue
			}
			if err := m.save(currentTracker); err != nil {
				log.Printf("E! Error happened when saving state file (%s): %v", m.stateFilePath, err)
				continue
			}
			shouldSave = false
		case <-notification.Delete:
			log.Printf("W! Deleting state file (%s)", m.stateFilePath)
			if err := os.Remove(m.stateFilePath); err != nil {
				log.Printf("W! Error happened while deleting state file (%s) on cleanup: %v", m.stateFilePath, err)
			}
			m.removeTempFiles(0)
			return
		case <-notification.Done:
			// Skip the final save if nothing changed since the last one, unless the state file was removed
			// meanwhile (e.g. by the logfile state folder cleanup during a rotation).
			if !shouldSave {
				if _, err := os.Stat(m.stateFilePath); err == nil || m.stateFilePath == "" {
					return
				}
			}
			if err := m.saveFinal(currentTracker); err != nil {
				log.Printf("E! Error happened during final state file (%s) save, duplicate log maybe sent at next start: %v", m.stateFilePath, err)
			}
			return
		}
	}
}

// removeTempFiles removes this state file's temporary files that are at least minAge old.
func (m *rangeManager) removeTempFiles(minAge time.Duration) {
	if m.stateFilePath == "" {
		return
	}
	dir := filepath.Dir(m.stateFilePath)
	prefix := tempFilePrefix(m.stateFilePath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}
		if info, err := entry.Info(); err != nil || time.Since(info.ModTime()) < minAge {
			continue
		}
		tmpPath := filepath.Join(dir, entry.Name())
		if err = os.Remove(tmpPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("W! Error happened while removing temporary state file (%s): %v", tmpPath, err)
		}
	}
}

type FileRangeQueue Queue[Range]

// RangeQueueBatcher is meant for merging continuous ranges before sending them to the FileRangeQueue.
type RangeQueueBatcher struct {
	queue FileRangeQueue
	r     Range
}

func NewRangeQueueBatcher(queue FileRangeQueue) *RangeQueueBatcher {
	return &RangeQueueBatcher{queue: queue}
}

// Merge stores the min start and max end between the current state and the provided range.
func (b *RangeQueueBatcher) Merge(r Range) {
	if !r.IsValid() {
		return
	}
	if !b.r.IsValid() {
		b.r = r
		return
	}
	b.r.start = min(b.r.start, r.start)
	b.r.end = max(b.r.end, r.end)
}

// Done enqueues the built range (if valid) on the queue.
func (b *RangeQueueBatcher) Done() {
	if b.queue != nil && b.r.IsValid() {
		b.queue.Enqueue(b.r)
	}
}
