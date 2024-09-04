// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entitystore

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
)

var (
	TypeStr, _  = component.NewType("entitystore")
	entityStore *EntityStore
)

func GetEntityStore() *EntityStore {
	return entityStore
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
	entityStore = &EntityStore{
		logger: settings.Logger,
		config: cfg.(*Config),
	}
	return entityStore, nil
}
