// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudmetadata

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"go.uber.org/zap"
)

var (
	globalProvider Provider
	globalErr      error
	globalMu       sync.RWMutex
	initialized    uint32 // atomic: 0 = not initialized, 1 = initialized
)

// InitGlobalProvider initializes the global cloud metadata provider.
// Safe to call multiple times - only the first call has effect.
//
// IMPORTANT: This function is typically called asynchronously during agent startup
// with a timeout context (e.g., 5 seconds). Callers using GetGlobalProvider() or
// GetGlobalProviderOrNil() must handle the case where initialization has not yet
// completed or has failed. Use GetGlobalProviderOrNil() for graceful degradation.
func InitGlobalProvider(ctx context.Context, logger *zap.Logger) error {
	// Fast path: already initialized
	if atomic.LoadUint32(&initialized) == 1 {
		globalMu.RLock()
		defer globalMu.RUnlock()
		return globalErr
	}

	globalMu.Lock()
	defer globalMu.Unlock()

	// Double-check under lock
	if atomic.LoadUint32(&initialized) == 1 {
		return globalErr
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	logger.Debug("[cloudmetadata] Initializing global provider...")

	globalProvider, globalErr = NewProvider(ctx, logger)
	if globalErr != nil {
		logger.Warn("[cloudmetadata] Cloud detection failed - continuing without metadata provider",
			zap.Error(globalErr))
		atomic.StoreUint32(&initialized, 1)
		return globalErr
	}

	cloudType := CloudProvider(globalProvider.GetCloudProvider()).String()
	logger.Info("[cloudmetadata] Cloud provider detected",
		zap.String("cloud", cloudType))

	if err := globalProvider.Refresh(ctx); err != nil {
		logger.Warn("[cloudmetadata] Failed to refresh cloud metadata during init",
			zap.Error(err))
	}

	logger.Info("[cloudmetadata] Provider initialized successfully",
		zap.String("cloud", cloudType),
		zap.Bool("available", globalProvider.IsAvailable()),
		zap.String("instanceId", MaskValue(globalProvider.GetInstanceID())),
		zap.String("region", globalProvider.GetRegion()))

	atomic.StoreUint32(&initialized, 1)
	return nil
}

// GetGlobalProvider returns the initialized global provider.
// Returns an error if the provider was not initialized or initialization failed.
func GetGlobalProvider() (Provider, error) {
	globalMu.RLock()
	defer globalMu.RUnlock()

	if globalProvider == nil {
		if globalErr != nil {
			return nil, fmt.Errorf("cloud metadata initialization failed: %w", globalErr)
		}
		return nil, fmt.Errorf("cloud metadata not initialized: call InitGlobalProvider first")
	}
	return globalProvider, nil
}

// GetGlobalProviderOrNil returns the provider or nil if unavailable.
// Use when metadata is optional and caller can handle nil gracefully.
func GetGlobalProviderOrNil() Provider {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalProvider
}

// ResetGlobalProvider resets the singleton state for testing.
// FOR TESTING ONLY.
func ResetGlobalProvider() {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalProvider = nil
	globalErr = nil
	atomic.StoreUint32(&initialized, 0)
}

// SetGlobalProviderForTest injects a mock provider. FOR TESTING ONLY.
func SetGlobalProviderForTest(p Provider) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalProvider = p
	globalErr = nil
	atomic.StoreUint32(&initialized, 1)
}
