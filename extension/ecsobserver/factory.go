// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsobserver

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/aws/amazon-cloudwatch-agent/service/registry"
)

// Type is the extension type.
var Type = component.MustNewType("ecsobserver")

// NewFactory creates a factory for the ECS observer extension.
func NewFactory() extension.Factory {
	return extension.NewFactory(
		Type,
		createDefaultConfig,
		createExtension,
		component.StabilityLevelBeta,
	)
}

func createDefaultConfig() component.Config {
	return &ecsobserver.Config{}
}

func createExtension(_ context.Context, params extension.Settings, cfg component.Config) (extension.Extension, error) {
	// Cast the config to ecsobserver.Config
	config := cfg.(*ecsobserver.Config)
	
	// Create and return our custom ECS observer extension
	return NewECSObserver(config, params.TelemetrySettings.Logger, params.TelemetrySettings)
}

// Register registers the ECS observer extension with the registry.
func Register() {
	registry.Register(registry.WithExtension(NewFactory()))
}
