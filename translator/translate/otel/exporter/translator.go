// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package exporter

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type translator struct {
	name    string
	factory exporter.Factory
}

func NewDefaultTranslator(factory exporter.Factory) common.Translator[component.Config] {
	return NewDefaultTranslatorWithName("", factory)
}

func NewDefaultTranslatorWithName(name string, factory exporter.Factory) common.Translator[component.Config] {
	return &translator{name, factory}
}

func (t *translator) Translate(*confmap.Conf) (component.Config, error) {
	return t.factory.CreateDefaultConfig(), nil
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}
