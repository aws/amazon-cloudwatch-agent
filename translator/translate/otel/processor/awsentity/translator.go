// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsentity

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsentity"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const name = "awsentity"

type translator struct {
	factory processor.Factory
}

func NewTranslator() common.Translator[component.Config] {
	return &translator{
		factory: awsentity.NewFactory(),
	}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), "")
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	return t.factory.CreateDefaultConfig().(*awsentity.Config), nil
}
