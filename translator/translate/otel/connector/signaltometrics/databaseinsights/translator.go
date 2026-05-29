// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package databaseinsights

import (
	"embed"
	"fmt"
	"strconv"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector"
	signaltometricsconfig "github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/config"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/connector"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

//go:embed *.yaml
var configFiles embed.FS

type translator struct {
	engine  string
	index   int
	factory connector.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslator(engine string, index int) common.ComponentTranslator {
	return &translator{
		engine:  engine,
		index:   index,
		factory: signaltometricsconnector.NewFactory(),
	}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), "topsql_"+strconv.Itoa(t.index))
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*signaltometricsconfig.Config)
	data, err := configFiles.ReadFile(fmt.Sprintf("signaltometrics_%s_topsql.yaml", t.engine))
	if err != nil {
		return nil, fmt.Errorf("unable to read signaltometrics connector config for engine %s: %w", t.engine, err)
	}
	return common.GetYamlFileToYamlConfig(cfg, string(data))
}
