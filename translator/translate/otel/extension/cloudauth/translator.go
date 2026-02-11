// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudauth

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"

	"github.com/aws/amazon-cloudwatch-agent/extension/cloudauth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const (
	tokenFileKey   = "token_file"
	stsResourceKey = "sts_resource"
)

var ID = component.NewID(cloudauth.TypeStr)

type translator struct {
	factory extension.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslator() common.ComponentTranslator {
	return &translator{
		factory: cloudauth.NewFactory(),
	}
}

func (t *translator) ID() component.ID {
	return ID
}

// Translate creates the cloudauth extension config from the agent JSON config.
// The extension is activated when credentials.oidc_auth is present.
func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*cloudauth.Config)
	cfg.Region = agent.Global_Config.Region
	cfg.RoleARN = agent.Global_Config.Role_arn

	// Per-section role_arn override (metrics or logs level).
	if roleARN, ok := common.GetString(conf, common.ConfigKey(common.MetricsKey, common.CredentialsKey, common.RoleARNKey)); ok {
		cfg.RoleARN = roleARN
	} else if roleARN, ok := common.GetString(conf, common.ConfigKey(common.LogsKey, common.CredentialsKey, common.RoleARNKey)); ok {
		cfg.RoleARN = roleARN
	}

	// Optional token file path for on-prem / user-managed tokens.
	if tf, ok := common.GetString(conf, common.ConfigKey(common.AgentKey, common.CredentialsKey, common.OIDCAuthKey, tokenFileKey)); ok {
		cfg.TokenFile = tf
	}

	// Optional STS resource/audience override.
	if res, ok := common.GetString(conf, common.ConfigKey(common.AgentKey, common.CredentialsKey, common.OIDCAuthKey, stsResourceKey)); ok {
		cfg.STSResource = res
	}

	return cfg, nil
}

// IsSet returns true if the agent JSON config has oidc_auth configured.
func IsSet(conf *confmap.Conf) bool {
	return conf.IsSet(common.ConfigKey(common.AgentKey, common.CredentialsKey, common.OIDCAuthKey))
}
