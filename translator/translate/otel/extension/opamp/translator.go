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
		name: "opamp",
		factory: opampextension.NewFactory(),
	}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*opampextension.Config)
	
	if !conf.IsSet(common.ConfigKey(common.AgentKey, "opamp")) {
		return cfg, nil
	}
	
	opampConf, err := conf.Sub(common.ConfigKey(common.AgentKey, "opamp"))
	if err != nil {
		return cfg, err
	}
	
	// Configure server
	if opampConf.IsSet("server") {
		serverConf, err := opampConf.Sub("server")
		if err != nil {
			return cfg, err
		}
		
		cfg.Server = &opampextension.OpAMPServer{}
		
		if serverConf.IsSet("ws") {
			wsConf, err := serverConf.Sub("ws")
			if err != nil {
				return cfg, err
			}
			// Use confmap to unmarshal WS config directly
			wsConf.Unmarshal(&cfg.Server.WS)
		} else if serverConf.IsSet("http") {
			// Use confmap to unmarshal HTTP config directly
			httpConf, err := serverConf.Sub("http")
			if err != nil {
				return cfg, err
			}
			httpConf.Unmarshal(&cfg.Server.HTTP)
		}
	}

	// Set instance UID
	if instanceUID, ok := opampConf.Get("instance_uid").(string); ok {
		cfg.InstanceUID = instanceUID
	}

	// Set PPID
	if ppidVal := opampConf.Get("ppid"); ppidVal != nil {
		switch v := ppidVal.(type) {
		case int:
			cfg.PPID = int32(v)
		case int32:
			cfg.PPID = v
		case int64:
			cfg.PPID = int32(v)
		case float64:
			cfg.PPID = int32(v)
		}
	}

	// Configure agent description
	if opampConf.IsSet("agent_description") {
		agentDescConf, err := opampConf.Sub("agent_description")
		if err != nil {
			return cfg, err
		}

		cfg.AgentDescription = opampextension.AgentDescription{}

		// Handle non-identifying attributes
		if agentDescConf.IsSet("non_identifying_attributes") {
			attrConf, err := agentDescConf.Sub("non_identifying_attributes")
			if err != nil {
				return cfg, err
			}

			// Initialize the map if it's nil
			if cfg.AgentDescription.NonIdentifyingAttributes == nil {
				cfg.AgentDescription.NonIdentifyingAttributes = make(map[string]string)
			}

			// Iterate through all keys and add them to the map
            for _, key := range attrConf.AllKeys() {
				if val, ok := attrConf.Get(key).(string); ok {
					cfg.AgentDescription.NonIdentifyingAttributes[key] = val
				}
			}
		}
	}

	// Configure capabilities
	if opampConf.IsSet("capabilities") {
    	capabilitiesConf, err := opampConf.Sub("capabilities")
    	if err != nil {
        	return cfg, err
    	}

   		// Initialize Capabilities (fix the incorrect initialization)
   		cfg.Capabilities = opampextension.Capabilities{} 

    	// Map each capability
    	if reportsEffectiveConfig, ok := capabilitiesConf.Get("reports_effective_config").(bool); ok {
        	cfg.Capabilities.ReportsEffectiveConfig = reportsEffectiveConfig
    	}
    	if reportsHealth, ok := capabilitiesConf.Get("reports_health").(bool); ok {
        	cfg.Capabilities.ReportsHealth = reportsHealth
    	}
    	if reportsAvailableComponents, ok := capabilitiesConf.Get("reports_available_components").(bool); ok {
        	cfg.Capabilities.ReportsAvailableComponents = reportsAvailableComponents
    	}
	}

	
	return cfg, nil
}