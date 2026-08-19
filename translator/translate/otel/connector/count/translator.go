// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package count

import (
	_ "embed"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/countconnector"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/connector"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

//go:embed dbi_dbload_postgresql.yaml
var dbiDbloadConfig string

//go:embed dbi_dbload_mysql.yaml
var dbiDbloadMysqlConfig string

type translator struct {
	name    string
	engine  string
	factory connector.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

// NewTranslator creates a count connector translator. The name sets the
// component ID; the engine ("postgresql" or "mysql") selects which config to load.
func NewTranslator(name string, engine string) common.ComponentTranslator {
	return &translator{
		name:    name,
		engine:  engine,
		factory: countconnector.NewFactory(),
	}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*countconnector.Config)

	switch t.engine {
	case common.PostgreSQLKey:
		return common.GetYamlFileToYamlConfig(cfg, dbiDbloadConfig)
	case common.MySQLKey:
		return common.GetYamlFileToYamlConfig(cfg, dbiDbloadMysqlConfig)
	}

	return nil, fmt.Errorf("unsupported count connector engine: %s", t.engine)
}
