// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package headerssetter

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/headerssetterextension"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

// HeaderMapping defines a header to set from a context key.
type HeaderMapping struct {
	HeaderName string
	ContextKey string
}

type Option func(*translator)

func WithAdditionalAuth(id component.ID) Option {
	return func(t *translator) {
		t.additionalAuth = &id
	}
}

func WithHeaders(headers []HeaderMapping) Option {
	return func(t *translator) {
		t.headers = headers
	}
}

type translator struct {
	name           string
	factory        extension.Factory
	additionalAuth *component.ID
	headers        []HeaderMapping
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslatorWithName(name string, opts ...Option) common.ComponentTranslator {
	t := &translator{
		name:    name,
		factory: headerssetterextension.NewFactory(),
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
	cfg := t.factory.CreateDefaultConfig().(*headerssetterextension.Config)
	cfg.AdditionalAuth = t.additionalAuth

	for _, h := range t.headers {
		key := h.HeaderName
		ctx := h.ContextKey
		cfg.HeadersConfig = append(cfg.HeadersConfig, headerssetterextension.HeaderConfig{
			Key:         &key,
			FromContext: &ctx,
			Action:      headerssetterextension.UPSERT,
		})
	}
	return cfg, nil
}
