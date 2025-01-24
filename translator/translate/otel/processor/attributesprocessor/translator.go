// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package attributesprocessor

import (
	"fmt"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor"
)

type translator struct {
	name    string
	factory processor.Factory
}

var _ common.Translator[component.Config] = (*translator)(nil)

func NewTranslatorWithName(name string) common.Translator[component.Config] {
	return &translator{name, attributesprocessor.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*attributesprocessor.Config)
	actions := []map[string]interface{}{
		{
			"key":          "http.client_id",
			"from_context": "X-Forwarded-For",
			"action":       "upsert",
		},
	}

	c := confmap.NewFromStringMap(map[string]interface{}{
		"actions": actions,
	})

	if err := c.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal into attributesprocessor config: %w", err)
	}

	return cfg, nil
}
