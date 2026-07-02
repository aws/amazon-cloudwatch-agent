// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package oidctoken

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/oidctokenextension"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"

	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/util"
)

// linuxOutputTokenFile is the token path sigv4auth reads on Linux/Darwin (etc dir).
const linuxOutputTokenFile = "/opt/aws/amazon-cloudwatch-agent/etc/.oidc-token"

type translator struct {
	factory extension.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

// NewTranslator returns the oidctoken extension translator (Azure provider).
func NewTranslator() common.ComponentTranslator {
	return &translator{factory: oidctokenextension.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewID(t.factory.Type())
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*oidctokenextension.Config)
	cfg.Provider = providerForMode(context.CurrentContext().Mode())
	cfg.OutputTokenFile = OutputTokenFile(context.CurrentContext().Os())
	return cfg, nil
}

// providerForMode maps the detected platform mode to the OIDC token provider.
func providerForMode(mode string) oidctokenextension.ProviderType {
	if mode == config.ModeAzureVM {
		return oidctokenextension.ProviderAzure
	}
	return oidctokenextension.ProviderAuto
}

// OutputTokenFile returns the token path for the target platform; sigv4auth points web_identity_token_file here.
func OutputTokenFile(targetPlatform string) string {
	if targetPlatform == config.OS_TYPE_WINDOWS {
		return util.GetWindowsProgramDataPath() + "\\Amazon\\AmazonCloudWatchAgent\\.oidc-token"
	}
	return linuxOutputTokenFile
}
