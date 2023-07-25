// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT
package metricsdecorator

import (
	"errors"
	"fmt"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/internal/metric"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

var metricsKey = common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey)

// ContextStatement follows the yaml structure defined by otel's transform processor:
// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/processor/transformprocessor/internal/common/config.go#L45-L48)
type ContextStatement struct {
	Context    string   `mapstructure:"context"`
	Statements []string `mapstructure:"statements"`
}

type translator struct {
	name    string
	factory processor.Factory
}

var _ common.Translator[component.Config] = (*translator)(nil)

func NewTranslator() common.Translator[component.Config] {
	return NewTranslatorWithName("")
}

func NewTranslatorWithName(name string) common.Translator[component.Config] {
	return &translator{name, transformprocessor.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

// Translate creates a processor config based on the fields in the
// Metrics section of the JSON config.
func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if conf == nil || !conf.IsSet(common.MetricsKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: common.MetricsKey}
	}

	cfg := t.factory.CreateDefaultConfig().(*transformprocessor.Config)
	contextStatement, err := t.getContextStatements(conf)
	if err != nil {
		return nil, fmt.Errorf("unable to translate context statements: %v", err)
	}

	c := confmap.NewFromStringMap(map[string]interface{}{
		"metric_statements": contextStatement,
	})
	if err := c.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal metric decoration processor: %w", err)
	}
	return cfg, nil
}

func IsSet(conf *confmap.Conf) bool {
	measurementMaps := getMeasurementMaps(conf)
	for _, measurementMap := range measurementMaps {
		for _, entry := range measurementMap {
			switch val := entry.(type) {
			case map[string]interface{}:
				_, ok1 := val["rename"]
				_, ok2 := val["unit"]
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

func (t *translator) getContextStatements(conf *confmap.Conf) (ContextStatement, error) {
	var statements []string
	measurementMaps := getMeasurementMaps(conf)
	for plugin, measurementMap := range measurementMaps {
		for _, entry := range measurementMap {
			switch val := entry.(type) {
			case map[string]interface{}:
				name, ok := val["name"]
				if !ok {
					return ContextStatement{}, errors.New("name field is missing for one of your metrics")
				}

				// metrics at this point are all prefixed with their plugin category name.
				// 1. The plain metric name without the plugin prefix will not properly decorate in CW
				// 2. Without the prefix, there will be conflicting statements for metrics with the same name, but different plugins
				if !strings.HasPrefix(name.(string), plugin) {
					name = metric.DecorateMetricName(plugin, name.(string))
				}

				if newUnit, ok := val["unit"]; ok {
					statement := fmt.Sprintf("set(unit, \"%s\") where name == \"%s\"", newUnit, name)
					statements = append(statements, statement)
				}
				if newName, ok := val["rename"]; ok {
					statement := fmt.Sprintf("set(name, \"%s\") where name == \"%s\"", newName, name)
					statements = append(statements, statement)
				}
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

func getMeasurementMaps(conf *confmap.Conf) map[string][]interface{} {
	measurementMap := make(map[string][]interface{})
	metricsList := append(common.LinuxPluginKeys, common.WindowsPluginKeys...)
	for _, metric := range metricsList {
		path := common.ConfigKey(metricsKey, metric, common.MeasurementKey)
		if conf.IsSet(path) {
			m := conf.Get(path).([]interface{})
			measurementMap[metric] = m
		}
	}
	return measurementMap
}
