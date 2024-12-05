// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agenthealth

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
)

var (
	TypeStr, _ = component.NewType("agenthealth")
)

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
		IsUsageDataEnabled: true,
		Stats:              nil,
	}
}

func createExtension(_ context.Context, settings extension.CreateSettings, cfg component.Config) (extension.Extension, error) {
	return NewAgentHealth(settings.Logger, cfg.(*Config)), nil
}
