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
	cfg := t.factory.CreateDefaultConfig().(*opampextension.Config)
	
	// Initialize the server with a default HTTP configuration
	defaultServerConfig := map[string]interface{}{
		"server": map[string]interface{}{
			"http": map[string]interface{}{
				"endpoint": "localhost:4320",
			},
		},
	}
	
	// Create a confmap from the default config and unmarshal it
	defaultConfMap := confmap.NewFromStringMap(defaultServerConfig)
	
	// Unmarshal the default config into the config
	if err := defaultConfMap.Unmarshal(cfg); err != nil {
		return nil, err
	}
	
	// If no OpAMP configuration is provided, return the default config
	if !conf.IsSet(common.ConfigKey(common.AgentKey, "opamp")) {
		return cfg, nil
	}
	
	// Get the user-provided OpAMP configuration
	opampConf, err := conf.Sub(common.ConfigKey(common.AgentKey, "opamp"))
	if err != nil {
		return cfg, err
	}
	
	// Configure server if provided in user config
	if opampConf.IsSet("server") {
		serverConf, err := opampConf.Sub("server")
		if err != nil {
			return cfg, err
		}
		
		// If user explicitly configures server, reset our default
		if serverConf.IsSet("ws") || serverConf.IsSet("http") {
			// Reset the server config (we'll replace it with user config)
			cfg.Server = &opampextension.OpAMPServer{}
			
			// Create a map for the server config
			serverMap := make(map[string]interface{})
			for _, key := range serverConf.AllKeys() {
				serverMap[key] = serverConf.Get(key)
			}
			
			// Unmarshal the user's server config
			serverConfMap := confmap.NewFromStringMap(serverMap)
			if err := serverConfMap.Unmarshal(cfg.Server); err != nil {
				return cfg, err
			}
		}
	}

	// Set instance UID if provided
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

	// Configure agent description if provided
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

	// Configure capabilities if provided
	if opampConf.IsSet("capabilities") {
		capabilitiesConf, err := opampConf.Sub("capabilities")
		if err != nil {
			return cfg, err
		}
		

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