// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otlphttpexporter

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

var (
	OTLPExportSectionKey = common.ConfigKey(common.MetricsKey, common.MetricsDestinationsKey, common.OtlpKey)
)

type translator struct {
	name    string
	factory exporter.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslator() common.ComponentTranslator {
	return NewTranslatorWithName("")
}

func NewTranslatorWithName(name string) common.ComponentTranslator {
	return &translator{name, otlphttpexporter.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*otlphttpexporter.Config)

	metricsEndpoint, ok := common.GetString(conf, common.ConfigKey(common.OtlpKey, common.Endpoint))
	if !ok {
		return nil, errors.New("otlphttpexporter: missing required endpoint")
	}
	cfg.MetricsEndpoint = metricsEndpoint

	// Configure authentication
	cfg.ClientConfig.Auth = &configauth.Authentication{AuthenticatorID: component.NewID(component.MustNewType(common.SigV4Auth))}

	return cfg, nil
}
