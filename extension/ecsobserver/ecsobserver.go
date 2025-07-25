// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsobserver

import (
	"context"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
)

// ECSObserver implements the extension.Extension interface for ECS observer.
type ECSObserver struct {
	config *ecsobserver.Config
	logger *zap.Logger
	ext    extension.Extension
}

// NewECSObserver creates a new ECS observer extension with the given config.
func NewECSObserver(config *ecsobserver.Config, logger *zap.Logger, settings component.TelemetrySettings) (*ECSObserver, error) {
	// Create the OpenTelemetry ECS observer extension
	factory := ecsobserver.NewFactory()
	
	// Create component ID with the correct type that the upstream factory expects
	componentID := component.NewIDWithName(component.MustNewType("ecs_observer"), "")
	
	ext, err := factory.Create(
		context.Background(),
		extension.Settings{
			ID:                componentID,
			TelemetrySettings: settings,
			BuildInfo:         component.BuildInfo{},
		},
		config,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create ECS observer extension: %w", err)
	}

	return &ECSObserver{
		config: config,
		logger: logger,
		ext:    ext,
	}, nil
}

// Start starts the ECS observer extension.
func (e *ECSObserver) Start(ctx context.Context, host component.Host) error {
	e.logger.Info("Starting ECS observer extension")
	return e.ext.Start(ctx, host)
}

// Shutdown stops the ECS observer extension.
func (e *ECSObserver) Shutdown(ctx context.Context) error {
	e.logger.Info("Shutting down ECS observer extension")
	return e.ext.Shutdown(ctx)
}

// GetConfig returns the ECS observer configuration.
func (e *ECSObserver) GetConfig() *ecsobserver.Config {
	return e.config
}
