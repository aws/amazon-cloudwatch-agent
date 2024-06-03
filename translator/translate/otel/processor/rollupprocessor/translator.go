// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package rollupprocessor

import (
	"sort"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"
	"golang.org/x/exp/maps"

	"github.com/aws/amazon-cloudwatch-agent/processor/rollupprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type translator struct {
	name    string
	factory processor.Factory
}

var _ common.Translator[component.Config] = (*translator)(nil)

func NewTranslator() common.Translator[component.Config] {
	return NewTranslatorWithName("")
}

func NewTranslatorWithName(name string) common.Translator[component.Config] {
	return &translator{name: name, factory: rollupprocessor.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if conf == nil || !conf.IsSet(common.MetricsAggregationDimensionsKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: common.MetricsAggregationDimensionsKey}
	}
	cfg := t.factory.CreateDefaultConfig().(*rollupprocessor.Config)
	if rollupDimensions := common.GetRollupDimensions(conf); len(rollupDimensions) != 0 {
		cfg.AttributeGroups = rollupDimensions
	}
	if dropOriginalMetrics := common.GetDropOriginalMetrics(conf); len(dropOriginalMetrics) != 0 {
		cfg.DropOriginal = maps.Keys(dropOriginalMetrics)
		sort.Strings(cfg.DropOriginal)
	}
	return cfg, nil
}
