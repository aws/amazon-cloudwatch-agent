// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nvidia

import (
	"log/slog"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
)

const nvidiaSMI = "nvidia-smi" //nolint:unused // Used in platform-specific files

// nvidiaDetector implements logging and checker for NVIDIA GPU
type nvidiaDetector struct {
	logger  *slog.Logger
	checker deviceChecker
}

// deviceChecker interface for OS-specific device detection
type deviceChecker interface {
	hasNvidiaDevice() bool
	hasDriverFiles() bool
}

// NewDetector creates new NVIDIA detector using OS-specific checker
func NewDetector(logger *slog.Logger) detector.DeviceDetector {
	return &nvidiaDetector{
		logger:  logger,
		checker: newChecker(),
	}
}

// Detect attempts to detect NVIDIA GPU devices and their status.
func (d *nvidiaDetector) Detect() (*detector.Metadata, error) {
	d.logger.Debug("Starting NVIDIA GPU detection")

	if !d.checker.hasNvidiaDevice() {
		d.logger.Debug("No NVIDIA GPU devices found")
		return nil, detector.ErrIncompatibleDetector
	}

	d.logger.Debug("NVIDIA GPU device detected")
	status := detector.StatusNeedsSetupNvidiaDriver
	if d.checker.hasDriverFiles() {
		d.logger.Debug("NVIDIA GPU device and driver both available")
		status = detector.StatusReady
	} else {
		d.logger.Debug("NVIDIA GPU device found but driver not available")
	}

	return &detector.Metadata{
		Categories: []detector.Category{detector.CategoryNvidiaGPU},
		Status:     status,
	}, nil
}
