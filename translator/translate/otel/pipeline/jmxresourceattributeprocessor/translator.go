// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package jmxresourceattributeprocessor

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

var (
	jmxKey = common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.JmxKey)
)

type translator struct {
	name    string
	factory processor.Factory
}

var _ common.Translator[component.Config] = (*translator)(nil)

func NewTranslator() common.Translator[component.Config] {
	return NewTranslatorWithName(common.PipelineNameJmx)
}

func NewTranslatorWithName(name string) common.Translator[component.Config] {
	t := &translator{name: name, factory: resourceprocessor.NewFactory()}
	return t
}

var _ common.Translator[component.Config] = (*translator)(nil)

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

// Translate creates a processor config based on the fields in the
// Metrics section of the JSON config.
func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if conf == nil || !conf.IsSet(jmxKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: jmxKey}
	}

	cfg := t.factory.CreateDefaultConfig().(*resourceprocessor.Config)

	c := confmap.NewFromStringMap(map[string]interface{}{
		"attributes": []interface{}{
			map[string]interface{}{
				"action":  "delete",
				"pattern": "telemetry.sdk.*",
			},
		},
	})

	if err := c.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal jmx processor: %w", err)
	}

	return cfg, nil
}
