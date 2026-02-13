// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudauth

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"

	"github.com/aws/amazon-cloudwatch-agent/extension/cloudauth"
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
// Each exporter's credentials.role_arn is used directly by the web identity
// credential chain provider for AssumeRoleWithWebIdentity.
func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*cloudauth.Config)

	if tf, ok := common.GetString(conf, common.ConfigKey(common.AgentKey, common.CredentialsKey, common.OIDCAuthKey, tokenFileKey)); ok {
		cfg.TokenFile = tf
	}

	if res, ok := common.GetString(conf, common.ConfigKey(common.AgentKey, common.CredentialsKey, common.OIDCAuthKey, stsResourceKey)); ok {
		cfg.STSResource = res
	}

	return cfg, nil
}

// IsSet returns true if the agent JSON config has oidc_auth configured.
func IsSet(conf *confmap.Conf) bool {
	return conf.IsSet(common.ConfigKey(common.AgentKey, common.CredentialsKey, common.OIDCAuthKey))
}
