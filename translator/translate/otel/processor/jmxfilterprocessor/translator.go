// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package jmxfilterprocessor

import (
	_ "embed"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type translator struct {
	name    string
	factory processor.Factory
}

type Option func(any)

var _ common.Translator[component.Config] = (*translator)(nil)

func NewTranslatorWithName(name string) common.Translator[component.Config] {
	return &translator{name: name, factory: filterprocessor.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if conf == nil || !conf.IsSet(common.ContainerInsightsConfigKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: common.ContainerInsightsConfigKey}
	}

	cfg := t.factory.CreateDefaultConfig().(*filterprocessor.Config)
	if t.name == common.PipelineNameContainerInsightsJmx {
		return common.GetYamlFileToYamlConfig(cfg, metricTransformJmxConfig)
	}

	return cfg, nil
}
