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

// WithMetadataKeys sets the metadata_keys for per-key batching.
func WithMetadataKeys(keys []string) common.TranslatorOption {
	return func(target any) {
		if t, ok := target.(*translator); ok {
			t.metadataKeys = keys
		}
	}
}

// WithSendBatchSize sets the preferred number of items per batch export.
func WithSendBatchSize(size uint32) common.TranslatorOption {
	return func(target any) {
		if t, ok := target.(*translator); ok {
			t.sendBatchSize = size
		}
	}
}

// WithSendBatchMaxSize sets the maximum number of items per batch export.
func WithSendBatchMaxSize(size uint32) common.TranslatorOption {
	return func(target any) {
		if t, ok := target.(*translator); ok {
			t.sendBatchMaxSize = size
		}
	}
}

type translator struct {
	factory processor.Factory
	common.NameProvider
	telemetrySectionKey string
	timeout             *time.Duration
	metadataKeys        []string
	sendBatchSize       uint32
	sendBatchMaxSize    uint32
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
	} else if t.telemetrySectionKey != "" {
		if duration, ok := common.GetDuration(conf, common.ConfigKey(t.telemetrySectionKey, common.ForceFlushIntervalKey)); ok {
			cfg.Timeout = duration
		} else if defaultDuration, ok := defaultForceFlushInterval[t.telemetrySectionKey]; ok {
			cfg.Timeout = defaultDuration
		} else {
			return cfg, fmt.Errorf("default force_flush_interval not defined for %s", t.telemetrySectionKey)
		}
	}

	if len(t.metadataKeys) > 0 {
		cfg.MetadataKeys = t.metadataKeys
	}
	if t.sendBatchSize > 0 {
		cfg.SendBatchSize = t.sendBatchSize
	}
	if t.sendBatchMaxSize > 0 {
		cfg.SendBatchMaxSize = t.sendBatchMaxSize
	}
	return cfg, nil
}
