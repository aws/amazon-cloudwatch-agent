// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package state

import (
	"path/filepath"
	"strings"
	"time"
)

const (
	// FileMode is the file permissions used for the state file.
	FileMode = 0644
)

// Queue handles queued state changes.
type Queue[T any] interface {
	ID() string
	// Enqueue the current state in memory.
	Enqueue(state T)
}

// Manager handles persistence of state.
type Manager[I, O any] interface {
	Queue[I]
	// Restore loads the previous state.
	Restore() (O, error)
	// Run starts the update/save loop.
	Run(ch Notification)
}

// Notification contains channels used to stop the Manager run loop.
type Notification struct {
	Delete chan struct{}
	Done   chan struct{}
}

// ManagerConfig provides all options available to configure a Manager.
type ManagerConfig struct {
	// Name is the metadata that will be persisted in the last line of the state file.
	Name string
	// StateFileDir is the directory where the state file will be written to.
	StateFileDir string
	// StateFilePrefix is an optional prefix added to the filename. Can be used to group state files.
	StateFilePrefix string
	// QueueSize determines the size of the internal buffer for pending state changes.
	QueueSize int
	// SaveInterval determines how often the state is persisted.
	SaveInterval time.Duration
	// MaxPersistItems is the maximum number of items to persist in the saved state. If zero or negative, the
	// persistence is unbounded.
	MaxPersistItems int
}

// StateFilePath returns the full path to the state file.
func (c ManagerConfig) StateFilePath() string {
	return FilePath(c.StateFileDir, c.StateFilePrefix+c.Name)
}

// FilePath combines the directory and escaped name.
func FilePath(dir, name string) string {
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, escapeFilePath(name))
}

func escapeFilePath(filePath string) string {
	escapedFilePath := filepath.ToSlash(filePath)
	escapedFilePath = strings.ReplaceAll(escapedFilePath, "/", "_")
	escapedFilePath = strings.ReplaceAll(escapedFilePath, " ", "_")
	escapedFilePath = strings.ReplaceAll(escapedFilePath, ":", "_")
	return escapedFilePath
}
