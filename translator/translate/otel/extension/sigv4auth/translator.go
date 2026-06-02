// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package sigv4auth

import (
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/sigv4authextension"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type translator struct {
	service string
	factory extension.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslator() common.ComponentTranslator {
	return &translator{factory: sigv4authextension.NewFactory()}
}

func NewTranslatorWithService(service string) common.ComponentTranslator {
	return &translator{service: service, factory: sigv4authextension.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.service)
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*sigv4authextension.Config)
	cfg.Region = agent.Global_Config.Region
	if t.service != "" {
		cfg.Service = t.service
	}
	if agent.Global_Config.Role_arn != "" {
		cfg.AssumeRole = sigv4authextension.AssumeRole{ARN: agent.Global_Config.Role_arn, STSRegion: agent.Global_Config.Region}
	}

	return cfg, nil
}

// CanResolveCredentials checks whether sigv4auth credentials can be resolved
// with the current environment. Returns true if credentials are available,
// false if the sigv4auth Validate() would fail (e.g. on-prem without AWS creds).
func CanResolveCredentials() bool {
	cfg := sigv4authextension.NewFactory().CreateDefaultConfig().(*sigv4authextension.Config)
	cfg.Region = agent.Global_Config.Region
	if agent.Global_Config.Role_arn != "" {
		cfg.AssumeRole = sigv4authextension.AssumeRole{ARN: agent.Global_Config.Role_arn, STSRegion: agent.Global_Config.Region}
	}
	return xconfmap.Validate(cfg) == nil
}
