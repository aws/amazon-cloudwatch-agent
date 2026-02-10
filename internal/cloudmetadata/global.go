// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudmetadata

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

var (
	globalProvider Provider
	globalErr      error
	initOnce       sync.Once
)

// InitGlobalProvider initializes the global cloud metadata provider.
// Safe to call multiple times - only the first call has effect (uses sync.Once).
// Called lazily on first GetGlobalProvider() call.
func initGlobalProvider() {
	logger := zap.NewNop()

	logger.Debug("[cloudmetadata] Initializing global provider...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

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
	}

	logger.Info("[cloudmetadata] Provider initialized successfully",
		zap.String("cloud", cloudType),
		zap.Bool("available", globalProvider.IsAvailable()),
		zap.String("region", globalProvider.GetRegion()))
}

// GetGlobalProvider returns the initialized global provider.
// Initializes lazily on first call (blocks first caller).
// Returns an error if the provider initialization failed.
func GetGlobalProvider() (Provider, error) {
	initOnce.Do(initGlobalProvider)
	if globalProvider == nil {
		if globalErr != nil {
			return nil, fmt.Errorf("cloud metadata initialization failed: %w", globalErr)
		}
		return nil, fmt.Errorf("cloud metadata not initialized")
	}
	return globalProvider, nil
}

// GetGlobalProviderOrNil returns the provider or nil if unavailable.
// Initializes lazily on first call (blocks first caller).
// Use when metadata is optional and caller can handle nil gracefully.
func GetGlobalProviderOrNil() Provider {
	initOnce.Do(initGlobalProvider)
	return globalProvider
}

// ResetGlobalProvider resets the singleton state for testing.
// FOR TESTING ONLY.
// Note: This creates a new sync.Once to allow re-initialization in tests.
func ResetGlobalProvider() {
	globalProvider = nil
	globalErr = nil
	initOnce = sync.Once{}
}

// SetGlobalProviderForTest injects a mock provider. FOR TESTING ONLY.
// Marks initialization as complete to prevent lazy init from running.
func SetGlobalProviderForTest(p Provider) {
	globalProvider = p
	globalErr = nil
	// Mark as initialized to prevent lazy init
	initOnce.Do(func() {})
}
