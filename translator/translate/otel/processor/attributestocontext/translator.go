// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package attributestocontext

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributestocontextprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type translator struct {
	name    string
	factory processor.Factory
	actions []attributestocontextprocessor.ActionKeyValue
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslator(actions []attributestocontextprocessor.ActionKeyValue) common.ComponentTranslator {
	return &translator{factory: attributestocontextprocessor.NewFactory(), actions: actions}
}

func NewTranslatorWithName(name string, actions []attributestocontextprocessor.ActionKeyValue) common.ComponentTranslator {
	return &translator{name: name, factory: attributestocontextprocessor.NewFactory(), actions: actions}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*attributestocontextprocessor.Config)
	cfg.Actions = t.actions
	return cfg, nil
}
