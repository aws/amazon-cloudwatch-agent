// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package registry

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/receiver"
)

type Option func(factories *otelcol.Factories)

var registry []Option

// Options getter for registry.
func Options() []Option {
	return registry
}

// Reset sets registry to nil.
func Reset() {
	registry = nil
}

// Register adds the options to the registry.
func Register(options ...Option) {
	registry = append(registry, options...)
}

// WithReceiver sets the receiver factory in the factories. Will overwrite duplicate types.
func WithReceiver(factory receiver.Factory) Option {
	return func(factories *otelcol.Factories) {
		if factories.Receivers == nil {
			factories.Receivers = make(map[component.Type]receiver.Factory)
		}
		factories.Receivers[factory.Type()] = factory
	}
}

// WithProcessor sets the processor factory in the factories. Will overwrite duplicate types.
func WithProcessor(factory processor.Factory) Option {
	return func(factories *otelcol.Factories) {
		if factories.Processors == nil {
			factories.Processors = make(map[component.Type]processor.Factory)
		}
		factories.Processors[factory.Type()] = factory
	}
}

// WithExporter sets the exporter factory in the factories. Will overwrite duplicate types.
func WithExporter(factory exporter.Factory) Option {
	return func(factories *otelcol.Factories) {
		if factories.Exporters == nil {
			factories.Exporters = make(map[component.Type]exporter.Factory)
		}
		factories.Exporters[factory.Type()] = factory
	}
}

// WithExtension sets the extension factory in the factories. Will overwrite duplicate types.
func WithExtension(factory extension.Factory) Option {
	return func(factories *otelcol.Factories) {
		if factories.Extensions == nil {
			factories.Extensions = make(map[component.Type]extension.Factory)
		}
		factories.Extensions[factory.Type()] = factory
	}
}
