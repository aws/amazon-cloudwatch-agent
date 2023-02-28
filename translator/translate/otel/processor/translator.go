// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package processor

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
)

type translator struct {
	name    string
	factory component.ProcessorFactory
}

func NewDefaultTranslator(factory component.ProcessorFactory) common.Translator[component.Config] {
	return NewDefaultTranslatorWithName("", factory)
}

func NewDefaultTranslatorWithName(name string, factory component.ProcessorFactory) common.Translator[component.Config] {
	return &translator{name, factory}
}

func (t *translator) Translate(*confmap.Conf) (component.Config, error) {
	return t.factory.CreateDefaultConfig(), nil
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}
