// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otlphttp

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type translator struct {
	name    string
	factory exporter.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslatorWithName(name string) common.ComponentTranslator {
	return &translator{name, otlphttpexporter.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

// Translate creates an otlphttp exporter config that sends OTLP logs to the
// CloudWatch OTLP endpoint with SigV4 authentication.
func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*otlphttpexporter.Config)

	region := agent.Global_Config.Region
	if region == "" {
		return nil, fmt.Errorf("region is required for otlphttp exporter")
	}

	cfg.ClientConfig.Endpoint = fmt.Sprintf("https://logs.%s.amazonaws.com", region)
	cfg.ClientConfig.Auth = &configauth.Authentication{
		AuthenticatorID: component.NewID(component.MustNewType(common.SigV4Auth)),
	}

	// Note: x-aws-log-group and x-aws-log-stream headers are injected as raw strings
	// in translatorutil.go:injectAppSignalsLogsHeaders() because configopaque.String
	// values are nil'd during mapstructure marshal (NilHookFunc) to prevent secret leakage.

	return cfg, nil
}
