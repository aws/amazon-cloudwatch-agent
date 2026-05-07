// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otlphttp

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

// EndpointConfig specifies the base endpoint and signal-specific endpoint for
// the otlphttp exporter.
type EndpointConfig struct {
	BaseEndpoint    string // e.g. "https://logs.us-east-1.amazonaws.com"
	LogsEndpoint    string // e.g. "https://logs.us-east-1.amazonaws.com/v1/logs"
	MetricsEndpoint string
	TracesEndpoint  string
}

type translator struct {
	name          string
	factory       exporter.Factory
	endpoint      EndpointConfig
	authenticator component.ID
	headers       map[string]string
}

type Option func(*translator)

// WithAuthenticator sets a custom authenticator extension for the exporter.
func WithAuthenticator(id component.ID) Option {
	return func(t *translator) {
		t.authenticator = id
	}
}

// WithHeaders sets static HTTP headers on the exporter.
func WithHeaders(headers map[string]string) Option {
	return func(t *translator) {
		t.headers = headers
	}
}

var _ common.ComponentTranslator = (*translator)(nil)

// NewTranslatorWithName creates an otlphttp exporter translator with the given
// endpoint configuration.
func NewTranslatorWithName(name string, endpoint EndpointConfig, opts ...Option) common.ComponentTranslator {
	t := &translator{
		name:          name,
		factory:       otlphttpexporter.NewFactory(),
		endpoint:      endpoint,
		authenticator: component.NewID(component.MustNewType(common.SigV4Auth)),
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

	cfg.ClientConfig.Endpoint = t.endpoint.BaseEndpoint
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
	cfg.ClientConfig.Auth = &configauth.Authentication{
		AuthenticatorID: t.authenticator,
	}

	if len(t.headers) > 0 {
		for k, v := range t.headers {
			cfg.ClientConfig.Headers[k] = configopaque.String(v)
		}
	}

	return cfg, nil
}
