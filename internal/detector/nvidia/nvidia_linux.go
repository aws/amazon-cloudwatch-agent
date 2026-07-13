// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux

package nvidia

import (
	"os"
	"os/exec"
	"regexp"
)

const defaultNvidiaSMIPath = "/usr/bin/nvidia-smi"

type checker struct {
	devPath     string
	driverPaths []string
}

// newChecker creates a new NVIDIA GPU checker with Linux defaults.
func newChecker() deviceChecker {
	return &checker{
		devPath:     "/dev",
		driverPaths: []string{defaultNvidiaSMIPath},
	}
}

// hasNvidiaDevice checks for NVIDIA GPU devices on Linux by looking for device files.
// NVIDIA GPUs create device files in /dev/nvidia[0-9]+ when the kernel module is loaded.
func (c *checker) hasNvidiaDevice() bool {
	entries, err := os.ReadDir(c.devPath)
	if err != nil {
		return false
	}
	pattern := regexp.MustCompile(`^nvidia\d+$`)
	for _, entry := range entries {
		if pattern.MatchString(entry.Name()) {
			return true
		}
	}
	return false
}

// hasDriverFiles checks if NVIDIA driver tools are available on the system.
// nvidia-smi is the primary management and monitoring tool for NVIDIA GPUs.
func (c *checker) hasDriverFiles() bool {
	for _, path := range c.driverPaths {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}

	// Fallback to PATH lookup
	_, err := exec.LookPath(nvidiaSMI)
	return err == nil
}
