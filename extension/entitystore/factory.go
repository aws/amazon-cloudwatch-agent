// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entitystore

import (
	"context"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
)

var (
	TypeStr, _  = component.NewType("entitystore")
	entityStore *EntityStore
	mutex       sync.RWMutex
)

func GetEntityStore() *EntityStore {
	mutex.RLock()
	defer mutex.RUnlock()
	if entityStore != nil && entityStore.ready.Load() {
		return entityStore
	}
	return nil
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
	return &Config{}
}

func createExtension(_ context.Context, settings extension.CreateSettings, cfg component.Config) (extension.Extension, error) {
	mutex.Lock()
	defer mutex.Unlock()
	entityStore = &EntityStore{
		logger: settings.Logger,
		config: cfg.(*Config),
	}
	return entityStore, nil
}
