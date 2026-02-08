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

// WithTelemetrySection sets the telemetry section key for the translator
func WithTelemetrySection(section string) common.TranslatorOption {
	return func(target any) {
		if t, ok := target.(*translator); ok {
			t.telemetrySectionKey = section
		}
	}
}

// WithTimeout sets a timeout override for the translator
func WithTimeout(timeout time.Duration) common.TranslatorOption {
	return func(target any) {
		if t, ok := target.(*translator); ok {
			t.timeout = &timeout
		}
	}
}

type translator struct {
	factory processor.Factory
	common.NameProvider
	telemetrySectionKey string
	timeout             *time.Duration
}

var _ common.ComponentTranslator = (*translator)(nil)
var _ common.NameSetter = (*translator)(nil)

// NewTranslator creates a new batch processor translator with options
func NewTranslator(opts ...common.TranslatorOption) common.ComponentTranslator {
	t := &translator{factory: batchprocessor.NewFactory()}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// Use NewTranslator with WithName and WithTelemetrySection options
func NewTranslatorWithNameAndSection(name string, telemetrySectionKey string) common.ComponentTranslator {
	return NewTranslator(common.WithName(name), WithTelemetrySection(telemetrySectionKey))
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.Name())
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*batchprocessor.Config)

	// First check if we have a timeout override
	if t.timeout != nil {
		cfg.Timeout = *t.timeout
	} else if duration, ok := common.GetDuration(conf, common.ConfigKey(t.telemetrySectionKey, common.ForceFlushIntervalKey)); ok {
		cfg.Timeout = duration
	} else if defaultDuration, ok := defaultForceFlushInterval[t.telemetrySectionKey]; ok {
		cfg.Timeout = defaultDuration
	} else {
		return cfg, fmt.Errorf("default force_flush_interval not defined for %s", t.telemetrySectionKey)
	}
	return cfg, nil
}
