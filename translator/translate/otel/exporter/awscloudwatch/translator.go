// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscloudwatch

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter"

	"github.com/aws/amazon-cloudwatch-agent/plugins/outputs/cloudwatch"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
)

const (
	namespaceKey          = "namespace"
	forceFlushIntervalKey = "force_flush_interval"

	internalMaxValuesPerDatum = 5000
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
	return &translator{name, cloudwatch.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

// Translate creates an exporter config based on the fields in the
// metrics section of the JSON config.
// TODO: remove dependency on global config.
func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if conf == nil || !conf.IsSet(common.MetricsKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: common.MetricsKey}
	}
	cfg := t.factory.CreateDefaultConfig().(*cloudwatch.Config)
	credentials := confmap.NewFromStringMap(agent.Global_Config.Credentials)
	_ = credentials.Unmarshal(cfg)
	cfg.RoleARN = getRoleARN(conf)
	cfg.Region = agent.Global_Config.Region
	if namespace, ok := common.GetString(conf, common.ConfigKey(common.MetricsKey, namespaceKey)); ok {
		cfg.Namespace = namespace
	}
	if endpointOverride, ok := common.GetString(conf, common.ConfigKey(common.MetricsKey, common.EndpointOverrideKey)); ok {
		cfg.EndpointOverride = endpointOverride
	}
	if forceFlushInterval, ok := common.GetDuration(conf, common.ConfigKey(common.MetricsKey, forceFlushIntervalKey)); ok {
		cfg.ForceFlushInterval = forceFlushInterval
	}
	if agent.Global_Config.Internal {
		cfg.MaxValuesPerDatum = internalMaxValuesPerDatum
	}
	if rollupDimensions := common.GetRollupDimensions(conf); rollupDimensions != nil {
		cfg.RollupDimensions = rollupDimensions
	}
	if dropOriginalMetrics := common.GetDropOriginalMetrics(conf); len(dropOriginalMetrics) != 0 {
		cfg.DropOriginalConfigs = dropOriginalMetrics
	}
	cfg.MiddlewareID = &agenthealth.MetricsID
	return cfg, nil
}

func getRoleARN(conf *confmap.Conf) string {
	key := common.ConfigKey(common.MetricsKey, common.CredentialsKey, common.RoleARNKey)
	roleARN, ok := common.GetString(conf, key)
	if !ok {
		roleARN = agent.Global_Config.Role_arn
	}
	return roleARN
}
