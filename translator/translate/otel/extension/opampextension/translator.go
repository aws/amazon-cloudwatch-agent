// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package opampextension

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const TypeStr = "opamp"

type translator struct{}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslator() common.ComponentTranslator {
	return &translator{}
}

func (t *translator) ID() component.ID {
	return component.MustNewID(TypeStr)
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