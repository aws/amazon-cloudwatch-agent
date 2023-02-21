// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package processor

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
)

type translator struct {
	factory component.ProcessorFactory
}

func NewDefaultTranslator(factory component.ProcessorFactory) common.Translator[component.Config] {
	return &translator{factory}
}

func (t *translator) Translate(*confmap.Conf, common.TranslatorOptions) (component.Config, error) {
	return t.factory.CreateDefaultConfig(), nil
}

func (t *translator) Type() component.Type {
	return t.factory.Type()
}
