// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package opampextension

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampextension"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)


type translator struct {
	name string
	factory extension.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslator() common.ComponentTranslator {
	return &translator{
		factory: opampextension.NewFactory(),
	}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	// Create a basic config map that will be used by the OTEL collector
	cfgMap := make(map[string]any)
	
	// Copy opamp configuration from agent.opamp to the root level
	if conf.IsSet(common.ConfigKey(common.AgentKey, "opamp")) {
		if opampConf, err := conf.Sub(common.ConfigKey(common.AgentKey, "opamp")); err == nil {
			for key, value := range opampConf.ToStringMap() {
				cfgMap[key] = value
			}
		}
	}
	
	// Return the config as a confmap.Conf
	return confmap.NewFromStringMap(cfgMap), nil
}