// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package state

import (
	"bytes"
	"encoding"
	"errors"
	"log"
	"math"
	"os"
	"strconv"
	"time"
)

const (
	// defaultSaveInterval is the default duration between state file saves
	defaultSaveInterval = 100 * time.Millisecond
	// defaultQueueSize is the default capacity of the offset queue
	defaultQueueSize = 2000
)

// FileOffset represents a position within a file. It handles truncation detection through
// sequence numbers.
type FileOffset struct {
	// seq handles file truncation, when file is truncated, we increase the seq
	seq    uint64
	offset uint64
}

var _ encoding.TextMarshaler = (*FileOffset)(nil)
var _ encoding.TextUnmarshaler = (*FileOffset)(nil)
var _ Comparator[FileOffset] = (*FileOffset)(nil)

// NewFileOffset creates a new FileOffset with the specified value.
func NewFileOffset(v uint64) FileOffset {
	return FileOffset{offset: v}
}

// Set updates the offset. Increments the sequence number when a smaller offset is given to indicate possible
// file truncation.
func (o *FileOffset) Set(v uint64) {
	if v < o.offset {
		o.seq++
	}
	o.offset = v
}

// SetInt64 updates the offset. Ignores negative values.
func (o *FileOffset) SetInt64(v int64) {
	if v >= 0 {
		o.Set(uint64(v))
	}
}

// Get returns the offset.
func (o FileOffset) Get() uint64 {
	return o.offset
}

// GetInt64 returns the offset. Returns 0 if offset exceeds MaxInt64.
func (o FileOffset) GetInt64() int64 {
	if o.offset > math.MaxInt64 {
		return 0
	}
	return int64(o.offset)
}

// MarshalText converts the offset to a string.
func (o FileOffset) MarshalText() ([]byte, error) {
	return []byte(strconv.FormatUint(o.offset, 10)), nil
}

// UnmarshalText parses the first line into the offset.
func (o *FileOffset) UnmarshalText(text []byte) error {
	firstLine := text
	index := bytes.IndexByte(text, '\n')
	if index != -1 {
		firstLine = text[:index]
	}
	offset, err := strconv.ParseUint(string(firstLine), 10, 64)
	if err != nil {
		return err
	}
	o.offset = offset
	return nil
}

// Compare two FileOffset based on sequence number and then offset value.
func (o FileOffset) Compare(other FileOffset) int {
	if o.seq != other.seq {
		if o.seq < other.seq {
			return -1
		}
		return 1
	}
	if o.offset < other.offset {
		return -1
	}
	if o.offset > other.offset {
		return 1
	}
	return 0
}

type fileOffsetManager struct {
	name          string
	stateFilePath string
	offsetCh      chan FileOffset
	saveInterval  time.Duration
}

// FileOffsetManager is a state manager that handles the FileOffset.
type FileOffsetManager Manager[FileOffset, FileOffset]

var _ FileOffsetManager = (*fileOffsetManager)(nil)

func NewFileOffsetManager(cfg ManagerConfig) FileOffsetManager {
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = defaultQueueSize
	}
	if cfg.SaveInterval <= 0 {
		cfg.SaveInterval = defaultSaveInterval
	}
	return &fileOffsetManager{
		name:          cfg.Name,
		stateFilePath: cfg.StateFilePath(),
		offsetCh:      make(chan FileOffset, cfg.QueueSize),
		saveInterval:  cfg.SaveInterval,
	}
}

// Enqueue the offset. Will drop the oldest in the queue if full.
func (m *fileOffsetManager) Enqueue(offset FileOffset) {
	select {
	case m.offsetCh <- offset:
	default:
		o := <-m.offsetCh
		log.Printf("D! Offset queue is full for %s. Dropping oldest offset: %d", m.stateFilePath, o.Get())
		m.offsetCh <- offset
	}
}

// Restore the offset of the file if the state file exists.
func (m *fileOffsetManager) Restore() (FileOffset, error) {
	var offset FileOffset
	content, err := os.ReadFile(m.stateFilePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Printf("D! No state file exists for %s", m.name)
		} else {
			log.Printf("W! Failed to read state file for %s: %v", m.name, err)
		}
		return offset, err
	}
	if err = offset.UnmarshalText(content); err != nil {
		log.Printf("W! Invalid state file content: %v", err)
		return offset, err
	}
	log.Printf("I! Reading from offset %v in %s", offset.Get(), m.name)
	return offset, nil
}

// save the offset in the state file.
func (m *fileOffsetManager) save(offset FileOffset) error {
	if m.stateFilePath == "" {
		return nil
	}
	data, err := offset.MarshalText()
	if err != nil {
		return err
	}
	data = append(data, []byte("\n"+m.name)...)
	return os.WriteFile(m.stateFilePath, data, FileMode)
}

// Run starts the update/save loop.
func (m *fileOffsetManager) Run(notification Notification) {
	t := time.NewTicker(m.saveInterval)
	defer t.Stop()

	var offset, lastSavedOffset FileOffset
	for {
		select {
		case o := <-m.offsetCh:
			if o.Compare(offset) > 0 {
				offset = o
			}
		case <-t.C:
			if offset.Compare(lastSavedOffset) == 0 {
				continue
			}
			if err := m.save(offset); err != nil {
				log.Printf("E! Error happened when saving state file (%s): %v", m.stateFilePath, err)
				continue
			}
			lastSavedOffset = offset
		case <-notification.Delete:
			log.Printf("W! Deleting state file (%s)", m.stateFilePath)
			if err := os.Remove(m.stateFilePath); err != nil {
				log.Printf("W! Error happened while deleting state file (%s) on cleanup: %v", m.stateFilePath, err)
			}
			return
		case <-notification.Done:
			if err := m.save(offset); err != nil {
				log.Printf("E! Error happened during final state file (%s) save, duplicate log maybe sent at next start: %v", m.stateFilePath, err)
			}
			return
		}
	}
}
