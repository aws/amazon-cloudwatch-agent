// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nodemetadatacache

import (
	"context"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
)

var (
	TypeStr, _        = component.NewType("nodemetadatacache")
	nodeMetadataCache *NodeMetadataCache
	mutex             sync.RWMutex
)

// GetNodeMetadataCache returns the singleton NodeMetadataCache instance,
// or nil if the extension has not been created or is not yet ready.
func GetNodeMetadataCache() *NodeMetadataCache {
	mutex.RLock()
	defer mutex.RUnlock()
	return nodeMetadataCache
}

// SetNodeMetadataCacheForTest sets the singleton for use in tests.
func SetNodeMetadataCacheForTest(c *NodeMetadataCache) {
	mutex.Lock()
	defer mutex.Unlock()
	nodeMetadataCache = c
}

func NewFactory() extension.Factory {
	return extension.NewFactory(
		TypeStr,
		createDefaultConfig,
		createExtension,
		component.StabilityLevelAlpha,
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Namespace: "amazon-cloudwatch",
	}
}

func createExtension(_ context.Context, settings extension.Settings, cfg component.Config) (extension.Extension, error) {
	mutex.Lock()
	defer mutex.Unlock()
	nodeMetadataCache = &NodeMetadataCache{
		logger: settings.Logger,
		config: cfg.(*Config),
		cache:  make(map[string]*NodeMetadata),
	}
	return nodeMetadataCache, nil
}
