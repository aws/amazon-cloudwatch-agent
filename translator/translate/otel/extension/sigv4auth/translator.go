// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package sigv4auth

import (
	"os"
	"path/filepath"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/sigv4authextension"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"

	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/cloudauth"
)

type translator struct {
	name    string
	service string
	factory extension.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslator() common.ComponentTranslator {
	return NewTranslatorWithName("")
}

func NewTranslatorWithName(name string) common.ComponentTranslator {
	return &translator{name: name, factory: sigv4authextension.NewFactory()}
}

func NewTranslatorWithNameAndService(name, service string) common.ComponentTranslator {
	return &translator{name: name, service: service, factory: sigv4authextension.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*sigv4authextension.Config)
	cfg.Region = agent.Global_Config.Region
	if t.service != "" {
		cfg.Service = t.service
	}
	if agent.Global_Config.Role_arn != "" {
		cfg.AssumeRole = sigv4authextension.AssumeRole{ARN: agent.Global_Config.Role_arn, STSRegion: agent.Global_Config.Region}
		// When oidc_auth is configured, the cloudauth extension writes a token
		// file that sigv4auth should use for web identity federation. This must
		// be set here so that Validate() uses the web identity credential path
		// instead of the default chain (which fails on non-AWS hosts).
		if conf != nil && cloudauth.IsSet(conf) {
			tokenFile := filepath.Join(paths.AgentDir, "var", "cloudauth-token")
			// Create an empty placeholder so Validate() can read the file.
			// The real token is written by the cloudauth extension at Start().
			_ = os.MkdirAll(filepath.Dir(tokenFile), 0o755)
			if _, err := os.Stat(tokenFile); os.IsNotExist(err) {
				_ = os.WriteFile(tokenFile, []byte("placeholder"), 0o600)
			}
			cfg.AssumeRole.WebIdentityTokenFile = tokenFile
		}
	}

	return cfg, nil
}
