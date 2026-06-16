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

//go:embed dbi_dbload.yaml
var dbiDbloadConfig string

//go:embed dbi_dbload_mysql.yaml
var dbiDbloadMysqlConfig string

type translator struct {
	name    string
	factory connector.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

// NewTranslator creates a count connector translator. The name determines which config to load.
func NewTranslator(name string) common.ComponentTranslator {
	return &translator{
		name:    name,
		factory: countconnector.NewFactory(),
	}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*countconnector.Config)

	switch t.name {
	case common.DbiConnectorDbload:
		return common.GetYamlFileToYamlConfig(cfg, dbiDbloadConfig)
	case common.DbiConnectorDbloadMysql:
		return common.GetYamlFileToYamlConfig(cfg, dbiDbloadMysqlConfig)
	}

	return nil, fmt.Errorf("unsupported count connector config: %s", t.name)
}
