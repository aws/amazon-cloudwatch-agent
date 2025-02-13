// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT
package metricsdecorator

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/internal/metric"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	translatorcontext "github.com/aws/amazon-cloudwatch-agent/translator/context"
	metricsconfig "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/util"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

// ContextStatement follows the yaml structure defined by otel's transform processor:
// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/processor/transformprocessor/internal/common/config.go#L45-L48)
type ContextStatement struct {
	Context    string   `mapstructure:"context"`
	Statements []string `mapstructure:"statements"`
}

type Translator interface {
	common.ComponentTranslator
	// IsSet determines whether the config has the fields needed for the translator.
	IsSet(conf *confmap.Conf) bool
}

type translator struct {
	name          string
	factory       processor.Factory
	index         int
	configKey     string
	ignorePlugins collections.Set[string]
}

type transformFn = func(string) string

type Option func(any)

var (
	defaultConfigKey = common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey)
)

func WithName(name string) Option {
	return func(a any) {
		if t, ok := a.(*translator); ok {
			t.name = name
		}
	}
}

func WithIndex(index int) Option {
	return func(a any) {
		if t, ok := a.(*translator); ok {
			t.index = index
		}
	}
}

func WithConfigKey(configKey string) Option {
	return func(a any) {
		if t, ok := a.(*translator); ok {
			t.configKey = configKey
		}
	}
}

func WithIgnorePlugins(ignorePlugins ...string) Option {
	return func(a any) {
		if t, ok := a.(*translator); ok {
			t.ignorePlugins = collections.NewSet(ignorePlugins...)
		}
	}
}

func NewTranslator(opts ...Option) Translator {
	t := &translator{
		factory:   transformprocessor.NewFactory(),
		configKey: defaultConfigKey,
		index:     -1,
	}
	for _, opt := range opts {
		opt(t)
	}
	if t.index != -1 {
		t.name = strings.Join([]string{t.name, strconv.Itoa(t.index)}, "/")
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

// Translate creates a processor config based on the fields in the
// Metrics section of the JSON config.
func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if conf == nil || !conf.IsSet(t.configKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: t.configKey}
	}

	cfg := t.factory.CreateDefaultConfig().(*transformprocessor.Config)
	contextStatement, err := t.getContextStatement(conf)
	if err != nil {
		return nil, fmt.Errorf("unable to translate context statements: %v", err)
	}

	c := confmap.NewFromStringMap(map[string]any{
		"metric_statements": []ContextStatement{contextStatement},
	})
	if err := c.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal metric decoration processor: %w", err)
	}
	return cfg, nil
}

func (t *translator) IsSet(conf *confmap.Conf) bool {
	measurementMaps := t.getMeasurementsByPlugin(conf)
	for _, measurementMap := range measurementMaps {
		for _, entry := range measurementMap {
			switch val := entry.(type) {
			case map[string]any:
				_, ok1 := val[common.RenameKey]
				_, ok2 := val[common.UnitKey]
				if ok1 || ok2 {
					return true
				}
			default:
				continue
			}
		}
	}
	return false
}

func (t *translator) getContextStatement(conf *confmap.Conf) (ContextStatement, error) {
	var statements []string
	measurementMaps := t.getMeasurementsByPlugin(conf)
	for plugin, measurementMap := range measurementMaps {
		plugin = metricsconfig.GetRealPluginName(plugin)
		var standardizeNameFn transformFn
		if t.configKey == defaultConfigKey {
			standardizeNameFn = decorateMetricNameFn(translatorcontext.CurrentContext().Os(), plugin)
		}
		for _, entry := range measurementMap {
			switch val := entry.(type) {
			case map[string]any:
				ms, err := getMetricStatements(val, standardizeNameFn)
				if err != nil {
					return ContextStatement{}, err
				}
				statements = append(statements, ms...)
			default:
				continue
			}
		}
	}
	return ContextStatement{
		Context:    "metric",
		Statements: statements,
	}, nil
}

func (t *translator) getMeasurementsByPlugin(conf *confmap.Conf) map[string][]any {
	m := common.GetIndexedMap(conf, t.configKey, t.index)
	if len(m) == 0 {
		return nil
	}
	measurementMap := make(map[string][]any)
	for plugin, value := range m {
		if t.ignorePlugins.Contains(plugin) {
			continue
		}
		if pluginMap, ok := value.(map[string]any); ok {
			if v, ok := pluginMap[common.MeasurementKey]; ok {
				measurementMap[plugin] = v.([]any)
			}
		}
	}
	return measurementMap
}

func getMetricStatements(m map[string]any, standardizeNameFn transformFn) ([]string, error) {
	var statements []string
	name, ok := m[common.NameKey]
	if !ok {
		return statements, errors.New("name field is missing for one of your metrics")
	}

	metricName := name.(string)
	if standardizeNameFn != nil {
		metricName = standardizeNameFn(metricName)
	}
	if metricName == "" {
		return statements, fmt.Errorf("metric name (%q) is invalid for decoration", metricName)
	}

	if newUnit, ok := m[common.UnitKey]; ok {
		statement := fmt.Sprintf("set(unit, \"%s\") where name == \"%s\"", newUnit, metricName)
		statements = append(statements, statement)
	}
	if newName, ok := m[common.RenameKey]; ok {
		statement := fmt.Sprintf("set(name, \"%s\") where name == \"%s\"", newName, metricName)
		statements = append(statements, statement)
	}
	return statements, nil
}

func decorateMetricNameFn(os, plugin string) transformFn {
	return func(name string) string {
		return metric.DecorateMetricName(plugin, util.GetValidMetric(os, plugin, name))
	}
}
