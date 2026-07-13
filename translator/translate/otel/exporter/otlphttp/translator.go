// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otlphttp

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

// EndpointConfig specifies signal-specific endpoints for the otlphttp exporter.
type EndpointConfig struct {
	LogsEndpoint    string
	MetricsEndpoint string
	TracesEndpoint  string
}

type translator struct {
	name          string
	factory       exporter.Factory
	endpoint      EndpointConfig
	authenticator component.ID
}

type Option func(*translator)

// WithAuthenticator sets a custom authenticator extension for the exporter.
func WithAuthenticator(id component.ID) Option {
	return func(t *translator) {
		t.authenticator = id
	}
}

var _ common.ComponentTranslator = (*translator)(nil)

// NewTranslatorWithName creates an otlphttp exporter translator with the given
// endpoint configuration.
func NewTranslatorWithName(name string, endpoint EndpointConfig, opts ...Option) common.ComponentTranslator {
	t := &translator{
		name:     name,
		factory:  otlphttpexporter.NewFactory(),
		endpoint: endpoint,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*otlphttpexporter.Config)

	if t.endpoint.LogsEndpoint != "" {
		cfg.LogsEndpoint = t.endpoint.LogsEndpoint
	}
	if t.endpoint.MetricsEndpoint != "" {
		cfg.MetricsEndpoint = t.endpoint.MetricsEndpoint
	}
	if t.endpoint.TracesEndpoint != "" {
		cfg.TracesEndpoint = t.endpoint.TracesEndpoint
	}
	cfg.ClientConfig.Compression = configcompression.TypeGzip
	if t.authenticator.Type().String() != "" {
		cfg.ClientConfig.Auth = &configauth.Authentication{
			AuthenticatorID: t.authenticator,
		}
	}

	return cfg, nil
}
