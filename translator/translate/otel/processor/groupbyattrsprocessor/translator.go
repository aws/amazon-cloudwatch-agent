// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package groupbyattrsprocessor

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbyattrsprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type translator struct {
	name    string
	keys    []string
	factory processor.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

type Option func(*translator)

func WithKeys(keys []string) Option {
	return func(t *translator) {
		t.keys = keys
	}
}

func NewTranslatorWithName(name string, opts ...Option) common.ComponentTranslator {
	t := &translator{name: name, keys: []string{}, factory: groupbyattrsprocessor.NewFactory()}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*groupbyattrsprocessor.Config)
	cfg.GroupByKeys = t.keys
	return cfg, nil
}
