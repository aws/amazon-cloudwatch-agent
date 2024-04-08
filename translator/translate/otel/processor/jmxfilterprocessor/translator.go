// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package jmxfilterprocessor

import (
	"fmt"
	"strconv"
	"strings"

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
	jmxTargets = common.JmxTargets
)

type translator struct {
	name    string
	index   int
	factory processor.Factory
}

type Option interface {
	apply(t *translator)
}

type optionFunc func(t *translator)

func (o optionFunc) apply(t *translator) {
	o(t)
}

func WithIndex(index int) Option {
	return optionFunc(func(t *translator) {
		t.index = index
	})
}

var _ common.Translator[component.Config] = (*translator)(nil)

func NewTranslator(opts ...Option) common.Translator[component.Config] {
	return NewTranslatorWithName(common.PipelineNameJmx, opts...)
}

func NewTranslatorWithName(name string, opts ...Option) common.Translator[component.Config] {
	t := &translator{name: name, index: -1, factory: filterprocessor.NewFactory()}
	for _, opt := range opts {
		opt.apply(t)
	}
	if t.index != -1 {
		t.name += "/" + strconv.Itoa(t.index)
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
	if conf == nil || !conf.IsSet(jmxKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: jmxKey}
	}

	cfg := t.factory.CreateDefaultConfig().(*filterprocessor.Config)

	var jmxKeyMap map[string]interface{}
	if jmxSlice := common.GetArray[any](conf, jmxKey); t.index != -1 && len(jmxSlice) > t.index {
		jmxKeyMap = jmxSlice[t.index].(map[string]interface{})
	} else if _, ok := conf.Get(jmxKey).(map[string]interface{}); !ok {
		jmxKeyMap = make(map[string]interface{})
	} else {
		jmxKeyMap = conf.Get(jmxKey).(map[string]interface{})
	}

	var includeMetricNames []string

	// When target name is set in configuration
	for _, jmxTarget := range jmxTargets {
		if values, isSlice := jmxKeyMap[jmxTarget].([]interface{}); isSlice {
			if len(values) > 0 {
				// target name is set and has specific metrics set
				for _, value := range values {
					if valueStr, ok := value.(string); ok {
						includeMetricNames = append(includeMetricNames, valueStr)
					}
				}
			} else {
				// target name is set and wildcard for all metrics
				includeMetricNames = append(includeMetricNames, strings.Replace(jmxTarget, "-", ".", -1)+"\\..*")
			}
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

func IsSet(conf *confmap.Conf, pipelineIndex int) bool {
	jmxMetrics := conf.Get(jmxKey).([]interface{})
	jmxMap, _ := jmxMetrics[pipelineIndex].(map[string]interface{})
	for _, jmxTarget := range jmxTargets {
		if _, ok := jmxMap[jmxTarget]; ok {
			return true
		}
	}
	return false
}
