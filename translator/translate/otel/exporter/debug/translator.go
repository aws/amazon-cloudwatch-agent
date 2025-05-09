// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package debug

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/debugexporter"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const defaultSamplingThereafter = 500

type translator struct {
	common.NameProvider
	factory exporter.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslator(opts ...common.TranslatorOption) common.ComponentTranslator {
	t := &translator{factory: debugexporter.NewFactory()}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.Name())
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if conf == nil || !conf.IsSet(common.AgentDebugConfigKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: common.AgentDebugConfigKey}
	}

	cfg := t.factory.CreateDefaultConfig().(*debugexporter.Config)
	cfg.Verbosity = configtelemetry.LevelDetailed
	cfg.SamplingThereafter = defaultSamplingThereafter
	return cfg, nil
}
