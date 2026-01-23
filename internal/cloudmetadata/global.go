// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudmetadata

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

var (
	globalProvider Provider
	globalOnce     sync.Once
	globalErr      error
	globalMu       sync.RWMutex
)

// InitGlobalProvider initializes the global cloud metadata provider.
// Safe to call multiple times - only the first call has effect.
//
// IMPORTANT: This function is typically called asynchronously during agent startup
// with a timeout context (e.g., 5 seconds). Callers using GetGlobalProvider() or
// GetGlobalProviderOrNil() must handle the case where initialization has not yet
// completed or has failed. Use GetGlobalProviderOrNil() for graceful degradation.
func InitGlobalProvider(ctx context.Context, logger *zap.Logger) error {
	globalOnce.Do(func() {
		if logger == nil {
			logger = zap.NewNop()
		}

		logger.Debug("[cloudmetadata] Initializing global provider...")

		globalMu.Lock()
		defer globalMu.Unlock()

		globalProvider, globalErr = NewProvider(ctx, logger)
		if globalErr != nil {
			logger.Warn("[cloudmetadata] Cloud detection failed - continuing without metadata provider",
				zap.Error(globalErr))
			return
		}

		cloudType := CloudProvider(globalProvider.GetCloudProvider()).String()
		logger.Info("[cloudmetadata] Cloud provider detected",
			zap.String("cloud", cloudType))

		if err := globalProvider.Refresh(ctx); err != nil {
			logger.Warn("[cloudmetadata] Failed to refresh cloud metadata during init",
				zap.Error(err))
			// Don't fail - provider may still be usable
		}

		logger.Info("[cloudmetadata] Provider initialized successfully",
			zap.String("cloud", cloudType),
			zap.Bool("available", globalProvider.IsAvailable()),
			zap.String("instanceId", MaskValue(globalProvider.GetInstanceID())),
			zap.String("region", globalProvider.GetRegion()))
	})

	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalErr
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

// ResetGlobalProvider resets the singleton state.
// FOR TESTING ONLY. Not safe for concurrent use with other global provider functions.
// Tests using this function must run serially (not in parallel with t.Parallel()).
// Resets sync.Once to allow re-initialization in test scenarios.
func ResetGlobalProvider() {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalProvider = nil
	globalOnce = sync.Once{}
	globalErr = nil
}

// SetGlobalProviderForTest injects a mock provider. FOR TESTING ONLY.
// This function is not safe for concurrent use with other global provider functions.
// Tests using this function must run serially (not in parallel with t.Parallel()).
// Marks the provider as initialized to prevent InitGlobalProvider from overwriting.
func SetGlobalProviderForTest(p Provider) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalProvider = p
	globalErr = nil
	// Mark as initialized so InitGlobalProvider won't overwrite
	globalOnce.Do(func() {})
}
