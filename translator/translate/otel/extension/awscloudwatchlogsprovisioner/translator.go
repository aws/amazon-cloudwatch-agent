// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscloudwatchlogsprovisioner

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/awscloudwatchlogsprovisionerextension"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type translator struct {
	factory        extension.Factory
	additionalAuth component.ID
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslator(additionalAuth component.ID) common.ComponentTranslator {
	return &translator{
		factory:        awscloudwatchlogsprovisionerextension.NewFactory(),
		additionalAuth: additionalAuth,
	}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), "")
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	region := agent.Global_Config.Region
	if region == "" {
		return nil, fmt.Errorf("region is required for awscloudwatchlogsprovisioner extension")
	}
	cfg := t.factory.CreateDefaultConfig().(*awscloudwatchlogsprovisionerextension.Config)
	cfg.Region = region
	cfg.AdditionalAuth = &t.additionalAuth
	return cfg, nil
}
