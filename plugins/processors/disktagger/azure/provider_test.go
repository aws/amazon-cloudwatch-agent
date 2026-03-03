// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package azure

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBaseDevice(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"sda", "sda"},
		{"sda1", "sda"},
		{"sda15", "sda"},
		{"sdb", "sdb"},
		{"sdb1", "sdb"},
		{"nvme0n1", "nvme0n1"},
		{"nvme0n1p1", "nvme0n1"},
		{"nvme0n1p15", "nvme0n1"},
		{"nvme1n1", "nvme1n1"},
		{"nvme1n1p2", "nvme1n1"},
		{"xvda", "xvda"},
		{"xvda1", "xvda"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, baseDevice(tt.input))
		})
	}
}

func TestResolveScsiDevice(t *testing.T) {
	// Create a fake sysfs tree under a temp dir.
	root := t.TempDir()
	blockDir := filepath.Join(root, "sys", "bus", "scsi", "devices", "0:0:0:0", "block", "sda")
	require.NoError(t, os.MkdirAll(blockDir, 0755))
	blockDir1 := filepath.Join(root, "sys", "bus", "scsi", "devices", "1:0:0:0", "block", "sdc")
	require.NoError(t, os.MkdirAll(blockDir1, 0755))

	p := &Provider{rootfsPrefix: root}
	assert.Equal(t, "sda", p.resolveScsiDevice(0, 0))
	assert.Equal(t, "sdc", p.resolveScsiDevice(1, 0))
	assert.Equal(t, "", p.resolveScsiDevice(1, 5))
}

func TestResolveSymlink(t *testing.T) {
	root := t.TempDir()
	// Create a symlink: root/dev/disk/azure/root -> ../../sda
	linkDir := filepath.Join(root, "dev", "disk", "azure")
	require.NoError(t, os.MkdirAll(linkDir, 0755))
	require.NoError(t, os.Symlink("../../sda", filepath.Join(linkDir, "root")))

	p := &Provider{rootfsPrefix: root}
	assert.Equal(t, "sda", p.resolveSymlink("/dev/disk/azure/root"))
	assert.Equal(t, "", p.resolveSymlink("/dev/disk/azure/nonexistent"))
}

func TestDeviceToSerialMap_ScsiPath(t *testing.T) {
	profile := storageProfile{
		OsDisk:    osDisk{Name: "os-disk"},
		DataDisks: []dataDisk{{LUN: "0", Name: "data-disk-0"}},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "True", r.Header.Get("Metadata"))
		json.NewEncoder(w).Encode(profile)
	}))
	defer server.Close()

	root := t.TempDir()
	// OS disk at SCSI 0:0:0:0
	require.NoError(t, os.MkdirAll(filepath.Join(root, "sys", "bus", "scsi", "devices", "0:0:0:0", "block", "sda"), 0755))
	// Data disk at SCSI 1:0:0:0
	require.NoError(t, os.MkdirAll(filepath.Join(root, "sys", "bus", "scsi", "devices", "1:0:0:0", "block", "sdc"), 0755))

	p := &Provider{
		client:       server.Client(),
		endpoint:     server.URL,
		rootfsPrefix: root,
		useSymlinks:  false,
	}

	result, err := p.DeviceToSerialMap(context.Background())
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"sda": "os-disk", "sdc": "data-disk-0"}, result)
}

func TestDeviceToSerialMap_SymlinkPath(t *testing.T) {
	profile := storageProfile{
		OsDisk: osDisk{Name: "os-disk"},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(profile)
	}))
	defer server.Close()

	root := t.TempDir()
	linkDir := filepath.Join(root, "dev", "disk", "azure")
	require.NoError(t, os.MkdirAll(linkDir, 0755))
	require.NoError(t, os.Symlink("../../sda", filepath.Join(linkDir, "root")))

	p := &Provider{
		client:       server.Client(),
		endpoint:     server.URL,
		rootfsPrefix: root,
		useSymlinks:  true,
	}

	result, err := p.DeviceToSerialMap(context.Background())
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"sda": "os-disk"}, result)
}

func TestFetchStorageProfile_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	p := &Provider{client: server.Client(), endpoint: server.URL}
	_, err := p.DeviceToSerialMap(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "IMDS returned 500")
}
