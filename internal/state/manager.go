// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package state

import (
	"errors"
	"log"
	"os"
	"time"
)

type rangeManager struct {
	name          string
	stateFilePath string
	queue         chan Range
	saveInterval  time.Duration
}

// RangeManager is a state manager that handles the Range.
type RangeManager Manager[Range, *RangeTree]

var _ RangeManager = (*rangeManager)(nil)

func NewFileRangeManager(cfg ManagerConfig) RangeManager {
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = defaultQueueSize
	}
	if cfg.SaveInterval <= 0 {
		cfg.SaveInterval = defaultSaveInterval
	}
	return &rangeManager{
		name:          cfg.Name,
		stateFilePath: cfg.StateFilePath(),
		queue:         make(chan Range, cfg.QueueSize),
		saveInterval:  cfg.SaveInterval,
	}
}

// Enqueue the offset. Will drop the oldest in the queue if full.
func (m *rangeManager) Enqueue(item Range) {
	select {
	case m.queue <- item:
	default:
		old := <-m.queue
		log.Printf("D! Offset queue is full for %s. Dropping oldest offset: %v", m.stateFilePath, old.String())
		m.queue <- item
	}
}

// Restore the offset of the file if the state file exists.
func (m *rangeManager) Restore() (*RangeTree, error) {
	content, err := os.ReadFile(m.stateFilePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Printf("D! No state file exists for %s", m.name)
		} else {
			log.Printf("W! Failed to read state file for %s: %v", m.name, err)
		}
		return nil, err
	}
	tree := NewRangeTree()
	if err = tree.UnmarshalText(content); err != nil {
		log.Printf("W! Invalid state file content: %v", err)
		return nil, err
	}
	log.Printf("I! Reading from offset %v in %s", tree.String(), m.name)
	return tree, nil
}

// Save the offset in the state file.
func (m *rangeManager) Save(tree *RangeTree) error {
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

	current := NewRangeTree()
	changedSinceLastSave := false
	for {
		select {
		case item := <-m.queue:
			changedSinceLastSave = changedSinceLastSave || current.Insert(item)
		case <-t.C:
			if !changedSinceLastSave {
				continue
			}
			if err := m.Save(current); err != nil {
				log.Printf("E! Error happened when saving state file (%s): %v", m.stateFilePath, err)
				continue
			}
			changedSinceLastSave = false
		case <-notification.Delete:
			log.Printf("W! Deleting state file (%s)", m.stateFilePath)
			if err := os.Remove(m.stateFilePath); err != nil {
				log.Printf("W! Error happened while deleting state file (%s) on cleanup: %v", m.stateFilePath, err)
			}
			return
		case <-notification.Done:
			if err := m.Save(current); err != nil {
				log.Printf("E! Error happened during final state file (%s) save, duplicate log maybe sent at next start: %v", m.stateFilePath, err)
			}
			return
		}
	}
}
