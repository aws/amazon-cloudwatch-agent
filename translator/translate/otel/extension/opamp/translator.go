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
	
	return cfg, nil
}