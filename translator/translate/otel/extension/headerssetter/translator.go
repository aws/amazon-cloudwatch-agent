// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package headerssetter

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/headerssetterextension"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

// HeaderMapping defines a header to set from a context key.
type HeaderMapping struct {
	HeaderName string
	ContextKey string
}

type translator struct {
	name           string
	additionalAuth component.ID
	headers        []HeaderMapping
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslatorWithName(name string, additionalAuth component.ID, headers []HeaderMapping) common.ComponentTranslator {
	return &translator{
		name:           name,
		additionalAuth: additionalAuth,
		headers:        headers,
	}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(component.MustNewType("headers_setter"), t.name)
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := headerssetterextension.NewFactory().CreateDefaultConfig().(*headerssetterextension.Config)
	cfg.AdditionalAuth = &t.additionalAuth

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
