// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package selftelemetry

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
)

var TypeStr, _ = component.NewType("selftelemetry")

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

func createExtension(_ context.Context, settings extension.Settings, cfg component.Config) (extension.Extension, error) {
	return newSelfTelemetry(settings, cfg.(*Config)), nil
}
