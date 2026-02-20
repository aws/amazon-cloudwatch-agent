// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package disktagger

import (
	"context"
	"strings"
	"sync"
)

// DiskProvider maps device names to disk identifiers (volume IDs, managed disk names, etc.)
type DiskProvider interface {
	// Refresh updates the internal device-to-disk mapping.
	// The context allows cancellation during shutdown.
	Refresh(ctx context.Context) error

	// Serial returns the disk identifier for a device name.
	// Supports prefix matching (e.g. "nvme0n1p1" matches "nvme0n1").
	Serial(devName string) string
}

// mapProvider wraps a simple map for providers that don't need prefix matching (e.g. Azure).
type mapProvider struct {
	fetchFunc func(ctx context.Context) (map[string]string, error)
	mu        sync.RWMutex
	cache     map[string]string
}

func newMapProvider(fetchFunc func(ctx context.Context) (map[string]string, error)) *mapProvider {
	return &mapProvider{fetchFunc: fetchFunc, cache: make(map[string]string)}
}

func (p *mapProvider) Refresh(ctx context.Context) error {
	result, err := p.fetchFunc(ctx)
	if err != nil {
		return err
	}
	p.mu.Lock()
	p.cache = result
	p.mu.Unlock()
	return nil
}

func (p *mapProvider) Serial(devName string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if v, ok := p.cache[devName]; ok {
		return v
	}
	for k, v := range p.cache {
		if strings.HasPrefix(devName, k) {
			return v
		}
	}
	return ""
}
