// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otlphttp

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type translator struct {
	name          string
	factory       exporter.Factory
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

func NewTranslatorWithName(name string, opts ...Option) common.ComponentTranslator {
	t := &translator{
		name:          name,
		factory:       otlphttpexporter.NewFactory(),
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

// Translate creates an otlphttp exporter config that sends OTLP logs to the
// CloudWatch OTLP endpoint.
func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*otlphttpexporter.Config)

	region := agent.Global_Config.Region
	if region == "" {
		return nil, fmt.Errorf("region is required for otlphttp exporter")
	}

	cfg.ClientConfig.Endpoint = fmt.Sprintf("https://logs.%s.amazonaws.com", region)
	cfg.LogsEndpoint = fmt.Sprintf("https://logs.%s.amazonaws.com/v1/logs", region)
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
