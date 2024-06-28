// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resourcestore

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
)

var (
	TypeStr, _    = component.NewType("resourcestore")
	resourceStore *ResourceStore
)

func GetResourceStore() *ResourceStore {
	return resourceStore
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
	resourceStore = &ResourceStore{
		logger: settings.Logger,
		config: cfg.(*Config),
	}
	return resourceStore, nil
}
