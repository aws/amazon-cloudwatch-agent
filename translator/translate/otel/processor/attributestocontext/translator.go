// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package attributestocontext

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributestocontextprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

// ActionMapping defines a resource attribute to copy to client metadata.
type ActionMapping struct {
	Key                   string
	FromResourceAttribute string
}

type translator struct {
	factory processor.Factory
	actions []ActionMapping
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslator(actions []ActionMapping) common.ComponentTranslator {
	return &translator{factory: attributestocontextprocessor.NewFactory(), actions: actions}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), "")
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig()
	var actionsList []interface{}
	for _, a := range t.actions {
		actionsList = append(actionsList, map[string]interface{}{
			"key":                     a.Key,
			"from_resource_attribute": a.FromResourceAttribute,
		})
	}
	cfgMap := map[string]interface{}{
		"actions": actionsList,
	}
	if err := confmap.NewFromStringMap(cfgMap).Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to configure attributestocontext: %w", err)
	}
	return cfg, nil
}
