// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package count

import (
	_ "embed"
	"fmt"
	"strconv"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/countconnector"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/connector"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

//go:embed dbi_dbload.yaml
var dbiDbloadConfig string

type translator struct {
	name    string
	index   int
	factory connector.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

// NewTranslator creates a count connector translator. The name determines which config to load.
func NewTranslator(name string, index int) common.ComponentTranslator {
	return &translator{
		name:    name,
		index:   index,
		factory: countconnector.NewFactory(),
	}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name+"_"+strconv.Itoa(t.index))
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*countconnector.Config)

	if t.name == common.DbiConnectorDbload {
		return common.GetYamlFileToYamlConfig(cfg, dbiDbloadConfig)
	}

	return nil, fmt.Errorf("unsupported count connector config: %s", t.name)
}
