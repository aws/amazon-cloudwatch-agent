// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package signaltometrics

import (
	_ "embed"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector"
	signaltometricsconfig "github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/config"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/connector"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

//go:embed dbi_topsql.yaml
var dbiTopsqlConfig string

type translator struct {
	name    string
	factory connector.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

// NewTranslator creates a signaltometrics connector translator. The name determines which config to load.
func NewTranslator(name string) common.ComponentTranslator {
	return &translator{
		name:    name,
		factory: signaltometricsconnector.NewFactory(),
	}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*signaltometricsconfig.Config)

	if t.name == common.DbiConnectorTopsql {
		return common.GetYamlFileToYamlConfig(cfg, dbiTopsqlConfig)
	}

	return nil, fmt.Errorf("unsupported signaltometrics connector config: %s", t.name)
}
