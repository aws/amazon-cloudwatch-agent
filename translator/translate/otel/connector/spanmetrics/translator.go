// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package spanmetrics

import (
	_ "embed"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/connector"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

//go:embed config.yaml
var defaultConfig string

type translator struct {
	name    string
	factory connector.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslator(name string) common.ComponentTranslator {
	return &translator{
		name:    name,
		factory: spanmetricsconnector.NewFactory(),
	}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig()
	return common.GetYamlFileToYamlConfig(cfg, defaultConfig)
}
