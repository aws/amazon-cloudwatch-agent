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
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

// EndpointConfig specifies signal-specific endpoints for the otlphttp exporter.
type EndpointConfig struct {
	LogsEndpoint    string
	MetricsEndpoint string
	TracesEndpoint  string
}

type translator struct {
	name                   string
	factory                exporter.Factory
	endpoint               EndpointConfig
	authenticator          component.ID
	queueBatchMetadataKeys []string
}

type Option func(*translator)

// WithAuthenticator sets a custom authenticator extension for the exporter.
func WithAuthenticator(id component.ID) Option {
	return func(t *translator) {
		t.authenticator = id
	}
}

// WithSendingQueueBatchMetadataKeys sets sending_queue.batch.partition.metadata_keys
// on the exporter. This is required for exporters whose routing depends on per-request
// client.Info context metadata (e.g. headerssetter-routed logs exporters that derive
// x-aws-log-group / x-aws-log-stream from context).
//
// At collector core v0.138.0 the default sending_queue enabled an exporter-level
// batcher (flush 200ms / min 8192 items) with no partitioner. Without partition keys
// that batcher merges requests across distinct routing metadata and resets the merged
// batch context to context.Background(), dropping the metadata (empty headers -> the
// log stream is silently never provisioned). Setting the partition metadata_keys makes
// the batcher partition per key set and preserve each partition's context.
func WithSendingQueueBatchMetadataKeys(keys ...string) Option {
	return func(t *translator) {
		t.queueBatchMetadataKeys = keys
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
	cfg.ClientConfig.Compression = configcompression.TypeGzip
	if t.authenticator.Type().String() != "" {
		cfg.ClientConfig.Auth = configoptional.Some(configauth.Config{
			AuthenticatorID: t.authenticator,
		})
	}

	if len(t.queueBatchMetadataKeys) > 0 {
		// Partition the exporter-level sending_queue batcher by these metadata keys so it
		// preserves per-request client.Info context instead of merging across partitions
		// and dropping it. See WithSendingQueueBatchMetadataKeys.
		cfg.QueueConfig.GetOrInsertDefault().Batch.GetOrInsertDefault().Partition.MetadataKeys = t.queueBatchMetadataKeys
	}

	return cfg, nil
}
