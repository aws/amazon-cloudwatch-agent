// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package filterprocessor

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
	matchTypeStrict = "strict"
)

type translator struct {
	name    string
	index   int
	factory processor.Factory
}

type Option func(any)

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

var _ common.Translator[component.Config] = (*translator)(nil)

func NewTranslator(opts ...Option) common.Translator[component.Config] {
	t := &translator{index: -1, factory: filterprocessor.NewFactory()}
	for _, opt := range opts {
		opt(t)
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
	if conf == nil || !conf.IsSet(common.JmxConfigKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: common.JmxConfigKey}
	}

	cfg := t.factory.CreateDefaultConfig().(*filterprocessor.Config)

	jmxMap := common.GetIndexedMap(conf, common.JmxConfigKey, t.index)

	var includeMetricNames []string
	for _, jmxTarget := range common.JmxTargets {
		if targetMap, ok := jmxMap[jmxTarget].(map[string]any); ok {
			includeMetricNames = append(includeMetricNames, common.GetMeasurements(targetMap)...)
		}
	}

	c := confmap.NewFromStringMap(map[string]interface{}{
		"metrics": map[string]any{
			"include": map[string]any{
				"match_type":   matchTypeStrict,
				"metric_names": includeMetricNames,
			},
		},
	})

	if err := c.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal filter processor (%s): %w", t.ID(), err)
	}

	return cfg, nil
}
