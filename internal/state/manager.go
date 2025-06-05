// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package state

import (
	"errors"
	"log"
	"os"
	"time"
)

const (
	// defaultSaveInterval is the default duration between state file saves
	defaultSaveInterval = 100 * time.Millisecond
	// defaultQueueSize is the default capacity of the offset queue
	defaultQueueSize = 2000
)

type rangeManager struct {
	name            string
	stateFilePath   string
	queue           chan Range
	saveInterval    time.Duration
	maxPersistItems int
	replaceTreeCh   chan *rangeTree
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
		name:            cfg.Name,
		stateFilePath:   cfg.StateFilePath(),
		queue:           make(chan Range, cfg.QueueSize),
		saveInterval:    cfg.SaveInterval,
		maxPersistItems: cfg.MaxPersistItems,
		replaceTreeCh:   make(chan *rangeTree, 1),
	}
}

// Enqueue the Range. Will drop the oldest in the queue if full.
func (m *rangeManager) Enqueue(item Range) {
	select {
	case m.queue <- item:
	default:
		old := <-m.queue
		log.Printf("D! Offset queue is full for %s. Dropping oldest offset: %v", m.stateFilePath, old.String())
		m.queue <- item
	}
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
	tree := newRangeTreeWithCap(m.maxPersistItems)
	if err = tree.UnmarshalText(content); err != nil {
		log.Printf("W! Invalid state file content: %v", err)
		return RangeList{}, err
	}
	m.replaceTreeCh <- tree
	log.Printf("I! Reading from offset %v in %s", tree.String(), m.name)
	return tree.Ranges(), nil
}

// save the ranges in the state file.
func (m *rangeManager) save(tree *rangeTree) error {
	if m.stateFilePath == "" {
		return nil
	}
	data, err := tree.MarshalText()
	if err != nil {
		return err
	}
	data = append(data, []byte("\n"+m.name)...)
	return os.WriteFile(m.stateFilePath, data, FileMode)
}

// Run starts the update/save loop.
func (m *rangeManager) Run(notification Notification) {
	t := time.NewTicker(m.saveInterval)
	defer t.Stop()

	var lastSeq uint64
	current := newRangeTreeWithCap(m.maxPersistItems)
	shouldSave := false
	for {
		select {
		case replace := <-m.replaceTreeCh:
			current = replace
		case item := <-m.queue:
			// truncation detected, clear tree
			if item.seq > lastSeq {
				lastSeq = item.seq
				current.Clear()
			}
			changed := current.Insert(item)
			shouldSave = shouldSave || changed
		case <-t.C:
			if !shouldSave {
				continue
			}
			if err := m.save(current); err != nil {
				log.Printf("E! Error happened when saving state file (%s): %v", m.stateFilePath, err)
				continue
			}
			shouldSave = false
		case <-notification.Delete:
			log.Printf("W! Deleting state file (%s)", m.stateFilePath)
			if err := os.Remove(m.stateFilePath); err != nil {
				log.Printf("W! Error happened while deleting state file (%s) on cleanup: %v", m.stateFilePath, err)
			}
			return
		case <-notification.Done:
			if err := m.save(current); err != nil {
				log.Printf("E! Error happened during final state file (%s) save, duplicate log maybe sent at next start: %v", m.stateFilePath, err)
			}
			return
		}
	}
}
