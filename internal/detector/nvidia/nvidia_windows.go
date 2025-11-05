// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows

package nvidia

import (
	"os"
	"os/exec"
	"strings"

	windowsregistry "golang.org/x/sys/windows/registry"
)

// registry abstracts registry operations for testing
type registry interface {
	OpenKey(k windowsregistry.Key, path string, access uint32) (registryKey, error)
}

// registryKey abstracts registry key operations
type registryKey interface {
	Close() error
	ReadSubKeyNames(n int) ([]string, error)
}

// registryWrapper implements Registry using the actual Windows registry
type registryWrapper struct{}

func (wr *registryWrapper) OpenKey(k windowsregistry.Key, path string, access uint32) (registryKey, error) {
	key, err := windowsregistry.OpenKey(k, path, access)
	if err != nil {
		return nil, err
	}
	return key, nil
}

const (
	defaultNvidiaSMIPath      = `C:\Program Files\NVIDIA Corporation\NVSMI\nvidia-smi.exe`
	defaultNvidiaSMIPathWin10 = `C:\Windows\System32\nvidia-smi.exe`
)

type checker struct {
	registry    registry
	driverPaths []string
}

// newChecker creates a new NVIDIA GPU checker with Windows defaults.
func newChecker() deviceChecker {
	return &checker{
		registry: &registryWrapper{},
		driverPaths: []string{
			defaultNvidiaSMIPath,
			defaultNvidiaSMIPathWin10,
		},
	}
}

// hasNvidiaDevice checks for NVIDIA GPU devices on Windows by examining the PCI device registry.
// VEN_10DE is NVIDIA's PCI vendor ID assigned by the PCI-SIG organization.
// Source: https://pcisig.com/membership/member-companies
func (c *checker) hasNvidiaDevice() bool {
	key, err := c.registry.OpenKey(windowsregistry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Enum\PCI`, windowsregistry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return false
	}
	defer key.Close()

	subkeys, err := key.ReadSubKeyNames(-1)
	if err != nil {
		return false
	}

	for _, subkey := range subkeys {
		if strings.HasPrefix(subkey, "VEN_10DE") {
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
