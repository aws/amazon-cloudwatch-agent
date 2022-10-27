// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsemf

import (
	_ "embed"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap"
	"gopkg.in/yaml.v3"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
)

//go:embed emf_config.yml
var defaultConfig string

type translator struct {
	factory component.ExporterFactory
}

var _ common.Translator[config.Exporter] = (*translator)(nil)

func NewTranslator() common.Translator[config.Exporter] {
	return &translator{awsemfexporter.NewFactory()}
}

func (t *translator) Type() config.Type {
	return t.factory.Type()
}

// Translate unmarshals the embedded config file into the default config.
func (t *translator) Translate(*confmap.Conf) (config.Exporter, error) {
	var rawConf map[string]interface{}
	if err := yaml.Unmarshal([]byte(defaultConfig), &rawConf); err != nil {
		return nil, fmt.Errorf("unable to read default config: %w", err)
	}
	conf := confmap.NewFromStringMap(rawConf)
	cfg := t.factory.CreateDefaultConfig()
	if err := conf.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal config: %w", err)
	}
	return cfg, nil
}
