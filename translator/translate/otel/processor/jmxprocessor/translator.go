// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package jmxprocessor

import (
	"fmt"
	"strconv"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const (
	regexp = "regexp"
)

var (
	jmxKey     = common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.JmxKey)
	configKeys = map[component.DataType]string{
		component.DataTypeMetrics: common.ConfigKey(jmxKey),
	}

	jmxTargets = []string{"activemq", "cassandra", "hbase", "hadoop", "jetty", "jvm", "kafka", "kafka-consumer", "kafka-producer", "solr", "tomcat", "wildfly"}
)

type translator struct {
	dataType component.DataType
	name     string
	index    int
	factory  processor.Factory
}

type Option interface {
	apply(t *translator)
}

type optionFunc func(t *translator)

func (o optionFunc) apply(t *translator) {
	o(t)
}

// WithDataType determines where the translator should look to find
// the configuration.
func WithDataType(dataType component.DataType) Option {
	return optionFunc(func(t *translator) {
		t.dataType = dataType
	})
}

func WithIndex(index int) Option {
	return optionFunc(func(t *translator) {
		t.index = index
	})
}

var _ common.Translator[component.Config] = (*translator)(nil)

func NewTranslator(opts ...Option) common.Translator[component.Config] {
	return NewTranslatorWithName("", opts...)
}

func NewTranslatorWithName(name string, opts ...Option) common.Translator[component.Config] {
	t := &translator{name: name, index: -1, factory: filterprocessor.NewFactory()}
	for _, opt := range opts {
		opt.apply(t)
	}
	if name == "" && t.dataType != "" {
		t.name = string(t.dataType)
		if t.index != -1 {
			t.name += "/" + strconv.Itoa(t.index)
		}
	}
	return t
}

var _ common.Translator[component.Config] = (*translator)(nil)

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

// Translate creates a processor config based on the fields in the
// Metrics section of the JSON config.
func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*filterprocessor.Config)

	configKey, ok := configKeys[t.dataType]
	if !ok {
		return nil, fmt.Errorf("no config key defined for data type: %s", t.dataType)
	}
	if conf == nil || !conf.IsSet(configKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: configKey}
	}

	var jmxKeyMap map[string]interface{}
	if jmxSlice := common.GetArray[any](conf, configKey); t.index != -1 && len(jmxSlice) > t.index {
		jmxKeyMap = jmxSlice[t.index].(map[string]interface{})
	} else {
		jmxKeyMap = conf.Get(configKey).(map[string]interface{})
	}

	var includeMetricNames []string

	// When target name is set in configuration
	for _, jmxTarget := range jmxTargets {
		if _, ok := jmxKeyMap[jmxTarget]; ok {
			includeMetricNames = append(includeMetricNames, t.getIncludeJmxMetrics(conf, jmxTarget)...)
		}
	}
	c := confmap.NewFromStringMap(map[string]interface{}{
		"metrics": map[string]interface{}{
			"include": map[string]interface{}{
				"match_type":   regexp,
				"metric_names": includeMetricNames,
			},
		},
	})

	if err := c.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal jmx processor: %w", err)
	}

	return cfg, nil
}

func (t *translator) getIncludeJmxMetrics(conf *confmap.Conf, target string) []string {
	var includeMetricName []string
	targetMap := conf.Get(common.ConfigKey(jmxKey, target))
	targetMetrics, _ := targetMap.([]interface{})

	if len(targetMetrics) == 0 {
		// add regex to target when no metric names provided
		targetKeyRegex := target + ".*"
		includeMetricName = append(includeMetricName, targetKeyRegex)
	} else {
		for _, targetMetricName := range targetMetrics {
			includeMetricName = append(includeMetricName, targetMetricName.(string))
		}
	}
	return includeMetricName
}

func IsSet(conf *confmap.Conf) bool {
	for _, jmxTarget := range jmxTargets {
		if conf.IsSet(common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.JmxKey, jmxTarget)) {
			return true
		}
	}
	return false
}

