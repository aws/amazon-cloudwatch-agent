// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package batchprocessor

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/batchprocessor"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

var defaultForceFlushInterval = map[string]time.Duration{
	common.MetricsKey: 60 * time.Second,
	common.LogsKey:    5 * time.Second,
}

type translator struct {
	name                string
	telemetrySectionKey string
	factory             processor.Factory
}

var _ common.Translator[component.Config] = (*translator)(nil)

func NewTranslatorWithNameAndSection(name string, telemetrySectionKey string) common.Translator[component.Config] {
	return &translator{name, telemetrySectionKey, batchprocessor.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*batchprocessor.Config)
	if duration, ok := common.GetDuration(conf, common.ConfigKey(t.telemetrySectionKey, common.ForceFlushIntervalKey)); ok {
		cfg.Timeout = duration
	} else if defaultDuration, ok := defaultForceFlushInterval[t.telemetrySectionKey]; ok {
		cfg.Timeout = defaultDuration
	} else {
		return cfg, fmt.Errorf("default force_flush_interval not defined for %s", t.telemetrySectionKey)
	}
	return cfg, nil
}
