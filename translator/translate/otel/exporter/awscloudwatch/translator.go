// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscloudwatch

import (
	"github.com/mitchellh/mapstructure"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/plugins/outputs/cloudwatch"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/agent"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/metrics/drop_origin"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/metrics/metric_decoration"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/metrics/rollup_dimensions"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
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
	cfg.RollupDimensions = getRollupDimensions(conf)
	cfg.DropOriginConfigs = getDropOriginalMetrics(conf)
	cfg.MetricDecorations = getMetricDecorations(conf)
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

// TODO: remove dependency on rule.
func getRollupDimensions(conf *confmap.Conf) [][]string {
	key := common.ConfigKey(common.MetricsKey, rollup_dimensions.SectionKey)
	value := conf.Get(key)
	if value == nil || !rollup_dimensions.IsValidRollupList(value) {
		return nil
	}
	aggregates := value.([]interface{})
	rollup := make([][]string, len(aggregates))
	for i, aggregate := range aggregates {
		dimensions := aggregate.([]interface{})
		rollup[i] = make([]string, len(dimensions))
		for j, dimension := range dimensions {
			rollup[i][j] = dimension.(string)
		}
	}
	return rollup
}

// TODO: remove dependency on rule.
func getDropOriginalMetrics(conf *confmap.Conf) map[string][]string {
	_, result := new(drop_origin.DropOrigin).ApplyRule(conf.Get(common.MetricsKey))
	dom, ok := result.(map[string][]string)
	if ok {
		return dom
	}
	return nil
}

// TODO: remove dependency on rule.
func getMetricDecorations(conf *confmap.Conf) []cloudwatch.MetricDecorationConfig {
	_, result := new(metric_decoration.MetricDecoration).ApplyRule(conf.Get(common.MetricsKey))
	mds, ok := result.([]interface{})
	if !ok || len(mds) == 0 {
		return nil
	}
	decorations := make([]cloudwatch.MetricDecorationConfig, len(mds))
	for i, md := range mds {
		var decoration cloudwatch.MetricDecorationConfig
		if err := mapstructure.Decode(md, &decoration); err != nil {
			continue
		}
		decorations[i] = decoration
	}
	return decorations
}
