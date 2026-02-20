// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package disktagger

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapProvider_Serial_ExactMatch(t *testing.T) {
	p := newMapProvider(nil)
	p.cache = map[string]string{"sda": "disk1", "sdb": "disk2"}

	assert.Equal(t, "disk1", p.Serial("sda"))
	assert.Equal(t, "disk2", p.Serial("sdb"))
}

func TestMapProvider_Serial_PrefixMatch(t *testing.T) {
	p := newMapProvider(nil)
	p.cache = map[string]string{"sda": "os-disk", "sdc": "data-disk"}

	assert.Equal(t, "os-disk", p.Serial("sda1"))
	assert.Equal(t, "os-disk", p.Serial("sda2"))
	assert.Equal(t, "data-disk", p.Serial("sdc1"))
}

func TestMapProvider_Serial_NoMatch(t *testing.T) {
	p := newMapProvider(nil)
	p.cache = map[string]string{"sda": "os-disk"}

	assert.Equal(t, "", p.Serial("sdb1"))
	assert.Equal(t, "", p.Serial("nvme0n1"))
}

func TestMapProvider_Serial_NvmePrefix(t *testing.T) {
	p := newMapProvider(nil)
	p.cache = map[string]string{"nvme0n1": "vol-abc"}

	assert.Equal(t, "vol-abc", p.Serial("nvme0n1"))
	assert.Equal(t, "vol-abc", p.Serial("nvme0n1p1"))
}

func TestMapProvider_Refresh(t *testing.T) {
	callCount := 0
	fetch := func(_ context.Context) (map[string]string, error) {
		callCount++
		return map[string]string{"sda": fmt.Sprintf("disk-%d", callCount)}, nil
	}

	p := newMapProvider(fetch)
	assert.Equal(t, "", p.Serial("sda"))

	require.NoError(t, p.Refresh(context.Background()))
	assert.Equal(t, "disk-1", p.Serial("sda"))

	require.NoError(t, p.Refresh(context.Background()))
	assert.Equal(t, "disk-2", p.Serial("sda"))
}

func TestMapProvider_Refresh_Error(t *testing.T) {
	fetch := func(_ context.Context) (map[string]string, error) {
		return nil, fmt.Errorf("imds error")
	}

	p := newMapProvider(fetch)
	p.cache = map[string]string{"sda": "old-disk"}

	err := p.Refresh(context.Background())
	assert.Error(t, err)
	// Cache should be unchanged on error
	assert.Equal(t, "old-disk", p.Serial("sda"))
}
