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

//go:embed dbi_topsql_postgresql.yaml
var dbiTopsqlConfig string

//go:embed dbi_topsql_mysql.yaml
var dbiTopsqlMysqlConfig string

type translator struct {
	name    string
	engine  string
	factory connector.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

// NewTranslator creates a signaltometrics connector translator. The name sets the
// component ID; the engine ("postgresql" or "mysql") selects which config to load.
func NewTranslator(name string, engine string) common.ComponentTranslator {
	return &translator{
		name:    name,
		engine:  engine,
		factory: signaltometricsconnector.NewFactory(),
	}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*signaltometricsconfig.Config)

	switch t.engine {
	case common.PostgreSQLKey:
		return common.GetYamlFileToYamlConfig(cfg, dbiTopsqlConfig)
	case common.MySQLKey:
		return common.GetYamlFileToYamlConfig(cfg, dbiTopsqlMysqlConfig)
	}

	return nil, fmt.Errorf("unsupported signaltometrics connector engine: %s", t.engine)
}
