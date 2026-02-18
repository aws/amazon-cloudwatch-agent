// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

// Package azure provides disk-to-device mapping for Azure VMs.
//
// # How Azure disk mapping works
//
// Azure VMs have three types of disks:
//   - OS disk: The boot disk containing the operating system
//   - Temporary/resource disk: Ephemeral local storage (not backed by Azure Storage)
//   - Data disks: Additional persistent disks attached to the VM
//
// Each disk is identified in IMDS (Instance Metadata Service) by name and,
// for data disks, by LUN (Logical Unit Number). The challenge is mapping
// these IMDS identifiers to Linux block device names (e.g. sda, sdc).
//
// # Device identification strategy
//
// Linux device names (sda, sdb, etc.) are assigned nondeterministically
// based on SCSI enumeration order. We use two methods to reliably map
// IMDS disk metadata to device names:
//
// Method 1 (preferred): /dev/disk/azure/ symlinks
//
// The Azure Linux Agent (waagent) installs udev rules that create stable
// symlinks under /dev/disk/azure/:
//
//	/dev/disk/azure/root       → ../../sda     (OS disk)
//	/dev/disk/azure/resource   → ../../sdb     (temp disk)
//	/dev/disk/azure/scsi1/lun0 → ../../sdc     (data disk at LUN 0)
//	/dev/disk/azure/scsi1/lun1 → ../../sdd     (data disk at LUN 1)
//
// This is the officially recommended method per Azure documentation:
// https://learn.microsoft.com/en-us/azure/virtual-machines/linux/azure-to-guest-disk-mapping
//
// Method 2 (fallback): sysfs SCSI address mapping
//
// If the Azure Linux Agent is not installed (no /dev/disk/azure/ symlinks),
// we fall back to reading the SCSI bus topology from sysfs. Azure VMs use
// a predictable SCSI layout:
//
//	Controller 0 (host 0): OS disk at LUN 0, temp disk at LUN 1
//	Controller 1 (host 1): Data disks at LUN 0, 1, 2, ...
//
// The sysfs path format is:
//
//	/sys/bus/scsi/devices/{host}:{channel}:{target}:{lun}/block/{device}
//
// For example:
//
//	/sys/bus/scsi/devices/0:0:0:0/block/sda  → OS disk
//	/sys/bus/scsi/devices/1:0:0:0/block/sdc  → data disk LUN 0
//	/sys/bus/scsi/devices/1:0:0:1/block/sdd  → data disk LUN 1
//
// This is the same mechanism the Azure Linux Agent uses internally to
// create the /dev/disk/azure/ symlinks.
//
// # Containerized environments
//
// When running inside a container, the host filesystem is typically
// mounted at /rootfs. Both methods check for /rootfs prefix.
package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	storageProfileEndpoint = "http://169.254.169.254/metadata/instance/compute/storageProfile"
	apiVersion             = "2025-04-07"

	// Azure Linux Agent symlink paths.
	azureDiskRoot    = "/dev/disk/azure/root"
	azureDiskDataDir = "/dev/disk/azure/scsi1"

	// Azure SCSI controller layout.
	// Controller 0: OS disk (LUN 0) and temp disk (LUN 1).
	// Controller 1: Data disks (LUN 0, 1, 2, ...).
	scsiHostOS   = 0
	scsiHostData = 1
	scsiLunOS    = 0
)

type managedDisk struct {
	ID string `json:"id"`
}

type dataDisk struct {
	LUN         string      `json:"lun"`
	Name        string      `json:"name"`
	ManagedDisk managedDisk `json:"managedDisk"`
}

type osDisk struct {
	Name        string      `json:"name"`
	ManagedDisk managedDisk `json:"managedDisk"`
}

type storageProfile struct {
	DataDisks []dataDisk `json:"dataDisks"`
	OsDisk    osDisk     `json:"osDisk"`
}

// Provider maps Linux device names to Azure managed disk names.
type Provider struct {
	client   *http.Client
	endpoint string
}

func NewProvider() *Provider {
	return &Provider{
		client:   &http.Client{Timeout: 5 * time.Second},
		endpoint: storageProfileEndpoint,
	}
}

