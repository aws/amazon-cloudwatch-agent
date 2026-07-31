// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package oidctoken

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/oidctokenextension"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"

	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type translator struct {
	factory extension.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslator() common.ComponentTranslator {
	return &translator{factory: oidctokenextension.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewID(t.factory.Type())
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*oidctokenextension.Config)
	// Only emitted for Azure VM/AKS, so the provider is always Azure.
	cfg.Provider = oidctokenextension.ProviderAzure
	cfg.OutputTokenFile = paths.OIDCTokenPath
	return cfg, nil
}
