// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudauth

import (
	"path/filepath"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/cloudauthextension"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const (
	tokenFileKey = "token_file"
	audienceKey  = "audience"
)

var (
	factory = cloudauthextension.NewFactory()
	ID      = component.NewID(factory.Type())
)

type translator struct{}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslator() common.ComponentTranslator {
	return &translator{}
}

func (t *translator) ID() component.ID {
	return ID
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	cfg := factory.CreateDefaultConfig().(*cloudauthextension.Config)

	cfg.TokenDir = filepath.Join(paths.AgentDir, "var")

	if tf, ok := common.GetString(conf, common.ConfigKey(common.AgentKey, common.CredentialsKey, common.OIDCAuthKey, tokenFileKey)); ok {
		cfg.TokenFile = tf
	}

	if res, ok := common.GetString(conf, common.ConfigKey(common.AgentKey, common.CredentialsKey, common.OIDCAuthKey, audienceKey)); ok {
		cfg.Audience = res
	}

	return cfg, nil
}

// IsSet returns true if the agent JSON config has oidc_auth configured.
func IsSet(conf *confmap.Conf) bool {
	return conf.IsSet(common.ConfigKey(common.AgentKey, common.CredentialsKey, common.OIDCAuthKey))
}
