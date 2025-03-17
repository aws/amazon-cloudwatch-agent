// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8smetadata

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"

	"github.com/aws/amazon-cloudwatch-agent/extension/k8smetadata"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type translator struct {
	name    string
	factory extension.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslator() common.ComponentTranslator {
	return &translator{
		factory: k8smetadata.NewFactory(),
	}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

// Translate creates an extension configuration.
func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*k8smetadata.Config)
	return cfg, nil
}
