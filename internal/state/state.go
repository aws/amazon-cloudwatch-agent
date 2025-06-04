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

// Comparator compares the itself with another of the same type.
type Comparator[T any] interface {
	Compare(other T) int
}

// Manager handles persistence of state.
type Manager[T any] interface {
	// Enqueue the current state in memory.
	Enqueue(state T)
	// Restore loads the previous state.
	Restore() (T, error)
	// Save persists the current state.
	Save(state T) error
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
	Name            string
	StateFileDir    string
	StateFilePrefix string
	QueueSize       int
	SaveInterval    time.Duration
}

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
