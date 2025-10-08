// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package detector

import (
	"context"
	"errors"
)

var (
	// ErrSkipProcess indicates that the current process being evaluated should be skipped.
	ErrSkipProcess = errors.New("skip process")
	// ErrIncompatibleDetector indicates that a specific detector is incompatible with a resource.
	ErrIncompatibleDetector = errors.New("incompatible detector")
)

// ProcessDetector defines an interface for detecting and categorizing processes.
type ProcessDetector interface {
	// Detect attempts to gather metadata for a given process. Returns an error if the detection fails.
	Detect(ctx context.Context, process Process) (*Metadata, error)
}

// DeviceDetector defines an interface for detecting and categorizing devices.
type DeviceDetector interface {
	// Detect attempts to gather metadata for devices. Returns an error if the detection fails.
	Detect() (*Metadata, error)
}

// Process defines an interface for interacting with system processes.
type Process interface {
	// PID returns the process ID.
	PID() int32
	// ExeWithContext returns the executable path of the process.
	ExeWithContext(ctx context.Context) (string, error)
	// CwdWithContext returns the current working directory of the process.
	CwdWithContext(ctx context.Context) (string, error)
	// CmdlineSliceWithContext returns the command line arguments of the process as a slice. Includes the executable
	// in the first entry.
	CmdlineSliceWithContext(ctx context.Context) ([]string, error)
	// EnvironWithContext returns the environment variables of the process. Each entry follows a <NAME>=<VALUE> pattern.
	EnvironWithContext(ctx context.Context) ([]string, error)
	// CreateTimeWithContext returns the creation time of the process in milliseconds.
	CreateTimeWithContext(ctx context.Context) (int64, error)
}
