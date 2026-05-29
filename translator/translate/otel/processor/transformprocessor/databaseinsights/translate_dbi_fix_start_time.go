// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package databaseinsights

import (
	_ "embed"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"gopkg.in/yaml.v3"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

//go:embed transform_dbi_fix_start_time.yaml
var fixStartTimeConfig []byte

type fixStartTimeTranslator struct{}

func NewFixStartTimeTranslator() common.ComponentTranslator {
	return &fixStartTimeTranslator{}
}

func (t *fixStartTimeTranslator) ID() component.ID {
	return component.MustNewIDWithName("transform", "dbi_fix_start_time")
}

func (t *fixStartTimeTranslator) Translate(_ *confmap.Conf) (component.Config, error) {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(fixStartTimeConfig, &raw); err != nil {
		return nil, err
	}
	cfg := &transformprocessor.Config{}
	if err := confmap.NewFromStringMap(raw).Unmarshal(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
