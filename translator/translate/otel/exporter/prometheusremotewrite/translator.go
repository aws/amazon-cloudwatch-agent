// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheusremotewrite

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

var (
	AMPSectionKey = common.ConfigKey(common.MetricsKey, common.MetricsDestinationsKey, common.AMPKey)
)

type translator struct {
	name    string
	factory exporter.Factory
}

var _ common.Translator[component.Config] = (*translator)(nil)

func NewTranslator() common.Translator[component.Config] {
	return NewTranslatorWithName("")
}

func NewTranslatorWithName(name string) common.Translator[component.Config] {
	return &translator{name, prometheusremotewriteexporter.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

// Translate creates an exporter config based on the fields in the
// amp or prometheus section of the JSON config.
func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if conf == nil || !conf.IsSet(AMPSectionKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: AMPSectionKey}
	}
	cfg := t.factory.CreateDefaultConfig().(*prometheusremotewriteexporter.Config)
	cfg.ClientConfig.Auth = &configauth.Authentication{AuthenticatorID: component.NewID(component.MustNewType(common.SigV4Auth))}
	cfg.ResourceToTelemetrySettings = resourcetotelemetry.Settings{Enabled: true, ClearAfterCopy: true}
	if value, ok := common.GetString(conf, common.ConfigKey(AMPSectionKey, common.WorkspaceIDKey)); ok {
		ampEndpoint := "https://aps-workspaces." + agent.Global_Config.Region + ".amazonaws.com/workspaces/" + value + "/api/v1/remote_write"
		cfg.ClientConfig.Endpoint = ampEndpoint
	}
	return cfg, nil
}
