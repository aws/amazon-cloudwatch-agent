// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otlphttp

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

// EndpointConfig specifies signal-specific endpoints for the otlphttp exporter.
type EndpointConfig struct {
	LogsEndpoint     string
	MetricsEndpoint  string
	TracesEndpoint   string
	ProfilesEndpoint string
}

type translator struct {
	name             string
	factory          exporter.Factory
	endpoint         EndpointConfig
	authenticator    component.ID
	requestQueueSize *int64
}

type Option func(*translator)

// WithAuthenticator sets a custom authenticator extension for the exporter.
func WithAuthenticator(id component.ID) Option {
	return func(t *translator) {
		t.authenticator = id
	}
}

// WithRequestQueueSize makes the translator emit an explicit sending_queue that
// bounds the exporter's in-memory queue to size export requests (sizer
// "requests"), instead of leaving the factory default of 1000. The queue stays
// non-blocking (block_on_overflow false): when it is full, further exports are
// rejected with a retryable error rather than the agent blocking the sender.
// When the option is unset the exporter keeps the factory default queue.
func WithRequestQueueSize(size int64) Option {
	return func(t *translator) {
		t.requestQueueSize = &size
	}
}

var _ common.ComponentTranslator = (*translator)(nil)

// NewTranslatorWithName creates an otlphttp exporter translator with the given
// endpoint configuration.
func NewTranslatorWithName(name string, endpoint EndpointConfig, opts ...Option) common.ComponentTranslator {
	t := &translator{
		name:     name,
		factory:  otlphttpexporter.NewFactory(),
		endpoint: endpoint,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*otlphttpexporter.Config)

	if t.endpoint.LogsEndpoint != "" {
		cfg.LogsEndpoint = t.endpoint.LogsEndpoint
	}
	if t.endpoint.MetricsEndpoint != "" {
		cfg.MetricsEndpoint = t.endpoint.MetricsEndpoint
	}
	if t.endpoint.TracesEndpoint != "" {
		cfg.TracesEndpoint = t.endpoint.TracesEndpoint
	}
	if t.endpoint.ProfilesEndpoint != "" {
		cfg.ProfilesEndpoint = t.endpoint.ProfilesEndpoint
	}
	cfg.ClientConfig.Compression = configcompression.TypeGzip
	if t.authenticator.Type().String() != "" {
		cfg.ClientConfig.Auth = configoptional.Some(configauth.Config{
			AuthenticatorID: t.authenticator,
		})
	}

	// Disable the exporter batcher; CWA batches in the pipeline batchprocessor.
	//
	// Use the OUTER Optional. Inner Batch=None writes "batch: null" in YAML
	// which the collector's confmap round-trip promotes back to Some(defaults),
	// re-enabling the batcher.
	cfg.QueueConfig = configoptional.None[exporterhelper.QueueBatchConfig]()

	// When a request-count cap is requested, emit an EXPLICIT sending_queue
	// instead of None. None marshals as an absent sending_queue key, which makes
	// the collector re-materialise the factory default (1000-request queue), so
	// the cap must be a concrete value. Start from the factory default so the
	// queue keeps parity with the other signals (sizer "requests", 10 consumers,
	// non-blocking) and only the size changes.
	//
	// NOTE: the agent YAML serializer writes the empty Batch optional as
	// "batch: null", which the collector would promote to Some(defaults) on
	// reload and re-enable the exporter batcher. The otlphttp queue-batch
	// converter in service/configprovider strips that null key so the batcher
	// stays off, matching metrics/logs. (A byte-sized cap, sizer: "bytes", is a
	// separate follow-up: SizerType.MarshalText has a pointer receiver and emits
	// "sizer: null", which reloads to the default request sizer.)
	if t.requestQueueSize != nil {
		queueConfig := exporterhelper.NewDefaultQueueConfig()
		queueConfig.QueueSize = *t.requestQueueSize
		cfg.QueueConfig = configoptional.Some(queueConfig)
	}

	return cfg, nil
}