func (p *Provider) DeviceToDiskID(ctx context.Context) (map[string]string, error) {
	profile, err := p.fetchStorageProfile(ctx)
	if err != nil {
		return nil, err
	}

	// Determine which device resolution method to use.
	useSymlinks := symlinkAvailable()

	result := make(map[string]string)

	// Map OS disk.
	if profile.OsDisk.Name != "" {
		var dev string
		if useSymlinks {
			dev = resolveSymlink(azureDiskRoot)
		} else {
			dev = resolveScsiDevice(scsiHostOS, scsiLunOS)
		}
		if dev != "" {
			result[dev] = profile.OsDisk.Name
		}
	}

	// Map data disks.
	for _, d := range profile.DataDisks {
		lun, err := strconv.Atoi(d.LUN)
		if err != nil {
			continue
		}
		var dev string
		if useSymlinks {
			link := filepath.Join(azureDiskDataDir, fmt.Sprintf("lun%d", lun))
			dev = resolveSymlink(link)
		} else {
			dev = resolveScsiDevice(scsiHostData, lun)
		}
		if dev != "" {
			result[dev] = d.Name
		}
	}

	return result, nil
}

// symlinkAvailable checks if Azure Linux Agent symlinks exist.
func symlinkAvailable() bool {
	for _, prefix := range []string{"", "/rootfs"} {
		if _, err := os.Lstat(prefix + azureDiskRoot); err == nil {
			return true
		}
	}
	return false
}

// resolveSymlink reads a /dev/disk/azure/* symlink and returns the base
// device name (without partition suffix).
//
// Example: /dev/disk/azure/root → ../../sda → "sda"
func resolveSymlink(path string) string {
	for _, prefix := range []string{"", "/rootfs"} {
		target, err := os.Readlink(prefix + path)
		if err != nil {
			continue
		}
		return baseDevice(filepath.Base(target))
	}
	return ""
}

// resolveScsiDevice finds the block device for a given SCSI host and LUN
// by scanning sysfs. This is the fallback when Azure Linux Agent is not
// installed.
//
// Example: host=1, lun=0 → scans /sys/bus/scsi/devices/1:0:0:0/block/* → "sdc"
func resolveScsiDevice(host, lun int) string {
	pattern := fmt.Sprintf("/sys/bus/scsi/devices/%d:0:0:%d/block/*", host, lun)
	for _, prefix := range []string{"", "/rootfs"} {
		matches, err := filepath.Glob(prefix + pattern)
		if err != nil || len(matches) == 0 {
			continue
		}
		return baseDevice(filepath.Base(matches[0]))
	}
	return ""
}

// baseDevice strips the partition suffix from a device name to get the
// base block device.
//
// Examples:
//
//	"sda1"      → "sda"
//	"nvme0n1p1" → "nvme0n1"
//	"sda"       → "sda"
func baseDevice(dev string) string {
	if strings.Contains(dev, "nvme") {
		// NVMe: nvme0n1p1 → nvme0n1
		if idx := strings.LastIndex(dev, "p"); idx > 0 {
			// Verify what follows 'p' is digits (partition number)
			if _, err := strconv.Atoi(dev[idx+1:]); err == nil {
				return dev[:idx]
			}
		}
		return dev
	}
	// SCSI: sda1 → sda
	return strings.TrimRight(dev, "0123456789")
}

func (p *Provider) fetchStorageProfile(ctx context.Context) (*storageProfile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("azure disktagger: create request: %w", err)
	}
	req.Header.Set("Metadata", "True")
	q := req.URL.Query()
	q.Set("api-version", apiVersion)
	q.Set("format", "json")
	req.URL.RawQuery = q.Encode()

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("azure disktagger: IMDS request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("azure disktagger: IMDS returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("azure disktagger: read response: %w", err)
	}

	var profile storageProfile
	if err := json.Unmarshal(body, &profile); err != nil {
		return nil, fmt.Errorf("azure disktagger: decode response: %w", err)
	}

	return &profile, nil
}
