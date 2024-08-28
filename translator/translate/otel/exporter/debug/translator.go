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
	name    string
	factory exporter.Factory
}

var _ common.Translator[component.Config] = (*translator)(nil)

func NewTranslator() common.Translator[component.Config] {
	return NewTranslatorWithName("")
}

func NewTranslatorWithName(name string) common.Translator[component.Config] {
	return &translator{name: name, factory: debugexporter.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
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
